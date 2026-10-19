package app

// Posting to Tally.
//
// TallyBridge is read-only for the phone: every phone and desktop report goes
// through TallyClient.request, which accepts TALLYREQUEST Export only. This
// file is the single, separate path that can create anything in Tally. It is
// called only by the InvoiceTotally connector (invoicetotally.go), only after
// the user has switched on "Allow InvoiceTotally to post to Tally" on this PC,
// and only for requests that pass validatePostingXML: one Purchase or Sales
// voucher, or one new ledger, stock item or unit, for the expected company.
// Nothing can be altered or deleted through it.

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

var (
	errPostingUnreachable = errors.New("Tally is not reachable")
	errPostingTimeout     = errors.New("Tally took too long to answer. Check in Tally whether it was saved before pushing again")
	errPostingGate        = errors.New("Tally is busy with another request")
)

const maxPostingXML = 2 << 20

// pnode is a small XML tree used to check posting requests and read masters.
type pnode struct {
	Name string
	Attr map[string]string
	Text string
	Kids []*pnode
}

func (n *pnode) kid(name string) *pnode {
	for _, k := range n.Kids {
		if k.Name == name {
			return k
		}
	}
	return nil
}

// walk visits n and every descendant; returning false stops the descent below that node.
func (n *pnode) walk(f func(*pnode) bool) {
	if !f(n) {
		return
	}
	for _, k := range n.Kids {
		k.walk(f)
	}
}

func parseXTree(b []byte) (*pnode, error) {
	clean, _ := sanitizeXML(b)
	d := xml.NewDecoder(bytes.NewReader(clean))
	var root *pnode
	var stack []*pnode
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if len(stack) > 64 {
				return nil, errors.New("XML is nested too deeply")
			}
			n := &pnode{Name: strings.ToUpper(t.Name.Local), Attr: map[string]string{}}
			if t.Name.Space != "" && !strings.EqualFold(t.Name.Space, "UDF") {
				n.Name = strings.ToUpper(t.Name.Space + ":" + t.Name.Local)
			}
			for _, a := range t.Attr {
				n.Attr[strings.ToUpper(a.Name.Local)] = a.Value
			}
			if len(stack) == 0 {
				if root != nil {
					return nil, errors.New("more than one root element")
				}
				root = n
			} else {
				p := stack[len(stack)-1]
				p.Kids = append(p.Kids, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].Text += string(t)
			}
		}
	}
	if root == nil {
		return nil, errors.New("empty XML")
	}
	return root, nil
}

// validatePostingXML accepts exactly the requests InvoiceTotally sends to post a
// bill: Import Data of one Purchase/Sales voucher, or of one new ledger, stock
// item or unit, into tallyCompany. It returns a short label for the activity log.
func validatePostingXML(b []byte, tallyCompany string) (string, error) {
	if len(b) > maxPostingXML {
		return "", errors.New("request is too large")
	}
	root, err := parseXTree(b)
	if err != nil {
		return "", fmt.Errorf("request is not valid XML: %v", err)
	}
	if root.Name != "ENVELOPE" {
		return "", errors.New("request must be a Tally ENVELOPE")
	}
	var bad error
	root.walk(func(n *pnode) bool {
		if a, ok := n.Attr["ACTION"]; ok && !strings.EqualFold(strings.TrimSpace(a), "Create") {
			bad = fmt.Errorf("only new entries can be created (found ACTION=%q)", a)
		}
		switch n.Name {
		case "TDL", "TDLMESSAGE", "OBJECT", "SYSTEM", "COLLECTION", "REPORT", "FORM", "FUNCTION":
			bad = fmt.Errorf("%s is not allowed in a posting request", n.Name)
		}
		return bad == nil
	})
	if bad != nil {
		return "", bad
	}
	head, body := root.kid("HEADER"), root.kid("BODY")
	if head == nil || body == nil || len(root.Kids) != 2 {
		return "", errors.New("request must have exactly a HEADER and a BODY")
	}
	for _, k := range head.Kids {
		if k.Name != "TALLYREQUEST" && k.Name != "VERSION" {
			return "", fmt.Errorf("%s is not allowed in the header", k.Name)
		}
	}
	if tr := head.kid("TALLYREQUEST"); tr == nil || !strings.EqualFold(strings.TrimSpace(tr.Text), "Import Data") {
		return "", errors.New("only Import Data requests can be posted")
	}
	imp := body.kid("IMPORTDATA")
	if imp == nil || len(body.Kids) != 1 {
		return "", errors.New("request body must be IMPORTDATA")
	}
	desc, data := imp.kid("REQUESTDESC"), imp.kid("REQUESTDATA")
	if desc == nil || data == nil || len(imp.Kids) != 2 {
		return "", errors.New("IMPORTDATA must have REQUESTDESC and REQUESTDATA")
	}
	report := ""
	if r := desc.kid("REPORTNAME"); r != nil {
		report = strings.TrimSpace(r.Text)
	}
	if report != "Vouchers" && report != "All Masters" {
		return "", fmt.Errorf("report %q is not allowed", report)
	}
	company := ""
	for _, k := range desc.Kids {
		switch k.Name {
		case "REPORTNAME":
		case "STATICVARIABLES":
			for _, v := range k.Kids {
				if v.Name != "SVCURRENTCOMPANY" {
					return "", fmt.Errorf("%s is not allowed", v.Name)
				}
				company = strings.TrimSpace(v.Text)
			}
		default:
			return "", fmt.Errorf("%s is not allowed in REQUESTDESC", k.Name)
		}
	}
	want := strings.TrimSpace(tallyCompany)
	if want == "" || company == "" || !strings.EqualFold(company, want) {
		return "", fmt.Errorf("request is for company %q but this connection posts only to %q", company, want)
	}
	if len(data.Kids) != 1 || data.Kids[0].Name != "TALLYMESSAGE" || len(data.Kids[0].Kids) != 1 {
		return "", errors.New("request must carry exactly one entry")
	}
	obj := data.Kids[0].Kids[0]
	if strings.TrimSpace(obj.Attr["ACTION"]) == "" {
		return "", errors.New("entry must be marked ACTION=\"Create\"")
	}
	switch obj.Name {
	case "VOUCHER":
		if report != "Vouchers" {
			return "", errors.New("vouchers must use the Vouchers report")
		}
		vt := strings.TrimSpace(obj.Attr["VCHTYPE"])
		name := ""
		if n := obj.kid("VOUCHERTYPENAME"); n != nil {
			name = strings.TrimSpace(n.Text)
		}
		if (vt != "Purchase" && vt != "Sales") || name != vt {
			return "", fmt.Errorf("only Purchase and Sales vouchers can be posted (got %q)", vt)
		}
		return vt + " voucher", nil
	case "LEDGER", "STOCKITEM", "UNIT":
		if report != "All Masters" {
			return "", errors.New("masters must use the All Masters report")
		}
		return map[string]string{"LEDGER": "new ledger", "STOCKITEM": "new stock item", "UNIT": "new unit"}[obj.Name], nil
	}
	return "", fmt.Errorf("%s entries cannot be posted", obj.Name)
}

