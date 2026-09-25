package app

import "time"

const Version = "2.0.0-native-preview"

type Settings struct {
	TallyURL    string `json:"tallyUrl"`
	TimeoutSec  int    `json:"timeoutSec"`
	CooldownSec int    `json:"cooldownSec"`
	Port        int    `json:"port"`
}

type Device struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Key      string    `json:"key,omitempty"`
	Created  time.Time `json:"createdAt"`
	LastSeen time.Time `json:"lastSeen"`
	Revoked  bool      `json:"revoked"`
}

type Company struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type RequestLog struct {
	At             time.Time `json:"at"`
	Report         string    `json:"report"`
	Company        string    `json:"company,omitempty"`
	DurationMS     int64     `json:"durationMs"`
	Success        bool      `json:"success"`
	ResponseBytes  int       `json:"responseBytes"`
	SanitizedChars int       `json:"sanitizedChars,omitempty"`
	Error          string    `json:"error,omitempty"`
}

type LiveState struct {
	TallyOnline     bool      `json:"tallyOnline"`
	TailscaleOnline bool      `json:"tailscaleOnline"`
	TailscaleStatus string    `json:"tailscaleStatus"`
	PhoneURL        string    `json:"phoneUrl"`
	Companies       []Company `json:"companies"`
	GateState       string    `json:"gateState"`
	BusyReport      string    `json:"busyReport"`
	LastSuccess     time.Time `json:"lastSuccess"`
	LastError       string    `json:"lastError"`
}

type AutoSyncState struct {
	Running   bool      `json:"running"`
	Company   string    `json:"company"`
	Step      string    `json:"step"`
	StartedAt time.Time `json:"startedAt"`
	LastOK    time.Time `json:"lastOk"`
	LastError string    `json:"lastError"`
}

type LedgerIndexRow struct {
	Name   string `json:"name"`
	Parent string `json:"parent"`
	GUID   string `json:"guid,omitempty"`
}
type LedgerIndexData struct {
	CommonData
	Ledgers []LedgerIndexRow `json:"ledgers"`
	Count   int              `json:"count"`
}

type OutstandingParty struct {
	DrCr   string  `json:"drCr,omitempty"`
	Party  string  `json:"party"`
	Amount float64 `json:"amount"`
	Parent string  `json:"parent,omitempty"`
	GUID   string  `json:"guid,omitempty"`
}
type OutstandingData struct {
	CommonData
	Parties []OutstandingParty `json:"parties"`
	Total   float64            `json:"total"`
	Count   int                `json:"count"`
	Type    string             `json:"type"`
}
type BillRow struct {
	SignedAmount   float64 `json:"signedAmount"`
	DrCr           string  `json:"drCr,omitempty"`
	Party          string  `json:"party,omitempty"`
	BillNumber     string  `json:"billNumber"`
	Date           string  `json:"date"`
	DueDate        string  `json:"dueDate,omitempty"`
	PendingAmount  float64 `json:"pendingAmount"`
	OriginalAmount float64 `json:"originalAmount,omitempty"`
	Ageing         int     `json:"ageing,omitempty"`
}
type PartyOutstandingData struct {
	DrCr string `json:"drCr,omitempty"`
	CommonData
	Party       string    `json:"party"`
	Bills       []BillRow `json:"bills"`
	Total       float64   `json:"total"`
	Count       int       `json:"count"`
	SummaryOnly bool      `json:"summaryOnly,omitempty"`
	BalanceFrom string    `json:"balanceFrom,omitempty"`
}

type VoucherRow struct {
	Debit       *float64 `json:"debit,omitempty"`
	Credit      *float64 `json:"credit,omitempty"`
	Balance     *float64 `json:"balance,omitempty"`
	MasterID    string   `json:"masterId"`
	Date        string   `json:"date"`
	Number      string   `json:"number"`
	Party       string   `json:"party"`
	Amount      float64  `json:"amount"`
	VoucherType string   `json:"voucherType,omitempty"`
}
type VoucherData struct {
	CommonData
	Vouchers []VoucherRow `json:"vouchers"`
	Total    float64      `json:"total"`
	Count    int          `json:"count"`
	Type     string       `json:"type,omitempty"`
	Period   string       `json:"period,omitempty"`
}

type InvoiceItem struct {
	Name     string  `json:"name"`
	HSN      string  `json:"hsn,omitempty"`
	Quantity string  `json:"quantity"`
	Rate     string  `json:"rate"`
	Amount   float64 `json:"amount"`
	Discount float64 `json:"discount,omitempty"`
}

type VoucherDetail struct {
	CommonData
	Company           string        `json:"company"`
	FirmName          string        `json:"firmName"`
	MasterID          string        `json:"masterId"`
	InvoiceNumber     string        `json:"invoiceNumber"`
	InvoiceDate       string        `json:"invoiceDate"`
	VoucherType       string        `json:"voucherType"`
	CustomerName      string        `json:"customerName"`
	CustomerCity      string        `json:"customerCity"`
	CustomerAddress   []string      `json:"customerAddress,omitempty"`
	GSTIN             string        `json:"gstin,omitempty"`
	DispatchedThrough string        `json:"dispatchedThrough"`
	LRNumber          string        `json:"lrNumber"`
	LRDate            string        `json:"lrDate,omitempty"`
	Destination       string        `json:"destination,omitempty"`
	VehicleNumber     string        `json:"vehicleNumber,omitempty"`
	EWayBillNumber    string        `json:"eWayBillNumber,omitempty"`
	Items             []InvoiceItem `json:"items"`
	DiscountAmount    float64       `json:"discountAmount"`
	TaxableAmount     float64       `json:"taxableAmount,omitempty"`
	CGST              float64       `json:"cgst,omitempty"`
	SGST              float64       `json:"sgst,omitempty"`
	IGST              float64       `json:"igst,omitempty"`
	Freight           float64       `json:"freight,omitempty"`
	RoundOff          float64       `json:"roundOff,omitempty"`
	TotalAmount       float64       `json:"totalAmount"`
	Narration         string        `json:"narration,omitempty"`
	ShareText         string        `json:"shareText"`
}

