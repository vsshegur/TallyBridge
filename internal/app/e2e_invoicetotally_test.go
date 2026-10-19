package app

// End-to-end run against a real InvoiceTotally server and a Tally (or the mock
// in the InvoiceTotally repo). Skipped unless TB_E2E_CLOUD is set:
//
//	TB_E2E_CLOUD=http://127.0.0.1:8787 TB_E2E_TOKEN=itb_… TB_E2E_TALLY=http://127.0.0.1:9100 \
//	TB_E2E_IMAGE=bill.jpg TB_E2E_OUT=/tmp/e2e go test ./internal/app -run TestE2EPhoneBillToTally -v -timeout 15m
//
// The phone sends the bill through this PC to InvoiceTotally; someone reviews and
// pushes it in the browser; the PC posts it to Tally; the phone sees "In Tally".

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/jpeg"
	_ "image/png"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func shrinkJPEG(t *testing.T, src []byte, maxSide int, q int) []byte {
	t.Helper()
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	k := 1.0
	if m := max(w, h); m > maxSide {
		k = float64(maxSide) / float64(m)
	}
	nw, nh := max(1, int(float64(w)*k)), max(1, int(float64(h)*k))
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			dst.Set(x, y, img.At(b.Min.X+int(float64(x)/k), b.Min.Y+int(float64(y)/k)))
		}
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: q}); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestE2EPhoneBillToTally(t *testing.T) {
	cloudURL := os.Getenv("TB_E2E_CLOUD")
	if cloudURL == "" {
		t.Skip("set TB_E2E_CLOUD to run the end-to-end test")
	}
	out := os.Getenv("TB_E2E_OUT")
	s, rsaKey := nativeFixture(t)
	cfg := s.store.Settings()
	cfg.TallyURL = os.Getenv("TB_E2E_TALLY")
	if err := s.store.UpdateSettings(cfg); err != nil {
		t.Fatal(err)
	}
	// Desktop admin page for screenshots.
	ln, err := net.Listen("tcp", "127.0.0.1:8765")
	if err == nil {
		srv := &http.Server{Handler: s.DesktopHandlerWithOrders()}
		go srv.Serve(ln)
		defer srv.Close()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := s.it.save(ctx, cloudURL, []string{os.Getenv("TB_E2E_TOKEN")}, nil); err != nil {
		t.Fatal(err)
	}
	if w := adminPost(t, s, "/api/v1/admin/invoicetotally/options", `{"postingEnabled":true}`); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	go s.bills.Run(ctx)
	defer s.it.Stop()

	ph := pairPhone(t, s, rsaKey, "Ravi's Pixel")
	_, info := ph.do(t, s, "GET", "/api/v1/mobile/bills", nil)
	conn := info["companies"].([]any)[0].(map[string]any)["id"].(string)
	src, err := os.ReadFile(os.Getenv("TB_E2E_IMAGE"))
	if err != nil {
		t.Fatal(err)
	}
	full, thumb := shrinkJPEG(t, src, 1800, 86), shrinkJPEG(t, src, 900, 60)
	up := "e2e-" + randomHex(10)
	if code, o := ph.do(t, s, "POST", "/api/v1/mobile/bills/page", billPageIn{Upload: up, Image: base64.StdEncoding.EncodeToString(full), Thumb: base64.StdEncoding.EncodeToString(thumb)}); code != 200 {
		t.Fatal(code, o)
	}
	if code, o := ph.do(t, s, "POST", "/api/v1/mobile/bills/submit", billSubmitIn{Upload: up, Connection: conn, Files: []billFile{{Name: "Camera", Pages: 1}}}); code != 200 {
		t.Fatal(code, o)
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		u, _ := s.bills.load(up)
		if u != nil && u.State == "sent" {
			t.Logf("bill reached InvoiceTotally as batch %s (%s)", u.BatchID, u.BatchName)
			if out != "" {
				os.WriteFile(filepath.Join(out, "bid"), []byte(u.BatchID), 0644)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("not sent: %+v", u)
		}
		time.Sleep(300 * time.Millisecond)
	}
	last := ""
	deadline = time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		_, o := ph.do(t, s, "GET", "/api/v1/mobile/bills?refresh=1", nil)
		u0 := o["uploads"].([]any)[0].(map[string]any)
		if st := u0["status"].(string); st != last {
			t.Logf("phone shows: %s", st)
			last = st
		}
		if last == "In Tally" {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if last != "In Tally" {
		t.Fatal("bill never reached Tally; last status:", last)
	}
	for _, a := range s.it.snapshot()["activity"].([]ITActivity) {
		t.Logf("activity: %s %s", a.At.Format("15:04:05"), a.Text)
	}
	if out != "" {
		os.WriteFile(filepath.Join(out, "done"), []byte("ok"), 0644)
		// keep the desktop page up briefly for screenshots
		for i := 0; i < 30; i++ {
			if _, err := os.Stat(filepath.Join(out, "shot-done")); err == nil {
				break
			}
			time.Sleep(time.Second)
		}
	}
}