// acquireGate waits (up to ctx) for Tally to be free, then marks it busy.
// Tally answers one request at a time; phone reads and auto-sync share this gate.
func (t *TallyClient) acquireGate(ctx context.Context, report string) error {
	for {
		t.mu.Lock()
		if !t.busy && !time.Now().Before(t.cooldownUntil) {
			t.busy = true
			t.busyReport = report
			t.mu.Unlock()
			return nil
		}
		t.mu.Unlock()
		select {
		case <-ctx.Done():
			return errPostingGate
		case <-time.After(400 * time.Millisecond):
		}
	}
}

// post sends one validated posting request to Tally and returns Tally's reply
// as UTF-8 text. errPostingUnreachable means nothing reached Tally (safe to
// retry); errPostingTimeout means it may have been saved (never resend).
func (t *TallyClient) post(ctx context.Context, report, company string, body []byte, timeout time.Duration) (string, error) {
	cfg := t.store.Settings()
	if err := validateLocalTallyURL(cfg.TallyURL); err != nil {
		return "", err
	}
	waitCtx, cancelWait := context.WithTimeout(ctx, 90*time.Second)
	err := t.acquireGate(waitCtx, report)
	cancelWait()
	if err != nil {
		return "", err
	}
	start := time.Now()
	var finalErr error
	respBytes := 0
	defer func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.busy = false
		t.busyReport = ""
		lg := RequestLog{At: time.Now(), Report: report, Company: company, DurationMS: time.Since(start).Milliseconds(), Success: finalErr == nil, ResponseBytes: respBytes}
		if finalErr != nil {
			lg.Error = finalErr.Error()
		}
		t.logs = append([]RequestLog{lg}, t.logs...)
		if len(t.logs) > 100 {
			t.logs = t.logs[:100]
		}
	}()
	rctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodPost, cfg.TallyURL, bytes.NewReader(body))
	if err != nil {
		finalErr = err
		return "", err
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Close = true
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true, Proxy: nil}}
	res, err := client.Do(req)
	if err != nil {
		var op *net.OpError
		if errors.As(err, &op) && op.Op == "dial" {
			finalErr = errPostingUnreachable
			t.markNetworkFailure(err)
			return "", errPostingUnreachable
		}
		finalErr = errPostingTimeout
		return "", errPostingTimeout
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	respBytes = len(raw)
	if err != nil {
		finalErr = errPostingTimeout
		return "", errPostingTimeout
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		finalErr = fmt.Errorf("Tally HTTP %s", res.Status)
		return "", finalErr
	}
	t.mu.Lock()
	t.tallyOnline = true
	t.lastSuccess = time.Now()
	t.lastError = ""
	t.mu.Unlock()
	return string(tallyToUTF8(raw)), nil
}

// requestWait is request() for the connector's master reads: it waits for a
// busy gate or cooldown instead of failing at once.
func (t *TallyClient) requestWait(ctx context.Context, report, company string, body []byte, timeout time.Duration) ([]byte, error) {
	for {
		b, _, _, err := t.request(report, company, body, timeout)
		if !errors.Is(err, ErrBusy) && !errors.Is(err, ErrCooldown) {
			return b, err
		}
		select {
		case <-ctx.Done():
			return nil, errPostingGate
		case <-time.After(time.Second):
		}
	}
}

// importOutcome reads Tally's import reply: created/altered counts and line errors.
func importOutcome(resp string) (ok bool, errs string) {
	num := func(tag string) int {
		i := strings.Index(resp, "<"+tag+">")
		if i < 0 {
			return 0
		}
		rest := resp[i+len(tag)+2:]
		j := strings.Index(rest, "<")
		if j < 0 {
			return 0
		}
		n := 0
		fmt.Sscan(strings.TrimSpace(rest[:j]), &n)
		return n
	}
	var msgs []string
	for rest := resp; ; {
		i := strings.Index(rest, "<LINEERROR>")
		if i < 0 {
			break
		}
		rest = rest[i+11:]
		j := strings.Index(rest, "</LINEERROR>")
		if j < 0 {
			break
		}
		msgs = append(msgs, strings.Join(strings.Fields(stripTags(rest[:j])), " "))
		rest = rest[j:]
	}
	ok = num("CREATED")+num("ALTERED") > 0 && num("ERRORS") == 0 && num("EXCEPTIONS") == 0
	return ok, strings.Join(msgs, "; ")
}
