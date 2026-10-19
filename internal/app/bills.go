package app

// Purchase bills from the phone. The phone app sends each page (a JPEG plus a
// small preview) to this PC over its existing signed connection; the PC keeps
// them on disk and relays them to InvoiceTotally with the company's bridge
// token, retrying until it gets through. The phone never holds a token, and a
// bill reaches InvoiceTotally even if the internet drops while it is sent.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxBillPageBody   = 4 << 20 // one page request: JPEG + preview, base64
	maxBillImageBytes = 1_400_000
	maxBillThumbBytes = 400_000
	maxBillPages      = 40
	maxBillFiles      = 20
	maxBillsOnDisk    = 400 << 20
)

var billIDRE = regexp.MustCompile(`^[a-zA-Z0-9_-]{16,64}$`)

type billFile struct {
	Name  string `json:"name"`
	Pages int    `json:"pages"`
}

type billCloud struct {
	State    string           `json:"state"`
	Stage    string           `json:"stage"`
	Error    string           `json:"error,omitempty"`
	Invoices []map[string]any `json:"invoices"`
	Synced   int              `json:"synced"`
	At       time.Time        `json:"at"`
}

type billUpload struct {
	ID          string          `json:"id"`
	Connection  string          `json:"connection"`
	Company     string          `json:"company"`
	VoucherType string          `json:"voucherType"`
	Device      string          `json:"device"`
	DeviceID    string          `json:"deviceId"`
	Files       []billFile      `json:"files"`
	Received    map[string]bool `json:"received"` // "file-page"
	CreatedAt   time.Time       `json:"createdAt"`
	SubmittedAt time.Time       `json:"submittedAt,omitempty"`
	State       string          `json:"state"` // receiving | queued | sent | failed
	Error       string          `json:"error,omitempty"`
	Attempts    int             `json:"attempts"`
	NextTry     time.Time       `json:"nextTry,omitempty"`
	BatchID     string          `json:"batchId,omitempty"`
	BatchName   string          `json:"batchName,omitempty"`
	FileIDs     []string        `json:"fileIds,omitempty"`
	Uploaded    map[string]bool `json:"uploaded,omitempty"`
	SentAt      time.Time       `json:"sentAt,omitempty"`
	Cloud       *billCloud      `json:"cloud,omitempty"`
}

func (u *billUpload) pages() int {
	n := 0
	for _, f := range u.Files {
		n += f.Pages
	}
	return n
}

type BillQueue struct {
	s    *Server
	dir  string
	mu   sync.Mutex
	wake chan struct{}
	// test hook: shorter retry delays
	backoff func(attempt int) time.Duration
}

func newBillQueue(s *Server, base string) *BillQueue {
	q := &BillQueue{s: s, dir: filepath.Join(base, "bills"), wake: make(chan struct{}, 1)}
	q.backoff = func(a int) time.Duration {
		d := 30 * time.Second << min(a, 5)
		if d > 15*time.Minute {
			d = 15 * time.Minute
		}
		return d
	}
	_ = os.MkdirAll(q.dir, 0700)
	return q
}

func (q *BillQueue) path(id string, name ...string) string {
	return filepath.Join(append([]string{q.dir, id}, name...)...)
}

