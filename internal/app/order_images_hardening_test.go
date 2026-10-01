package app

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"
)

func pngWithSize(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	if width > 0 && height > 0 {
		img.Set(0, 0, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestOrderPhotoRejectsOversizedDimensions(t *testing.T) {
	data := pngWithSize(t, maxOrderPhotoSide+1, 1)
	if err := validatePhotoBytes("image/png", data); err == nil || !strings.Contains(err.Error(), "dimensions") {
		t.Fatalf("expected oversized-dimension rejection, got %v", err)
	}
}

func TestOrderPhotoResponseUsesPrivateNoStoreHeaders(t *testing.T) {
	s := newOrderTestServer(t)
	o, err := s.saveOrder(SalesOrder{Company: "Firm", Buyer: "Photo Buyer", PhotoOrder: true})
	if err != nil {
		t.Fatal(err)
	}
	data := tinyPNG(t)
	att, err := s.saveOrderPhoto(orderPhotoUpload{OrderID: o.ID, Name: "customer.png", ContentType: "image/png", Data: base64.StdEncoding.EncodeToString(data)})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/api/v1/admin/orders/image?order="+o.ID+"&attachment="+att.ID, nil)
	w := httptest.NewRecorder()
	s.serveOrderPhoto(w, r)
	if w.Code != 200 {
		t.Fatalf("unexpected status %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("unexpected cache policy %q", got)
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected nosniff header, got %q", got)
	}
	if got := w.Header().Get("Cross-Origin-Resource-Policy"); got != "same-origin" {
		t.Fatalf("expected same-origin resource policy, got %q", got)
	}
}