type LedgerStatementData struct {
	OpeningBalance *float64 `json:"openingBalance,omitempty"`
	ClosingBalance *float64 `json:"closingBalance,omitempty"`
	DebitTotal     float64  `json:"debitTotal"`
	CreditTotal    float64  `json:"creditTotal"`
	BalanceNote    string   `json:"balanceNote,omitempty"`
	CommonData
	Company   string       `json:"company"`
	Ledger    string       `json:"ledger"`
	FromDate  string       `json:"fromDate"`
	ToDate    string       `json:"toDate"`
	Vouchers  []VoucherRow `json:"vouchers"`
	Total     float64      `json:"total"`
	Count     int          `json:"count"`
	ShareText string       `json:"shareText"`
}

type LedgerOutstandingData struct {
	DrCr string `json:"drCr,omitempty"`
	CommonData
	Company     string    `json:"company"`
	Ledger      string    `json:"ledger"`
	Bills       []BillRow `json:"bills"`
	Total       float64   `json:"total"`
	Count       int       `json:"count"`
	SummaryOnly bool      `json:"summaryOnly,omitempty"`
	BalanceFrom string    `json:"balanceFrom,omitempty"`
	ShareText   string    `json:"shareText"`
}

type GroupLedgerRow struct {
	Ledger  string  `json:"ledger"`
	Parent  string  `json:"parent,omitempty"`
	Balance float64 `json:"balance"`
	DrCr    string  `json:"drCr,omitempty"`
	GUID    string  `json:"guid,omitempty"`
}

type GroupBalanceData struct {
	CommonData
	Company   string           `json:"company"`
	Group     string           `json:"group"`
	Ledgers   []GroupLedgerRow `json:"ledgers"`
	Total     float64          `json:"total"`
	DrTotal   float64          `json:"drTotal"`
	CrTotal   float64          `json:"crTotal"`
	Count     int              `json:"count"`
	ShareText string           `json:"shareText"`
}

type CommonData struct {
	Live       bool      `json:"live"`
	Source     string    `json:"source,omitempty"`
	Stale      bool      `json:"stale,omitempty"`
	FetchedAt  time.Time `json:"fetchedAt"`
	LiveError  string    `json:"liveError,omitempty"`
	DurationMS int64     `json:"durationMs,omitempty"`
}

type Summary struct {
	CommonData
	Receivable *float64  `json:"receivable,omitempty"`
	Payable    *float64  `json:"payable,omitempty"`
	Sales      *float64  `json:"sales,omitempty"`
	RCount     *int      `json:"rCount,omitempty"`
	PCount     *int      `json:"pCount,omitempty"`
	SCount     *int      `json:"sCount,omitempty"`
	RFetchedAt time.Time `json:"rFetchedAt,omitempty"`
	PFetchedAt time.Time `json:"pFetchedAt,omitempty"`
	SFetchedAt time.Time `json:"sFetchedAt,omitempty"`
}

type BankDetails struct {
	Ledger        string `json:"ledger"`
	BankName      string `json:"bankName,omitempty"`
	AccountHolder string `json:"accountHolder,omitempty"`
	AccountNumber string `json:"accountNumber,omitempty"`
	IFSC          string `json:"ifsc,omitempty"`
	Branch        string `json:"branch,omitempty"`
	UPI           string `json:"upi,omitempty"`
}

type BankListData struct {
	CommonData
	Company string        `json:"company"`
	Banks   []BankDetails `json:"banks"`
	Count   int           `json:"count"`
}

type SearchResult struct {
	Kind        string  `json:"kind"`
	Title       string  `json:"title"`
	Subtitle    string  `json:"subtitle,omitempty"`
	MasterID    string  `json:"masterId,omitempty"`
	Ledger      string  `json:"ledger,omitempty"`
	Party       string  `json:"party,omitempty"`
	Number      string  `json:"number,omitempty"`
	Date        string  `json:"date,omitempty"`
	LRNumber    string  `json:"lrNumber,omitempty"`
	Amount      float64 `json:"amount,omitempty"`
	VoucherType string  `json:"voucherType,omitempty"`
}

type SearchIndexData struct {
	CommonData
	Company string         `json:"company"`
	Results []SearchResult `json:"results"`
	Count   int            `json:"count"`
}

type AgeingBucket struct {
	Label  string  `json:"label"`
	Amount float64 `json:"amount"`
	Bills  int     `json:"bills"`
}

type AgeingData struct {
	CommonData
	Company         string         `json:"company"`
	Buckets         []AgeingBucket `json:"buckets"`
	Total           float64        `json:"total"`
	BillCount       int            `json:"billCount"`
	CoveredParties  int            `json:"coveredParties"`
	TotalParties    int            `json:"totalParties"`
	CoveragePercent int            `json:"coveragePercent"`
}

type Customer360Data struct {
	CommonData
	Company          string       `json:"company"`
	Ledger           string       `json:"ledger"`
	Outstanding      float64      `json:"outstanding"`
	Overdue          float64      `json:"overdue"`
	PendingBillCount int          `json:"pendingBillCount"`
	LatestInvoices   []VoucherRow `json:"latestInvoices"`
	RecentActivity   []VoucherRow `json:"recentActivity"`
	LastPayment      *VoucherRow  `json:"lastPayment,omitempty"`
	ShareText        string       `json:"shareText"`
}
