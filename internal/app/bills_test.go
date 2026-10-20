package app

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"strings"
	"testing"
	"time"
)

func testJPEG(t *testing.T, w, h int, shade uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{shade, uint8(x), uint8(y), 255})
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

type phone struct {
	token string
	id    string
	key   *ecdsa.PrivateKey
}

func pairPhone(t *testing.T, s *Server, rsaKey *rsa.PrivateKey, name string) phone {
	t.Helper()
	token := accessToken(t, s, rsaKey, nil)
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pub, _ := x509.MarshalPKIXPublicKey(&k.PublicKey)
	code, _ := s.store.GeneratePairCode()
	body, _ := json.Marshal(map[string]string{"code": code, "name": name, "publicKey": base64.StdEncoding.EncodeToString(pub)})
	w := serve(s.NativeHandlerWithOrders(), nativeRequest(t, "POST", "/api/v2/native/pair", string(body), token, "", nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var r map[string]any
	json.Unmarshal(w.Body.Bytes(), &r)
	id := r["deviceId"].(string)
	s.native.mu.Lock()
	for i := range s.native.Devices {
		if s.native.Devices[i].ID == id {
			s.native.Devices[i].Approved = true
		}
	}
	s.native.mu.Unlock()
	return phone{token, id, k}
}

func (p phone) do(t *testing.T, s *Server, method, path string, body any) (int, map[string]any) {
	t.Helper()
	raw := ""
	if body != nil {
		b, _ := json.Marshal(body)
		raw = string(b)
	}
	w := serve(s.NativeHandlerWithOrders(), nativeRequest(t, method, path, raw, p.token, p.id, p.key))
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestPhoneBillsReachInvoiceTotally(t *testing.T) {
	s, rsaKey := nativeFixture(t)
	cloud := newFakeCloud(t)
	s.bills.backoff = func(int) time.Duration { return 10 * time.Millisecond }
	ravi := pairPhone(t, s, rsaKey, "Ravi's Pixel")
	other := pairPhone(t, s, rsaKey, "Shop tablet")

	// Before InvoiceTotally is connected the phone is told what to do.
	code, out := ravi.do(t, s, "GET", "/api/v1/mobile/bills", nil)
	if code != 200 || out["enabled"] != false || !strings.Contains(out["reason"].(string), "Connect InvoiceTotally") {
		t.Fatal(code, out)
	}
	img := base64.StdEncoding.EncodeToString(testJPEG(t, 1200, 1600, 10))
	if code, _ := ravi.do(t, s, "POST", "/api/v1/mobile/bills/page", billPageIn{Upload: "u" + strings.Repeat("1", 20), Image: img}); code != 409 {
		t.Fatal("page accepted while not connected", code)
	}

	if _, err := s.it.save(context.Background(), cloud.srv.URL, []string{cloud.token}, nil); err != nil {
		t.Fatal(err)
	}
	s.it.Stop() // only the bill relay is under test here
	code, out = ravi.do(t, s, "GET", "/api/v1/mobile/bills", nil)
	companies := out["companies"].([]any)
	if code != 200 || out["enabled"] != true || len(companies) != 1 {
		t.Fatal(code, out)
	}
	b, _ := json.Marshal(out)
	if strings.Contains(string(b), cloud.token) {
		t.Fatal("bridge token sent to the phone")
	}
	conn := companies[0].(map[string]any)["id"].(string)

	up := "bill-" + strings.Repeat("a", 20)
	thumb := base64.StdEncoding.EncodeToString(testJPEG(t, 300, 400, 10))
	page2 := base64.StdEncoding.EncodeToString(testJPEG(t, 1200, 1600, 99))
	for _, p := range []billPageIn{{Upload: up, File: 0, Page: 0, Image: img, Thumb: thumb}, {Upload: up, File: 0, Page: 1, Image: page2}, {Upload: up, File: 0, Page: 1, Image: page2}} {
		if code, out := ravi.do(t, s, "POST", "/api/v1/mobile/bills/page", p); code != 200 {
			t.Fatal(code, out)
		}
	}
	if code, out := ravi.do(t, s, "POST", "/api/v1/mobile/bills/page", billPageIn{Upload: up, Page: 2, Image: base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nnot a jpeg"))}); code != 400 || !strings.Contains(out["error"].(string), "JPEG") {
		t.Fatal("non-JPEG page accepted", code, out)
	}
	if code, _ := other.do(t, s, "POST", "/api/v1/mobile/bills/page", billPageIn{Upload: up, Page: 3, Image: img}); code != 400 {
		t.Fatal("another phone added a page to this upload")
	}
	if code, out := ravi.do(t, s, "POST", "/api/v1/mobile/bills/submit", billSubmitIn{Upload: up, Connection: conn, Files: []billFile{{Name: "Camera", Pages: 3}}}); code != 400 || !strings.Contains(out["error"].(string), "did not arrive") {
		t.Fatal("submit with a missing page accepted", code, out)
	}
	if code, _ := other.do(t, s, "POST", "/api/v1/mobile/bills/submit", billSubmitIn{Upload: up, Connection: conn, Files: []billFile{{Name: "Camera", Pages: 2}}}); code != 400 {
		t.Fatal("another phone submitted this upload")
	}
	code, out = ravi.do(t, s, "POST", "/api/v1/mobile/bills/submit", billSubmitIn{Upload: up, Connection: conn, Files: []billFile{{Name: "Camera", Pages: 2}}})
	if code != 200 || out["state"] != "queued" || out["pages"].(float64) != 2 {
		t.Fatal(code, out)
	}
	if code, out := ravi.do(t, s, "POST", "/api/v1/mobile/bills/submit", billSubmitIn{Upload: up, Connection: conn, Files: []billFile{{Name: "Camera", Pages: 2}}}); code != 200 {
		t.Fatal("a repeated submit (lost reply) must succeed", code, out)
	}

	cloud.mu.Lock()
	cloud.failPages = 1 // the internet drops once mid-upload
	cloud.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.bills.Run(ctx)
	waitFor(t, "sent", func() bool {
		u, err := s.bills.load(up)
		return err == nil && u.State == "sent"
	})
	cloud.mu.Lock()
	if len(cloud.batches) != 1 {
		t.Fatal("batches:", len(cloud.batches))
	}
	for _, b := range cloud.batches {
		if b["uploaded_by"] != "Ravi's Pixel" || b["voucher_type"] != "Purchase" {
			t.Fatal(b)
		}
	}
	var pages map[int]map[string]any
	for _, p := range cloud.pages {
		pages = p
	}
	if len(pages) != 2 || pages[0]["image_b64"] != img || pages[0]["thumb_b64"] != thumb || pages[1]["image_b64"] != page2 || pages[1]["thumb_b64"] != nil {
		t.Fatal("pages did not arrive intact", len(pages))
	}
	if len(cloud.started) != 1 {
		t.Fatal("batch not started")
	}
	cloud.mu.Unlock()
	ents, _ := os.ReadDir(s.bills.path(up))
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".jpg") {
			t.Fatal("page images kept on the PC after sending:", e.Name())
		}
	}

	code, out = ravi.do(t, s, "GET", "/api/v1/mobile/bills?refresh=1", nil)
	ups := out["uploads"].([]any)
	if code != 200 || len(ups) != 1 {
		t.Fatal(code, out)
	}
	u0 := ups[0].(map[string]any)
	if u0["status"] != "Ready to review" || u0["cloud"].(map[string]any)["invoices"].([]any)[0].(map[string]any)["number"] != "180" {
		t.Fatal(u0)
	}
	if _, out := other.do(t, s, "GET", "/api/v1/mobile/bills", nil); len(out["uploads"].([]any)) != 0 {
		t.Fatal("another phone sees this phone's bills")
	}
	if list := s.bills.list(true); len(list) != 1 || list[0].Device != "Ravi's Pixel" {
		t.Fatalf("desktop list %+v", list)
	}
}

func TestBillPageSizeLimits(t *testing.T) {
	s, _ := nativeFixture(t)
	big := base64.StdEncoding.EncodeToString(append([]byte{0xFF, 0xD8, 0xFF}, make([]byte, maxBillImageBytes)...))
	if err := s.bills.addPage(billPageIn{Upload: strings.Repeat("x", 20), Image: big}, NativeDevice{ID: "d"}); err == nil {
		t.Fatal("oversized page accepted")
	}
	if err := s.bills.addPage(billPageIn{Upload: "../../etc/passwd" + strings.Repeat("x", 10), Image: "x"}, NativeDevice{ID: "d"}); err == nil {
		t.Fatal("path traversal id accepted")
	}
	if err := s.bills.addPage(billPageIn{Upload: strings.Repeat("y", 20), Page: maxBillPages, Image: base64.StdEncoding.EncodeToString(testJPEG(t, 10, 10, 1))}, NativeDevice{ID: "d"}); err == nil {
		t.Fatal("page beyond the limit accepted")
	}
}
