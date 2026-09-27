package app

import (
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ProformaItem struct {
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	HSN            string  `json:"hsn"`
	Unit           string  `json:"unit"`
	Quantity       string  `json:"quantity"`
	Rate           string  `json:"rate"`
	Discount       string  `json:"discount"`
	GST            string  `json:"gst"`
	Gross          float64 `json:"gross"`
	DiscountAmount float64 `json:"discountAmount"`
	Taxable        float64 `json:"taxable"`
	CGST           float64 `json:"cgst"`
	SGST           float64 `json:"sgst"`
	IGST           float64 `json:"igst"`
	Total          float64 `json:"total"`
}
type Proforma struct {
	Company         string         `json:"company"`
	Number          string         `json:"number"`
	Date            string         `json:"date"`
	SupplierGST     string         `json:"supplierGST"`
	SupplierState   string         `json:"supplierState"`
	SupplierAddress string         `json:"supplierAddress"`
	SupplierPhone   string         `json:"supplierPhone,omitempty"`
	SupplierEmail   string         `json:"supplierEmail,omitempty"`
	Buyer           string         `json:"buyer"`
	BuyerGST        string         `json:"buyerGST"`
	City            string         `json:"city"`
	Address         string         `json:"address"`
	PlaceState      string         `json:"placeState"`
	Dispatch        string         `json:"dispatch"`
	LR              string         `json:"lr"`
	Notes           string         `json:"notes"`
	Items           []ProformaItem `json:"items"`
	Gross           float64        `json:"gross"`
	Discount        float64        `json:"discountAmount"`
	Taxable         float64        `json:"taxable"`
	CGST            float64        `json:"cgst"`
	SGST            float64        `json:"sgst"`
	IGST            float64        `json:"igst"`
	RoundOff        float64        `json:"roundOff"`
	Total           float64        `json:"total"`
	ShareText       string         `json:"shareText"`
	ShareTextWithBank string       `json:"shareTextWithBank,omitempty"`
	Bank            BankDetails    `json:"bank"`
}

var decimalInput = regexp.MustCompile(`^\d{1,10}(\.\d{1,4})?$`)
var gstPattern = regexp.MustCompile(`^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$`)

func validState(s string) bool {
	v, e := strconv.Atoi(s)
	return e == nil && len(s) == 2 && v >= 1 && v <= 38
}
func decimal(s string, max int64) (*big.Rat, error) {
	if !decimalInput.MatchString(s) {
		return nil, errors.New("enter a valid non-negative decimal")
	}
	n, ok := new(big.Rat).SetString(s)
	if !ok || n.Cmp(big.NewRat(max, 1)) > 0 {
		return nil, errors.New("number is above the allowed limit")
	}
	return n, nil
}
func cents(n *big.Rat) int64 {
	r := new(big.Rat).Mul(n, big.NewRat(100, 1))
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(r.Num(), r.Denom(), rem)
	if new(big.Int).Mul(rem, big.NewInt(2)).Cmp(r.Denom()) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	return q.Int64()
}
func currencyFromCents(n int64) float64 { return float64(n) / 100 }
func calculateProforma(p Proforma) (Proforma, error) {
	p.SupplierGST = strings.ToUpper(strings.TrimSpace(p.SupplierGST))
	p.BuyerGST = strings.ToUpper(strings.TrimSpace(p.BuyerGST))
	for _, s := range []string{p.Company, p.Buyer, p.Number} {
		if strings.TrimSpace(s) == "" || len(s) > 160 {
			return p, errors.New("company, buyer and proforma number are required (up to 160 characters)")
		}
	}
	if _, e := time.Parse("2006-01-02", p.Date); e != nil {
		return p, errors.New("use a valid invoice date (YYYY-MM-DD)")
	}
	for _, s := range []string{p.SupplierGST, p.BuyerGST} {
		if s != "" && (!gstPattern.MatchString(s) || !validState(s[:2])) {
			return p, errors.New("check the 15-character GSTIN and state code")
		}
	}
	if !validState(p.SupplierState) || !validState(p.PlaceState) {
		return p, errors.New("select supplier and place-of-supply state codes")
	}
	if p.SupplierGST != "" && p.SupplierGST[:2] != p.SupplierState {
		return p, errors.New("supplier GSTIN does not match supplier state")
	}
	for _, s := range []string{p.SupplierAddress, p.Address, p.Notes, p.Dispatch, p.LR, p.City} {
		if len(s) > 2000 {
			return p, errors.New("a text field is too long")
		}
	}
	if len(p.Items) == 0 || len(p.Items) > 100 {
		return p, errors.New("add between 1 and 100 Tally stock items")
	}
	gross, discount, taxable, cgst, sgst, igst := int64(0), int64(0), int64(0), int64(0), int64(0), int64(0)
	for i := range p.Items {
		row := &p.Items[i]
		if row.Name == "" || len(row.Name) > 300 || len(row.Description) > 2000 || len(row.HSN) > 20 || len(row.Unit) > 30 {
			return p, fmt.Errorf("check item %d name and description", i+1)
		}
		qty, e := decimal(strings.TrimSpace(row.Quantity), 1000000)
		if e != nil || qty == nil || qty.Sign() <= 0 {
			return p, fmt.Errorf("item %d: quantity must be positive", i+1)
		}
		rate, e := decimal(strings.TrimSpace(row.Rate), 10000000)
		if e != nil {
			return p, fmt.Errorf("item %d: enter a valid unit rate", i+1)
		}
		disc, e := decimal(strings.TrimSpace(row.Discount), 100)
		if e != nil {
			return p, fmt.Errorf("item %d: discount must be 0 to 100 percent", i+1)
		}
		gst, e := decimal(strings.TrimSpace(row.GST), 100)
		if e != nil {
			return p, fmt.Errorf("item %d: confirm GST percentage (0 for exempt)", i+1)
		}
		product := new(big.Rat).Mul(qty, rate)
		if product.Cmp(big.NewRat(1000000000, 1)) > 0 {
			return p, errors.New("line value is too large")
		}
		g := cents(product)
		d := cents(new(big.Rat).Mul(big.NewRat(g, 100), new(big.Rat).Quo(disc, big.NewRat(100, 1))))
		net := g - d
		cg, sg, ig := int64(0), int64(0), int64(0)
		if p.SupplierState == p.PlaceState {
			cg = cents(new(big.Rat).Mul(big.NewRat(net, 100), new(big.Rat).Quo(gst, big.NewRat(200, 1))))
			sg = cg
		} else {
			ig = cents(new(big.Rat).Mul(big.NewRat(net, 100), new(big.Rat).Quo(gst, big.NewRat(100, 1))))
		}
		row.Gross = currencyFromCents(g)
		row.DiscountAmount = currencyFromCents(d)
		row.Taxable = currencyFromCents(net)
		row.CGST = currencyFromCents(cg)
		row.SGST = currencyFromCents(sg)
		row.IGST = currencyFromCents(ig)
		row.Total = currencyFromCents(net + cg + sg + ig)
		gross += g
		discount += d
		taxable += net
		cgst += cg
		sgst += sg
		igst += ig
	}
	p.Gross = currencyFromCents(gross)
	p.Discount = currencyFromCents(discount)
	p.Taxable = currencyFromCents(taxable)
	p.CGST = currencyFromCents(cgst)
	p.SGST = currencyFromCents(sgst)
	p.IGST = currencyFromCents(igst)
	beforeRound := taxable + cgst + sgst + igst
	rounded := ((beforeRound + 50) / 100) * 100
	p.RoundOff = currencyFromCents(rounded - beforeRound)
	p.Total = currencyFromCents(rounded)
	p.ShareText = proformaText(p, false)
	p.ShareTextWithBank = proformaText(p, true)
	return p, nil
}
func proformaText(p Proforma, includeBank bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*PROFORMA INVOICE*\nNot a tax invoice\n%s\nProforma no: %s\nDate: %s\nBuyer: %s\n", p.Company, p.Number, displayDate(p.Date), p.Buyer)
	for _, f := range [][2]string{{"Supplier GSTIN", p.SupplierGST}, {"Supplier mobile", p.SupplierPhone}, {"Supplier address", p.SupplierAddress}, {"Buyer GSTIN", p.BuyerGST}, {"City", p.City}, {"Buyer address", p.Address}, {"Place of supply (state code)", p.PlaceState}, {"Dispatched through", p.Dispatch}, {"LR / RR no", p.LR}} {
		if f[1] != "" { fmt.Fprintf(&b, "%s: %s\n", f[0], f[1]) }
	}
	for i, r := range p.Items {
		fmt.Fprintf(&b, "\n*%d. %s*\n", i+1, r.Name)
		if r.Description != "" { b.WriteString(r.Description + "\n") }
		fmt.Fprintf(&b, "Qty: %s %s | Rate: ₹%s\nDiscount: %s%% | GST: %s%%\nNet: ₹%s | Including GST: ₹%s\n", r.Quantity, r.Unit, r.Rate, r.Discount, r.GST, indianNumber(r.Taxable), indianNumber(r.Total))
	}
	fmt.Fprintf(&b, "\nItems: ₹%s\nDiscount: ₹%s\nTaxable: ₹%s", indianNumber(p.Gross), indianNumber(p.Discount), indianNumber(p.Taxable))
	for _, f := range []struct{ label string; value float64 }{{"CGST", p.CGST}, {"SGST", p.SGST}, {"IGST", p.IGST}} {
		if f.value != 0 { fmt.Fprintf(&b, "\n%s: ₹%s", f.label, indianNumber(f.value)) }
	}
	fmt.Fprintf(&b, "\nRound off: ₹%s\n*Total: ₹%s*", indianNumber(p.RoundOff), indianNumber(p.Total))
	if p.Notes != "" { b.WriteString("\n\n" + p.Notes) }
	if includeBank { b.WriteString(bankText(p.Bank)) }
	return b.String()
}
func (s *Server) handleProforma(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "POST" {
		jsonErr(w, 405, errors.New("POST required"))
		return
	}
	var p Proforma
	if readJSON(r, &p) != nil {
		jsonErr(w, 400, errors.New("invalid proforma"))
		return
	}
	var masters StockItemsData
	if _, ok := s.cache.Get("stock-items", p.Company, "", &masters); !ok {
		jsonErr(w, 409, errors.New("load Tally stock items before preparing a proforma"))
		return
	}
	names := map[string]bool{}
	for _, it := range masters.Items {
		names[it.Name] = true
	}
	for _, it := range p.Items {
		if !names[it.Name] {
			jsonErr(w, 400, errors.New("choose items from this company's Tally stock list"))
			return
		}
	}
	profile := s.savedCompanyProfile(p.Company)
	if profile.GSTIN != "" { p.SupplierGST = profile.GSTIN }
	if address := companyProfileAddress(profile); address != "" { p.SupplierAddress = address }
	if profile.StateCode != "" { p.SupplierState = profile.StateCode }
	p.SupplierPhone = profile.Phone
	p.SupplierEmail = profile.Email
	p.Bank, _ = s.store.PDFBank(p.Company)
	result, e := calculateProforma(p)
	if e != nil {
		jsonErr(w, 400, e)
		return
	}
	if strings.HasSuffix(r.URL.Path, "-pdf") {
		data, e := renderPDFMode(proformaHTML(result), false)
		if e != nil {
			jsonErr(w, 503, e)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="Proforma.pdf"`)
		_, _ = w.Write(data)
		return
	}
	jsonOut(w, result)
}
func proformaHTML(p Proforma) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="note"><b>PROFORMA ONLY - NOT A TAX INVOICE</b><br>For quotation and payment discussion.</div><div class="grid"><section class="panel"><h3>From</h3><p><b>%s</b></p><p>%s</p><p>GSTIN: %s</p><p>Mobile: %s</p><p>State code: %s</p></section><section class="panel"><h3>Bill to</h3><p><b>%s</b></p><p>%s</p><p>%s</p><p>GSTIN: %s</p><p>Place of supply: %s</p></section></div><table class="kv"><tr><td>Proforma number</td><td>%s</td></tr><tr><td>Date</td><td>%s</td></tr><tr><td>Dispatched through</td><td>%s</td></tr><tr><td>LR / RR number</td><td>%s</td></tr></table>`, he(p.Company), he(p.SupplierAddress), he(p.SupplierGST), he(p.SupplierPhone), he(p.SupplierState), he(p.Buyer), he(p.Address), he(p.City), he(p.BuyerGST), he(p.PlaceState), he(p.Number), he(displayDate(p.Date)), he(p.Dispatch), he(p.LR))
	b.WriteString(`<table class="items"><colgroup><col style="width:5%"><col style="width:29%"><col style="width:12%"><col style="width:14%"><col style="width:10%"><col style="width:10%"><col style="width:20%"></colgroup><thead><tr><th>#</th><th>Item / HSN</th><th class="num">Qty</th><th class="num">Rate</th><th class="num">Disc %</th><th class="num">GST %</th><th class="num">Taxable INR</th></tr></thead><tbody>`)
	for i, r := range p.Items {
		fmt.Fprintf(&b, `<tr><td>%d</td><td><b>%s</b>%s%s</td><td class="num">%s %s</td><td class="num">%s</td><td class="num">%s</td><td class="num">%s</td><td class="num">%s</td></tr>`, i+1, he(r.Name), descriptionHTML(r.Description), descriptionHTML("HSN: "+r.HSN), he(r.Quantity), he(r.Unit), he(r.Rate), he(r.Discount), he(r.GST), he(indianNumber(r.Taxable)))
	}
	b.WriteString(`</tbody></table><style>.proforma-bottom{display:table;width:100%;table-layout:fixed;break-inside:avoid}.proforma-bottom>section{display:table-cell;vertical-align:top}.proforma-bottom .bank{margin-top:0;padding:3mm;font-size:8pt}.proforma-bottom .totals{width:100%;margin:0;font-size:8.5pt}.proforma-bottom .totals td{padding:1.6mm 1.5mm}.proforma-bottom .grand td{font-size:10pt}</style><div class="proforma-bottom"><section style="width:44%;padding-right:5mm">`)
	if p.Bank.AccountNumber != "" || p.Bank.UPI != "" { b.WriteString(bankHTML([]BankDetails{p.Bank}, PaymentQR{Amount: p.Total, Reference: "Proforma "+p.Number, Payee: p.Company})) }
	b.WriteString(`</section><section style="width:56%"><table class="totals">`)
	for _, f := range []struct{ label string; value float64 }{{"Items subtotal", p.Gross}, {"Discount (deducted)", -p.Discount}, {"Taxable amount", p.Taxable}, {"CGST", p.CGST}, {"SGST", p.SGST}, {"IGST", p.IGST}, {"Round off", p.RoundOff}} {
		if f.value != 0 || f.label == "Round off" { fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td></tr>", he(f.label), he(inr(f.value))) }
	}
	fmt.Fprintf(&b, `<tr class="grand"><td>Total INR</td><td>%s</td></tr></table>`, he(inr(p.Total)))
	b.WriteString(`</section></div>`)
	if p.Notes != "" { b.WriteString(`<div class="narration">` + he(p.Notes) + `</div>`) }
	profile := CompanyProfile{Name: p.Company, GSTIN: p.SupplierGST, Address: p.SupplierAddress, Phone: p.SupplierPhone, Email: p.SupplierEmail, StateCode: p.SupplierState}
	return pdfDocument(p.Company, "PROFORMA INVOICE", p.Number+" | "+displayDate(p.Date), b.String(), profile)
}
