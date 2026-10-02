package app

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"
)

func newOrderTestServer(t *testing.T) *Server {
	t.Helper()
	s, err := NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func ptrRate(v float64) *float64 { return &v }

func TestOrderCanBeSavedWithoutPriceAndSharedWhilePending(t *testing.T) {
	s := newOrderTestServer(t)
	o, err := s.saveOrder(SalesOrder{
		Company: "Vijayalaxmi Textiles",
		Buyer:   "ABC Traders",
		BuyerDetails: map[string]string{
			"MOBILENUMBER": "9876543210",
			"ADDRESS":      "Market Road",
			"CITY":         "Solapur",
		},
		Items: []OrderItem{{Name: "Bath Towel", Unit: "Pcs", Quantity: 20}},
		Notes: "Assorted colours",
	})
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != "pending" || o.Number == "" || o.ID == "" {
		t.Fatalf("unexpected saved order: %#v", o)
	}
	for _, want := range []string{"*ORDER DETAILS*", "ABC Traders", "Bath Towel", "Qty: 20 Pcs", "Assorted colours"} {
		if !strings.Contains(o.ShareText, want) {
			t.Fatalf("share text missing %q: %s", want, o.ShareText)
		}
	}
	if strings.Contains(o.ShareText, "Rate:") || strings.Contains(o.ShareText, "Order Total:") {
		t.Fatalf("price must stay absent when no rate was entered: %s", o.ShareText)
	}
}

func TestOrderRateIsOptionalButIncludedWhenProvided(t *testing.T) {
	s := newOrderTestServer(t)
	o, err := s.saveOrder(SalesOrder{
		Company: "Vipul Textiles",
		Buyer:   "Customer",
		Items:   []OrderItem{{Name: "Kitchen Napkin", Unit: "Pcs", Quantity: 10, Rate: ptrRate(12.5)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(o.ShareText, "Rate: ₹12.50") {
		t.Fatalf("priced order share text is missing rate: %s", o.ShareText)
	}
	if strings.Contains(o.ShareText, "Amount:") || strings.Contains(o.ShareText, "Order Total:") {
		t.Fatalf("order sharing must show rate only, never calculated amounts: %s", o.ShareText)
	}
}

func TestDispatchedOrderHasNoShareTextAndIsRemovedAfterSevenDays(t *testing.T) {
	s := newOrderTestServer(t)
	o, err := s.saveOrder(SalesOrder{
		Company: "Vijayalaxmi Textiles",
		Buyer:   "ABC Traders",
		Items:   []OrderItem{{Name: "Towel", Quantity: 5}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatched, err := s.dispatchOrder(o.ID, "VRL Logistics")
	if err != nil {
		t.Fatal(err)
	}
	if dispatched.Status != "dispatched" || dispatched.ShareText != "" || dispatched.DispatchedAt.IsZero() || dispatched.DispatchedThrough != "VRL Logistics" {
		t.Fatalf("unexpected dispatched order: %#v", dispatched)
	}

	orderMu.Lock()
	orders, err := s.loadOrdersLocked(time.Now())
	if err != nil {
		orderMu.Unlock()
		t.Fatal(err)
	}
	orders[0].DispatchedAt = time.Now().Add(-7*24*time.Hour - time.Minute)
	if err := s.saveOrdersLocked(orders); err != nil {
		orderMu.Unlock()
		t.Fatal(err)
	}
	orderMu.Unlock()

	list, err := s.listOrders("")
	if err != nil {
		t.Fatal(err)
	}
	if list.PendingCount != 0 || list.DispatchedCount != 0 {
		t.Fatalf("expired dispatched order was not physically removed: %#v", list)
	}
}

func TestPendingOrderCanBeEditedButDispatchedCannot(t *testing.T) {
	s := newOrderTestServer(t)
	o, err := s.saveOrder(SalesOrder{Company: "Firm", Buyer: "Buyer", Items: []OrderItem{{Name: "Item", Quantity: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	o.Items[0].Quantity = 9
	updated, err := s.saveOrder(o)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Items[0].Quantity != 9 || updated.Number != o.Number {
		t.Fatalf("pending edit did not preserve identity: %#v", updated)
	}
	if _, err := s.dispatchOrder(o.ID, ""); err != nil {
		t.Fatal(err)
	}
	updated.Items[0].Quantity = 10
	if _, err := s.saveOrder(updated); err == nil {
		t.Fatal("expected dispatched order edit to be rejected")
	}
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 12, 12))
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			img.Set(x, y, color.RGBA{R: 30, G: uint8(80 + x), B: uint8(120 + y), A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestPhotoOrderCanBePhotoOnlyDeduplicatesAndDeletesPhotoAfterSevenDays(t *testing.T) {
	s := newOrderTestServer(t)
	if _, err := s.saveOrder(SalesOrder{Company: "Firm", Buyer: "Buyer"}); err == nil {
		t.Fatal("ordinary empty order should still be rejected")
	}
	o, err := s.saveOrder(SalesOrder{Company: "Vijayalaxmi Textiles", Buyer: "Photo Customer", PhotoOrder: true, Notes: "Customer sent this on WhatsApp"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.PhotoOrder || len(o.Items) != 0 {
		t.Fatalf("expected photo-only order: %#v", o)
	}
	data := tinyPNG(t)
	q := orderPhotoUpload{OrderID: o.ID, Name: "whatsapp-order.png", ContentType: "image/png", Data: base64.StdEncoding.EncodeToString(data)}
	att, err := s.saveOrderPhoto(q)
	if err != nil {
		t.Fatal(err)
	}
	if att.ID == "" || att.Size != int64(len(data)) || att.SHA256 == "" {
		t.Fatalf("unexpected attachment: %#v", att)
	}
	if _, err := os.Stat(s.orderAttachmentPath(att)); err != nil {
		t.Fatalf("photo not stored: %v", err)
	}
	again, err := s.saveOrderPhoto(q)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != att.ID {
		t.Fatalf("retry created duplicate attachment: first=%s second=%s", att.ID, again.ID)
	}
	list, err := s.listOrders("")
	if err != nil {
		t.Fatal(err)
	}
	if list.PendingCount != 1 || len(list.Pending[0].Attachments) != 1 || !strings.Contains(list.Pending[0].ShareText, "Photo order: 1 image") {
		t.Fatalf("photo order missing from list: %#v", list)
	}
	if _, err := s.dispatchOrder(o.ID, ""); err != nil {
		t.Fatal(err)
	}
	orderMu.Lock()
	orders, err := s.loadOrdersLocked(time.Now())
	if err != nil {
		orderMu.Unlock()
		t.Fatal(err)
	}
	orders[0].DispatchedAt = time.Now().Add(-7*24*time.Hour - time.Minute)
	if err := s.saveOrdersLocked(orders); err != nil {
		orderMu.Unlock()
		t.Fatal(err)
	}
	orderMu.Unlock()
	if _, err := s.listOrders(""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.orderAttachmentPath(att)); !os.IsNotExist(err) {
		t.Fatalf("expired photo should be removed with the order, stat err=%v", err)
	}
}
