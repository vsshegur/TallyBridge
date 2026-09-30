package app

import (
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
	if !strings.Contains(o.ShareText, "Rate: ₹12.50") || !strings.Contains(o.ShareText, "Order Total: ₹125.00") {
		t.Fatalf("priced order share text is incomplete: %s", o.ShareText)
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
	dispatched, err := s.dispatchOrder(o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if dispatched.Status != "dispatched" || dispatched.ShareText != "" || dispatched.DispatchedAt.IsZero() {
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
	if _, err := s.dispatchOrder(o.ID); err != nil {
		t.Fatal(err)
	}
	updated.Items[0].Quantity = 10
	if _, err := s.saveOrder(updated); err == nil {
		t.Fatal("expected dispatched order edit to be rejected")
	}
}
