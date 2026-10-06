package app

import (
	"strings"
	"testing"
)

func TestGroupOutstandingOldestFirstAcrossLedgers(t *testing.T) {
	bills := []BillRow{
		{Party: "ABC", BillNumber: "ABC-30", Date: "2026-09-06", PendingAmount: 3000, InvoiceDays: 30, DrCr: "Dr"},
		{Party: "XYZ", BillNumber: "XYZ-75", Date: "2026-07-23", PendingAmount: 7500, InvoiceDays: 75, DrCr: "Dr"},
		{Party: "ABC", BillNumber: "ABC-90", Date: "2026-07-08", PendingAmount: 9000, InvoiceDays: 90, DrCr: "Dr"},
		{Party: "XYZ", BillNumber: "XYZ-45", Date: "2026-08-22", PendingAmount: 4500, InvoiceDays: 45, DrCr: "Dr"},
		{Party: "ABC", BillNumber: "ABC-60", Date: "2026-08-07", PendingAmount: 6000, InvoiceDays: 60, DrCr: "Dr"},
	}
	got := sortedBills(bills)
	want := []string{"ABC-90", "XYZ-75", "ABC-60", "XYZ-45", "ABC-30"}
	if len(got) != len(want) {
		t.Fatalf("got %d bills, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].BillNumber != want[i] {
			t.Fatalf("position %d = %s, want %s", i, got[i].BillNumber, want[i])
		}
	}
}

func TestGroupOutstandingShareHasGroupOnceAndBillDetails(t *testing.T) {
	x := GroupOutstandingData{
		Company: "VIJAYALAXMI TEXTILES",
		Group:   "Sundry Debtors",
		Bills: []BillRow{
			{Party: "ABC", BillNumber: "INV-001", Date: "2026-07-08", PendingAmount: 9000, InvoiceDays: 90, DrCr: "Dr"},
			{Party: "XYZ", BillNumber: "INV-002", Date: "2026-07-23", PendingAmount: 7500, InvoiceDays: 75, DrCr: "Dr"},
		},
		Total: 16500,
		Count: 2,
	}
	text := groupOutstandingShareText(x)
	if strings.Count(text, "Group: Sundry Debtors") != 1 {
		t.Fatalf("parent group must appear exactly once, got:\n%s", text)
	}
	for _, want := range []string{"ABC", "XYZ", "INV-001", "INV-002", "08 Jul 2026", "23 Jul 2026", "90 days", "75 days", "₹9,000", "₹7,500"} {
		if !strings.Contains(text, want) {
			t.Fatalf("share text missing %q:\n%s", want, text)
		}
	}
	if strings.Index(text, "INV-001") > strings.Index(text, "INV-002") {
		t.Fatalf("oldest bill must appear first:\n%s", text)
	}
}