func (q *BillQueue) load(id string) (*billUpload, error) {
	b, err := os.ReadFile(q.path(id, "meta.json"))
	if err != nil {
		return nil, err
	}
	var u billUpload
	if err := json.Unmarshal(b, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (q *BillQueue) save(u *billUpload) error {
	b, _ := json.MarshalIndent(u, "", "  ")
	return atomicWrite(q.path(u.ID, "meta.json"), b, 0600)
}

func (q *BillQueue) all() []*billUpload {
	ents, _ := os.ReadDir(q.dir)
	out := []*billUpload{}
	for _, e := range ents {
		if !e.IsDir() || !billIDRE.MatchString(e.Name()) {
			continue
		}
		if u, err := q.load(e.Name()); err == nil {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (q *BillQueue) diskUse() int64 {
	var n int64
	_ = filepath.WalkDir(q.dir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if fi, e := d.Info(); e == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return n
}

func (q *BillQueue) poke() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// ----------------------------------------------------------------- phone side

type billPageIn struct {
	Upload string `json:"upload"`
	File   int    `json:"file"`
	Page   int    `json:"page"`
	Image  string `json:"image"`
	Thumb  string `json:"thumb"`
}

func decodeJPEG(b64 string, limit int) ([]byte, error) {
	if len(b64) > limit*4/3+8 {
		return nil, errors.New("page image is too large")
	}
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(b) < 4 || len(b) > limit || !bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}) {
		return nil, errors.New("page must be a JPEG image")
	}
	return b, nil
}

func (q *BillQueue) addPage(in billPageIn, device NativeDevice) error {
	if !billIDRE.MatchString(in.Upload) {
		return errors.New("invalid upload id")
	}
	if in.File < 0 || in.File >= maxBillFiles || in.Page < 0 || in.Page >= maxBillPages {
		return errors.New("too many pages in one upload")
	}
	img, err := decodeJPEG(in.Image, maxBillImageBytes)
	if err != nil {
		return err
	}
	var thumb []byte
	if in.Thumb != "" {
		if thumb, err = decodeJPEG(in.Thumb, maxBillThumbBytes); err != nil {
			return err
		}
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	u, err := q.load(in.Upload)
	if err != nil {
		if q.diskUse() > maxBillsOnDisk {
			return errors.New("the PC is still holding too many unsent bills — check its internet connection")
		}
		receiving := 0
		for _, x := range q.all() {
			if x.State == "receiving" {
				receiving++
			}
		}
		if receiving >= 20 {
			return errors.New("too many unfinished uploads — open TallyBridge on the PC")
		}
		if err := os.MkdirAll(q.path(in.Upload), 0700); err != nil {
			return err
		}
		u = &billUpload{ID: in.Upload, State: "receiving", CreatedAt: time.Now(), Device: device.Name, DeviceID: device.ID, Received: map[string]bool{}}
	} else if u.DeviceID != device.ID {
		return errors.New("this upload belongs to another phone")
	} else if u.State != "receiving" {
		return nil // already submitted: a repeated page is harmless
	}
	if len(u.Received) >= maxBillPages && !u.Received[fmt.Sprintf("%d-%d", in.File, in.Page)] {
		return errors.New("too many pages in one upload")
	}
	name := fmt.Sprintf("%02d-%03d", in.File, in.Page)
	if err := atomicWrite(q.path(u.ID, "p"+name+".jpg"), img, 0600); err != nil {
		return err
	}
	if thumb != nil {
		if err := atomicWrite(q.path(u.ID, "t"+name+".jpg"), thumb, 0600); err != nil {
			return err
		}
	}
	u.Received[fmt.Sprintf("%d-%d", in.File, in.Page)] = true
	return q.save(u)
}

type billSubmitIn struct {
	Upload      string     `json:"upload"`
	Connection  string     `json:"connection"`
	VoucherType string     `json:"voucherType"`
	Files       []billFile `json:"files"`
}

func (q *BillQueue) submit(in billSubmitIn, device NativeDevice) (*billUpload, error) {
	conn, ok := q.s.it.connection(in.Connection)
	if !ok {
		return nil, errors.New("that company is no longer connected on the PC — refresh and pick again")
	}
	if len(in.Files) == 0 || len(in.Files) > maxBillFiles {
		return nil, errors.New("add at least one page")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	u, err := q.load(in.Upload)
	if err != nil {
		return nil, errors.New("the PC has not received these pages — send again")
	}
	if u.DeviceID != device.ID {
		return nil, errors.New("this upload belongs to another phone")
	}
	if u.State != "receiving" {
		return u, nil // repeated submit after a dropped reply
	}
	total := 0
	for i, f := range in.Files {
		if f.Pages < 1 || f.Pages > maxBillPages {
			return nil, errors.New("invalid page count")
		}
		for p := 0; p < f.Pages; p++ {
			if !u.Received[fmt.Sprintf("%d-%d", i, p)] {
				return nil, fmt.Errorf("page %d of %s did not arrive — send again", p+1, f.Name)
			}
		}
		total += f.Pages
		name := strings.TrimSpace(f.Name)
		if name == "" {
			name = "Bill photos"
		}
		if len(name) > 120 {
			name = name[:120]
		}
		in.Files[i].Name = name
	}
	if total > maxBillPages {
		return nil, errors.New("too many pages in one upload")
	}
	u.Files = in.Files
	u.Connection = conn.ID
	u.Company = conn.Company
	u.VoucherType = "Purchase"
	if in.VoucherType == "Sales" {
		u.VoucherType = "Sales"
	}
	u.State = "queued"
	u.SubmittedAt = time.Now()
	u.NextTry = time.Time{}
	if err := q.save(u); err != nil {
		return nil, err
	}
	q.poke()
	return u, nil
}

// ----------------------------------------------------------------- relay

func (q *BillQueue) Run(ctx context.Context) {
	q.cleanup()
	lastClean := time.Now()
	for {
		next := 20 * time.Second
		for _, u := range q.all() {
			if ctx.Err() != nil {
				return
			}
			if u.State != "queued" {
				continue
			}
			if !time.Now().Before(u.NextTry) {
				q.send(ctx, u.ID)
				if cur, err := q.load(u.ID); err == nil && cur.State == "queued" {
					u = cur
				} else {
					continue
				}
			}
			if d := time.Until(u.NextTry); d < next {
				next = max(d, 10*time.Millisecond)
			}
		}
		if time.Since(lastClean) > time.Hour {
			q.cleanup()
			lastClean = time.Now()
		}
		select {
		case <-ctx.Done():
			return
		case <-q.wake:
		case <-time.After(next):
		}
	}
}

func retryable(err error) bool {
	var he *itHTTPError
	if errors.As(err, &he) {
		return he.Code == 401 || he.Code == 408 || he.Code == 429 || he.Code >= 500
	}
	return true
}

func (q *BillQueue) fail(u *billUpload, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	cur, e := q.load(u.ID)
	if e != nil || cur.State != "queued" {
		return
	}
	cur.Attempts++
	cur.Error = friendlyNetErr(err)
	if retryable(err) {
		cur.NextTry = time.Now().Add(q.backoff(cur.Attempts))
	} else {
		cur.State = "failed"
	}
	_ = q.save(cur)
}

func (q *BillQueue) update(id string, f func(*billUpload)) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if u, err := q.load(id); err == nil {
		f(u)
		_ = q.save(u)
	}
}

func (q *BillQueue) send(ctx context.Context, id string) {
	u, err := q.load(id)
	if err != nil || u.State != "queued" {
		return
	}
	it := q.s.it
	cfg := it.config()
	conn, ok := it.connection(u.Connection)
	if !ok || cfg.ServerURL == "" {
		q.update(id, func(x *billUpload) {
			x.State, x.Error = "failed", "That company was removed from TallyBridge on this PC"
		})
		return
	}
	if u.BatchID == "" {
		var r struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Files []struct {
				ID string `json:"id"`
			} `json:"files"`
		}
		files := []map[string]any{}
		for _, f := range u.Files {
			files = append(files, map[string]any{"filename": f.Name, "pages": f.Pages})
		}
		if err := it.call(ctx, cfg.ServerURL, conn.Token, "POST", "/api/bridge/batches",
			map[string]any{"voucher_type": u.VoucherType, "uploaded_by": u.Device, "files": files}, &r, time.Minute); err != nil {
			q.fail(u, err)
			return
		}
		if len(r.Files) != len(u.Files) {
			q.fail(u, &itHTTPError{Code: 400, Msg: "InvoiceTotally did not accept the files"})
			return
		}
		u.BatchID, u.BatchName = r.ID, r.Name
		for _, f := range r.Files {
			u.FileIDs = append(u.FileIDs, f.ID)
		}
		q.update(id, func(x *billUpload) { x.BatchID, x.BatchName, x.FileIDs = u.BatchID, u.BatchName, u.FileIDs })
	}
	if u.Uploaded == nil {
		u.Uploaded = map[string]bool{}
	}
	for fi, f := range u.Files {
		for p := 0; p < f.Pages; p++ {
			key := fmt.Sprintf("%d-%d", fi, p)
			if u.Uploaded[key] {
				continue
			}
			name := fmt.Sprintf("%02d-%03d", fi, p)
			img, err := os.ReadFile(q.path(id, "p"+name+".jpg"))
			if err != nil {
				q.update(id, func(x *billUpload) {
					x.State, x.Error = "failed", "A page image is missing on the PC — send the bill again"
				})
				return
			}
			thumb, _ := os.ReadFile(q.path(id, "t"+name+".jpg"))
			body := map[string]any{"idx": p, "image_b64": base64.StdEncoding.EncodeToString(img)}
			if len(thumb) > 0 {
				body["thumb_b64"] = base64.StdEncoding.EncodeToString(thumb)
			}
			if err := it.call(ctx, cfg.ServerURL, conn.Token, "POST", "/api/bridge/files/"+url.PathEscape(u.FileIDs[fi])+"/pages", body, nil, 2*time.Minute); err != nil {
				q.fail(u, err)
				return
			}
			u.Uploaded[key] = true
			q.update(id, func(x *billUpload) { x.Uploaded = u.Uploaded })
		}
	}
	if err := it.call(ctx, cfg.ServerURL, conn.Token, "POST", "/api/bridge/batches/"+url.PathEscape(u.BatchID)+"/start", map[string]any{}, nil, time.Minute); err != nil {
		q.fail(u, err)
		return
	}
	q.update(id, func(x *billUpload) {
		x.State, x.Error, x.SentAt, x.NextTry = "sent", "", time.Now(), time.Time{}
		x.Cloud = &billCloud{State: "Reading", Stage: "find", Invoices: []map[string]any{}} // zero At: ask InvoiceTotally on the next look
		q.dropImages(id)                                                                    // InvoiceTotally has them now
	})
	it.log(fmt.Sprintf("Sent %d page(s) from %s to InvoiceTotally (%s)", u.pages(), u.Device, u.Company), "phone", true, nil)
}

func (q *BillQueue) dropImages(id string) {
	ents, _ := os.ReadDir(q.path(id))
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".jpg") {
			_ = os.Remove(q.path(id, e.Name()))
		}
	}
}

func (q *BillQueue) cleanup() {
	for _, u := range q.all() {
		age := time.Since(u.CreatedAt)
		if (u.State == "receiving" && age > 24*time.Hour) || ((u.State == "sent" || u.State == "failed") && age > 14*24*time.Hour) {
			_ = os.RemoveAll(q.path(u.ID))
		}
	}
}

func (q *BillQueue) retry(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	u, err := q.load(id)
	if err != nil || (u.State != "failed" && u.State != "queued") {
		return errors.New("only waiting or failed uploads can be retried")
	}
	u.State, u.NextTry, u.Error = "queued", time.Time{}, ""
	if err := q.save(u); err != nil {
		return err
	}
	q.poke()
	return nil
}

func (q *BillQueue) remove(id string) error {
	if !billIDRE.MatchString(id) {
		return errors.New("invalid upload")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, err := q.load(id); err != nil {
		return errors.New("upload not found")
	}
	return os.RemoveAll(q.path(id))
}

// refreshCloud asks InvoiceTotally how the sent bills are doing (at most every 15 s).
func (q *BillQueue) refreshCloud(ctx context.Context, ups []*billUpload) {
	it := q.s.it
	cfg := it.config()
	by := map[string][]*billUpload{}
	for _, u := range ups {
		if u.State == "sent" && u.BatchID != "" && time.Since(u.SentAt) < 7*24*time.Hour && (u.Cloud == nil || time.Since(u.Cloud.At) > 15*time.Second) {
			by[u.Connection] = append(by[u.Connection], u)
		}
	}
	for connID, list := range by {
		conn, ok := it.connection(connID)
		if !ok || cfg.ServerURL == "" {
			continue
		}
		if len(list) > 30 {
			list = list[:30]
		}
		ids := []string{}
		for _, u := range list {
			ids = append(ids, u.BatchID)
		}
		var r struct {
			Batches []struct {
				ID       string           `json:"id"`
				Stage    string           `json:"stage"`
				State    string           `json:"state"`
				Error    string           `json:"error"`
				Invoices []map[string]any `json:"invoices"`
				Synced   int              `json:"synced"`
			} `json:"batches"`
		}
		if err := it.call(ctx, cfg.ServerURL, conn.Token, "GET", "/api/bridge/batches?ids="+strings.Join(ids, ","), nil, &r, 15*time.Second); err != nil {
			continue
		}
		found := map[string]bool{}
		for _, b := range r.Batches {
			found[b.ID] = true
			cloud := &billCloud{State: b.State, Stage: b.Stage, Error: b.Error, Invoices: b.Invoices, Synced: b.Synced, At: time.Now()}
			if cloud.Invoices == nil {
				cloud.Invoices = []map[string]any{}
			}
			if b.Stage == "review" && len(b.Invoices) > 0 && b.Synced == len(b.Invoices) {
				cloud.State = "In Tally"
			}
			for _, u := range list {
				if u.BatchID == b.ID {
					u.Cloud = cloud
					q.update(u.ID, func(x *billUpload) { x.Cloud = cloud })
				}
			}
		}
		for _, u := range list {
			if !found[u.BatchID] {
				cloud := &billCloud{State: "Deleted in InvoiceTotally", Stage: "deleted", At: time.Now(), Invoices: []map[string]any{}}
				u.Cloud = cloud
				q.update(u.ID, func(x *billUpload) { x.Cloud = cloud })
			}
		}
	}
}

type billView struct {
	ID        string     `json:"id"`
	Company   string     `json:"company"`
	Device    string     `json:"device"`
	Type      string     `json:"voucherType"`
	Pages     int        `json:"pages"`
	Files     []billFile `json:"files"`
	State     string     `json:"state"`
	Status    string     `json:"status"`
	Error     string     `json:"error,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	SentAt    time.Time  `json:"sentAt,omitempty"`
	NextTry   time.Time  `json:"nextTry,omitempty"`
	Batch     string     `json:"batch,omitempty"`
	Cloud     *billCloud `json:"cloud,omitempty"`
}

func (u *billUpload) view() billView {
	status := map[string]string{"receiving": "Receiving pages", "queued": "Waiting to send", "sent": "Sent", "failed": "Not sent"}[u.State]
	if u.State == "queued" && u.Attempts > 0 {
		status = "Will retry — " + u.Error
	}
	if u.State == "sent" && u.Cloud != nil && u.Cloud.State != "" {
		status = u.Cloud.State
	}
	return billView{ID: u.ID, Company: u.Company, Device: u.Device, Type: u.VoucherType, Pages: u.pages(), Files: u.Files, State: u.State, Status: status,
		Error: u.Error, CreatedAt: u.CreatedAt, SentAt: u.SentAt, NextTry: u.NextTry, Batch: u.BatchName, Cloud: u.Cloud}
}

func (q *BillQueue) list(desktop bool) []billView {
	out := []billView{}
	for _, u := range q.all() {
		if u.State == "receiving" && !desktop {
			continue
		}
		out = append(out, u.view())
		if len(out) >= 60 {
			break
		}
	}
	return out
}

func (s *Server) phoneDevice(r *http.Request) NativeDevice {
	id := r.Header.Get("X-TB-Device")
	s.native.mu.Lock()
	defer s.native.mu.Unlock()
	for _, d := range s.native.Devices {
		if d.ID == id {
			return d
		}
	}
	return NativeDevice{ID: id, Name: "Phone"}
}

// handleBills serves /api/v1/mobile/bills* (behind phone authentication).
func (s *Server) handleBills(w http.ResponseWriter, r *http.Request) {
	cfg := s.it.config()
	enabled := cfg.ServerURL != "" && len(cfg.Connections) > 0 && !cfg.PhoneOff
	reason := ""
	switch {
	case cfg.ServerURL == "" || len(cfg.Connections) == 0:
		reason = "Connect InvoiceTotally on the PC first: TallyBridge → Purchase bills."
	case cfg.PhoneOff:
		reason = "Sending bills from the phone is switched off on the PC."
	}
	dev := s.phoneDevice(r)
	switch {
	case r.URL.Path == "/api/v1/mobile/bills" && r.Method == http.MethodGet:
		mine := []*billUpload{}
		for _, u := range s.bills.all() {
			if u.DeviceID == dev.ID && u.State != "receiving" {
				mine = append(mine, u)
			}
		}
		if len(mine) > 40 {
			mine = mine[:40]
		}
		if r.URL.Query().Get("refresh") == "1" {
			s.bills.refreshCloud(r.Context(), mine)
		}
		views := []billView{}
		for _, u := range mine {
			views = append(views, u.view())
		}
		jsonOut(w, map[string]any{"enabled": enabled, "reason": reason, "companies": s.it.phoneCompanies(), "uploads": views,
			"maxPages": maxBillPages, "maxImageBytes": maxBillImageBytes, "maxThumbBytes": maxBillThumbBytes})
	case r.URL.Path == "/api/v1/mobile/bills/page" && r.Method == http.MethodPost:
		if !enabled {
			jsonErr(w, 409, errors.New(reason))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBillPageBody)
		var in billPageIn
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			jsonErr(w, 400, errors.New("invalid page upload"))
			return
		}
		if err := s.bills.addPage(in, dev); err != nil {
			jsonErr(w, 400, err)
			return
		}
		jsonOut(w, map[string]any{"ok": true})
	case r.URL.Path == "/api/v1/mobile/bills/submit" && r.Method == http.MethodPost:
		if !enabled {
			jsonErr(w, 409, errors.New(reason))
			return
		}
		var in billSubmitIn
		if err := readJSON(r, &in); err != nil {
			jsonErr(w, 400, errors.New("invalid request"))
			return
		}
		u, err := s.bills.submit(in, dev)
		if err != nil {
			jsonErr(w, 400, err)
			return
		}
		jsonOut(w, u.view())
	default:
		http.NotFound(w, r)
	}
}
