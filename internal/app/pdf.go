package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Embedded fonts keep Marathi/Hindi text intact on PCs without extra fonts.
//
//go:embed pdf-fonts/*.ttf
var pdfFonts embed.FS
var pdfGate = make(chan struct{}, 1)
var renderedPDFMu sync.Mutex
var renderedPDFs = map[[32]byte][]byte{}
var renderedPDFBytes int
var fontCSS string
var fontOnce sync.Once

func pdfFontCSS() string {
	fontOnce.Do(func() {
		for _, f := range []struct{ file, name string }{{"NotoSans-Regular.ttf", "ReportSans"}, {"NotoSansDevanagari-Regular.ttf", "ReportDevanagari"}} {
			b, _ := pdfFonts.ReadFile("pdf-fonts/" + f.file)
			fontCSS += "@font-face{font-family:" + f.name + ";src:url(data:font/ttf;base64," + base64.StdEncoding.EncodeToString(b) + ") format('truetype');font-weight:100 900;}"
		}
	})
	return fontCSS
}
func he(s string) string   { return html.EscapeString(s) }
func inr(v float64) string { return "INR " + indianNumber(v) }
func pdfDataNote(x CommonData) string {
	if x.FetchedAt.IsZero() {
		return "Data freshness not recorded"
	}
	label := "Last updated"
	if x.Live {
		label = "Read from Tally"
	}
	return label + ": " + x.FetchedAt.Local().Format("02 Jan 2006, 15:04")
}
func pdfDocument(company, title, subtitle, body string) string {
	return `<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; font-src data:; img-src data:"><style>` + pdfFontCSS() + `
 @page{size:A4;margin:16mm 13mm 18mm;@bottom-left{content:"TallyBridge  |  READ ONLY";font:8pt Arial;color:#68758a;}@bottom-right{content:"Page " counter(page) " of " counter(pages);font:8pt Arial;color:#68758a;}}
 *{box-sizing:border-box}body{margin:0;font:9pt ReportSans,ReportDevanagari,Arial,sans-serif;color:#22334b;line-height:1.45;overflow-wrap:anywhere}
 h1{font-size:22pt;line-height:1.25;letter-spacing:-.4pt;margin:0 0 4mm;color:#132c49}h2{font-size:12pt;margin:0 0 3mm}h3{font-size:9pt;margin:0 0 2mm;text-transform:uppercase;letter-spacing:.5pt;color:#52677f}
 .heading{border-bottom:2pt solid #193f62;padding-bottom:4mm;margin-bottom:5mm}.caption{font-size:8pt;color:#65758a}.title{font-size:10pt;letter-spacing:1pt;font-weight:bold;color:#204e74}.subtitle{margin-top:2mm}
 .grid{display:table;width:100%;table-layout:fixed;margin-bottom:5mm}.panel{display:table-cell;vertical-align:top;padding:3.5mm;background:#f3f6fa;border:1pt solid #e0e7ef}.panel+.panel{border-left:5mm solid white}.panel p{margin:0 0 1.5mm}.label{color:#65758a;font-size:8pt}.kv{width:100%;border-collapse:collapse}.kv td{padding:1mm 0;vertical-align:top}.kv td:first-child{width:39%;color:#65758a;padding-right:3mm}
 table.items{border-collapse:collapse;width:100%;table-layout:fixed;margin:3mm 0 5mm;font-size:8.5pt}.items thead{display:table-header-group}.items th{background:#173f62;color:white;font-size:8pt;font-weight:bold;padding:2.5mm 2mm;text-align:left}.items td{padding:2.5mm 2mm;border-bottom:1pt solid #dce4ed;vertical-align:top}.items tbody tr:nth-child(even){background:#f7f9fc}.items tr{break-inside:avoid}.items .num{text-align:right;font-variant-numeric:tabular-nums}.muted{color:#65758a}.totals{width:62%;margin-left:auto;break-inside:avoid;border-collapse:collapse}.totals td{padding:1.8mm 2mm;vertical-align:top}.totals td:last-child{text-align:right;font-variant-numeric:tabular-nums;width:45%}.grand td{background:#eaf0f7;border-top:1.5pt solid #173f62;font-size:11pt;font-weight:bold;padding:3mm 2mm}.note{padding:3mm;margin:4mm 0;background:#fff7e5;border-left:2pt solid #d6a550;font-size:8pt;break-inside:avoid}.bank{margin-top:6mm;padding:4mm;background:#f3f6fa;border:1pt solid #dce4ed;break-inside:avoid}.bank .kv td{padding:.7mm 0}.narration{margin-top:4mm;white-space:pre-wrap}.meta{margin-bottom:4mm;font-size:8pt;color:#65758a}
 </style></head><body><header class="heading"><h1>` + he(company) + `</h1><div class="title">` + he(title) + `</div><div class="subtitle">` + he(subtitle) + `</div></header>` + body + `</body></html>`
}
func bankHTML(banks []BankDetails) string {
	if len(banks) == 0 {
		return ""
	}
	b := banks[0]
	var out strings.Builder
	out.WriteString(`<section class="bank"><h3>Payment bank details</h3><table class="kv">`)
	for _, r := range [][2]string{{"Bank", firstNonBlank(b.BankName, b.Ledger)}, {"Account holder", b.AccountHolder}, {"Account number", b.AccountNumber}, {"IFSC", b.IFSC}, {"Branch", b.Branch}, {"UPI", b.UPI}} {
		if r[1] != "" {
			fmt.Fprintf(&out, "<tr><td>%s</td><td>%s</td></tr>", he(r[0]), he(r[1]))
		}
	}
	out.WriteString(`</table></section>`)
	return out.String()
}
func invoiceHTML(x VoucherDetail, bank ...BankDetails) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="meta">%s</div><div class="grid"><section class="panel"><h3>Bill to</h3><p><b>%s</b></p>`, he(pdfDataNote(x.CommonData)), he(firstNonBlank(x.CustomerName, "Customer not returned")))
	for _, a := range x.CustomerAddress {
		if a != "" {
			fmt.Fprintf(&b, "<p>%s</p>", he(a))
		}
	}
	if x.CustomerCity != "" {
		fmt.Fprintf(&b, "<p>%s</p>", he(x.CustomerCity))
	}
	if x.GSTIN != "" {
		fmt.Fprintf(&b, `<p><span class="label">GSTIN</span> %s</p>`, he(x.GSTIN))
	}
	b.WriteString(`</section><section class="panel"><h3>Invoice details</h3><table class="kv">`)
	for _, r := range [][2]string{{"Invoice number", dash(x.InvoiceNumber)}, {"Invoice date", displayDate(x.InvoiceDate)}, {"Voucher type", x.VoucherType}} {
		if r[1] != "" {
			fmt.Fprintf(&b, "<tr><td>%s</td><td><b>%s</b></td></tr>", he(r[0]), he(r[1]))
		}
	}
	b.WriteString(`</table></section></div>`)
	dispatch := [][2]string{{"Dispatched through", x.DispatchedThrough}, {"LR / RR number", x.LRNumber}, {"LR / RR date", displayDate(x.LRDate)}, {"Destination", x.Destination}, {"Vehicle number", x.VehicleNumber}, {"e-Way bill", x.EWayBillNumber}}
	has := false
	for _, r := range dispatch {
		if r[1] != "" && r[1] != "-" {
			has = true
		}
	}
	if has {
		b.WriteString(`<h3>Dispatch details</h3><table class="kv">`)
		for _, r := range dispatch {
			if r[1] != "" && r[1] != "-" {
				fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td></tr>", he(r[0]), he(r[1]))
			}
		}
		b.WriteString(`</table>`)
	}
	b.WriteString(`<table class="items"><colgroup><col style="width:5%"><col style="width:33%"><col style="width:11%"><col style="width:14%"><col style="width:16%"><col style="width:21%"></colgroup><thead><tr><th>#</th><th>Product</th><th>HSN</th><th class="num">Quantity</th><th class="num">Rate</th><th class="num">Amount (INR)</th></tr></thead><tbody>`)
	for i, it := range x.Items {
		fmt.Fprintf(&b, `<tr><td>%d</td><td>%s</td><td>%s</td><td class="num">%s</td><td class="num">%s</td><td class="num">%s</td></tr>`, i+1, he(it.Name), he(it.HSN), he(it.Quantity), he(it.Rate), he(indianNumber(it.Amount)))
	}
	if len(x.Items) == 0 {
		b.WriteString(`<tr><td colspan="6">No product rows returned by Tally.</td></tr>`)
	}
	b.WriteString(`</tbody></table><table class="totals">`)
	for _, r := range []struct {
		label string
		value float64
	}{{"Discount", x.DiscountAmount}, {"Taxable amount", x.TaxableAmount}, {"CGST", x.CGST}, {"SGST", x.SGST}, {"IGST", x.IGST}, {"Freight / charges", x.Freight}, {"Round off", x.RoundOff}} {
		if r.value != 0 {
			fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td></tr>", he(r.label), he(inr(r.value)))
		}
	}
	fmt.Fprintf(&b, `<tr class="grand"><td>Total amount</td><td>%s</td></tr></table>`, he(inr(x.TotalAmount)))
	b.WriteString(bankHTML(bank))
	if x.Narration != "" {
		fmt.Fprintf(&b, `<div class="narration"><h3>Narration</h3>%s</div>`, he(x.Narration))
	}
	return pdfDocument(firstNonBlank(x.FirmName, x.Company), "INVOICE DETAIL", "Read-only copy of data returned by TallyPrime", b.String())
}
func invoicePDF(x VoucherDetail, bank ...BankDetails) ([]byte, error) {
	return renderPDF(invoiceHTML(x, bank...))
}

type pdfTableDoc struct {
	Title, Company, Subtitle string
	Headers                  []string
	Widths                   []float64
	Rows                     [][]string
	TotalLabel, Total        string
	Note                     string
	Numeric                  []bool
}

func tableHTML(d pdfTableDoc, bank ...BankDetails) string {
	var b strings.Builder
	if d.Note != "" {
		fmt.Fprintf(&b, `<div class="note">%s</div>`, he(d.Note))
	}
	b.WriteString(`<table class="items"><colgroup>`)
	total := 0.0
	for _, w := range d.Widths {
		total += w
	}
	for _, w := range d.Widths {
		fmt.Fprintf(&b, `<col style="width:%.2f%%">`, 100*w/total)
	}
	b.WriteString(`</colgroup><thead><tr>`)
	for i, h := range d.Headers {
		cl := ""
		if i < len(d.Numeric) && d.Numeric[i] {
			cl = ` class="num"`
		}
		fmt.Fprintf(&b, "<th%s>%s</th>", cl, he(h))
	}
	b.WriteString(`</tr></thead><tbody>`)
	for _, row := range d.Rows {
		b.WriteString(`<tr>`)
		for i := range d.Headers {
			v := ""
			if i < len(row) {
				v = row[i]
			}
			cl := ""
			if i < len(d.Numeric) && d.Numeric[i] {
				cl = ` class="num"`
			}
			fmt.Fprintf(&b, "<td%s>%s</td>", cl, he(v))
		}
		b.WriteString(`</tr>`)
	}
	if len(d.Rows) == 0 {
		fmt.Fprintf(&b, `<tr><td colspan="%d">No recognized detail rows available.</td></tr>`, len(d.Headers))
	}
	b.WriteString(`</tbody></table>`)
	if d.Total != "" {
		fmt.Fprintf(&b, `<table class="totals"><tr class="grand"><td>%s</td><td>%s</td></tr></table>`, he(d.TotalLabel), he(d.Total))
	}
	b.WriteString(bankHTML(bank))
	return pdfDocument(d.Company, d.Title, d.Subtitle, b.String())
}
func statementDoc(x LedgerStatementData) pdfTableDoc {
	rows := [][]string{}
	if x.OpeningBalance != nil {
		rows = append(rows, []string{"", "Opening balance", "", "", "", balanceText(*x.OpeningBalance)})
	}
	for _, v := range x.Vouchers {
		dr, cr, bal := "Not returned", "Not returned", "Unavailable"
		if v.Debit != nil {
			dr = indianNumber(*v.Debit)
			cr = indianNumber(*v.Credit)
		}
		if v.Balance != nil {
			bal = balanceText(*v.Balance)
		}
		rows = append(rows, []string{displayDate(v.Date), v.Number, v.VoucherType, dr, cr, bal})
	}
	rows = append(rows, []string{"", "Period totals", "", indianNumber(x.DebitTotal), indianNumber(x.CreditTotal), ""})
	closing := "Unavailable"
	if x.ClosingBalance != nil {
		closing = balanceText(*x.ClosingBalance)
	}
	return pdfTableDoc{Title: "LEDGER STATEMENT", Company: x.Company, Subtitle: x.Ledger + " | " + displayDate(x.FromDate) + " to " + displayDate(x.ToDate) + " | " + pdfDataNote(x.CommonData), Headers: []string{"Date", "Voucher", "Type", "Debit (INR)", "Credit (INR)", "Balance"}, Widths: []float64{70, 90, 70, 80, 80, 121}, Rows: rows, TotalLabel: "Closing balance", Total: closing, Note: x.BalanceNote, Numeric: []bool{false, false, false, true, true, true}}
}
func ledgerStatementPDF(x LedgerStatementData, bank ...BankDetails) ([]byte, error) {
	return renderPDF(tableHTML(statementDoc(x), bank...))
}
func outstandingDoc(x LedgerOutstandingData) pdfTableDoc {
	rows := [][]string{}
	for _, v := range x.Bills {
		rows = append(rows, []string{v.BillNumber, displayDate(v.Date), displayDate(v.DueDate), fmt.Sprint(v.Ageing), indianNumber(v.PendingAmount) + " " + v.DrCr})
	}
	note := ""
	if x.SummaryOnly {
		note = "Summary only: bill-wise breakup is unavailable. Total is the saved party ledger balance, not a newly verified bill-wise total."
	}
	return pdfTableDoc{Title: "OUTSTANDING BALANCE", Company: x.Company, Subtitle: x.Ledger + " | " + pdfDataNote(x.CommonData), Headers: []string{"Bill number", "Bill date", "Due date", "Days", "Pending (INR)"}, Widths: []float64{130, 90, 90, 50, 151}, Rows: rows, TotalLabel: "Net outstanding", Total: inr(x.Total) + " " + x.DrCr, Note: note, Numeric: []bool{false, false, false, true, true}}
}
func ledgerOutstandingPDF(x LedgerOutstandingData, bank ...BankDetails) ([]byte, error) {
	return renderPDF(tableHTML(outstandingDoc(x), bank...))
}
func groupDoc(x GroupBalanceData) pdfTableDoc {
	rows := [][]string{}
	for _, v := range x.Ledgers {
		rows = append(rows, []string{v.Ledger, v.Parent, indianNumber(v.Balance), v.DrCr})
	}
	return pdfTableDoc{Title: "LEDGER GROUP DETAILS", Company: x.Company, Subtitle: x.Group + " | " + pdfDataNote(x.CommonData), Headers: []string{"Ledger", "Parent group", "Balance (INR)", "Dr/Cr"}, Widths: []float64{190, 140, 130, 51}, Rows: rows, TotalLabel: "Net balance", Total: inr(x.Total), Numeric: []bool{false, false, true, false}}
}
func groupBalancesPDF(x GroupBalanceData) ([]byte, error) { return renderPDF(tableHTML(groupDoc(x))) }
func customerDoc(x Customer360Data) pdfTableDoc {
	rows := [][]string{}
	for _, v := range x.LatestInvoices {
		rows = append(rows, []string{displayDate(v.Date), v.Number, v.VoucherType, inr(v.Amount)})
	}
	return pdfTableDoc{Title: "CUSTOMER 360", Company: x.Company, Subtitle: x.Ledger + " | " + pdfDataNote(x.CommonData), Headers: []string{"Date", "Invoice", "Type", "Amount"}, Widths: []float64{100, 150, 130, 131}, Rows: rows, TotalLabel: "Outstanding", Total: inr(x.Outstanding), Note: fmt.Sprintf("Known overdue %s | %d known pending bills. Bill coverage may be incomplete.", inr(x.Overdue), x.PendingBillCount), Numeric: []bool{false, false, false, true}}
}
func customer360PDF(x Customer360Data, bank ...BankDetails) ([]byte, error) {
	return renderPDF(tableHTML(customerDoc(x), bank...))
}
func trimPDF(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n-3]) + "..."
	}
	return s
}

func pdfBrowsers() []string {
	var paths []string
	seen := map[string]bool{}
	add := func(p string) {
		if p != "" && !seen[strings.ToLower(p)] {
			seen[strings.ToLower(p)] = true
			paths = append(paths, p)
		}
	}
	add(os.Getenv("TALLYBRIDGE_PDF_BROWSER"))
	// Prefer Chrome, but try Edge automatically if Chrome cannot print.
	for _, relative := range [][]string{{"Google", "Chrome", "Application", "chrome.exe"}, {"Microsoft", "Edge", "Application", "msedge.exe"}} {
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA"} {
			if base := os.Getenv(env); base != "" {
				p := filepath.Join(append([]string{base}, relative...)...)
				if info, e := os.Stat(p); e == nil && !info.IsDir() {
					add(p)
				}
			}
		}
	}
	for _, name := range []string{"chrome.exe", "msedge.exe", "google-chrome", "microsoft-edge", "chromium", "chromium-browser"} {
		if p, e := exec.LookPath(name); e == nil {
			add(p)
		}
	}
	return paths
}

// A launcher's successful exit does not prove it printed anything. Some Windows
// browser launchers hand off to a child, so watch for the complete file as well.
func waitForPDF(ctx context.Context, output string, exited <-chan error) ([]byte, error) {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		if data, e := os.ReadFile(output); e == nil && completePDF(data) {
			return data, nil
		}
		select {
		case <-ctx.Done():
			return nil, errors.New("no completed PDF before timeout")
		case err := <-exited:
			exited = nil
			if err != nil {
				if data, e := os.ReadFile(output); e == nil && completePDF(data) {
					return data, nil
				}
				return nil, fmt.Errorf("browser exited: %w", err)
			}
		case <-tick.C:
		}
	}
}
func printPDF(parent context.Context, browser, dir, inputURL string, index int) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	output := filepath.Join(dir, fmt.Sprintf("report-%d.pdf", index))
	args := []string{"--headless", "--disable-gpu", "--disable-extensions", "--disable-background-networking", "--no-first-run", "--no-default-browser-check", "--disable-sync", "--no-pdf-header-footer", "--allow-file-access-from-files", "--user-data-dir=" + filepath.Join(dir, fmt.Sprintf("profile-%d", index)), "--print-to-pdf=" + output, "--timeout=10000", inputURL}
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		args = append([]string{"--no-sandbox"}, args...)
	}
	cmd := pdfCommand(ctx, browser, args...)
	cmd.Dir = dir
	if e := cmd.Start(); e != nil {
		return nil, e
	}
	exited := make(chan error, 1)
	done := make(chan struct{})
	go func() { exited <- cmd.Wait(); close(done) }()
	data, err := waitForPDF(ctx, output, exited)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
	return data, err
}
func renderPDF(document string) ([]byte, error) {
	key := sha256.Sum256([]byte(document))
	renderedPDFMu.Lock()
	cached, ok := renderedPDFs[key]
	renderedPDFMu.Unlock()
	if ok {
		return append([]byte(nil), cached...), nil
	}

	select {
	case pdfGate <- struct{}{}:
		defer func() { <-pdfGate }()
	default:
		return nil, errors.New("another PDF is being prepared; please try again shortly")
	}
	browsers := pdfBrowsers()
	if len(browsers) == 0 {
		return nil, errors.New("Microsoft Edge or Google Chrome is required to create PDFs; install or repair one on the Windows PC")
	}
	dir, e := os.MkdirTemp("", "tallybridge-pdf-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(dir)
	input := filepath.Join(dir, "report.html")
	if e = os.WriteFile(input, []byte(document), 0600); e != nil {
		return nil, e
	}
	fp := filepath.ToSlash(input)
	if runtime.GOOS == "windows" {
		fp = "/" + fp
	}
	u := (&url.URL{Scheme: "file", Path: fp}).String()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	var data []byte
	var failures []string
	for i, browser := range browsers {
		data, e = printPDF(ctx, browser, dir, u, i)
		if e == nil {
			break
		}
		failures = append(failures, filepath.Base(browser)+": "+e.Error())
		if ctx.Err() != nil {
			break
		}
	}
	if e != nil {
		return nil, fmt.Errorf("PDF creation failed on the PC (%s). Install/update Google Chrome, then retry. Keep TallyBridge open", strings.Join(failures, "; "))
	}

	renderedPDFMu.Lock()
	if len(renderedPDFs) >= 8 || renderedPDFBytes+len(data) > 24<<20 {
		renderedPDFs = map[[32]byte][]byte{}
		renderedPDFBytes = 0
	}
	if len(data) <= 24<<20 {
		renderedPDFs[key] = append([]byte(nil), data...)
		renderedPDFBytes += len(data)
	}
	renderedPDFMu.Unlock()
	return data, nil
}

func completePDF(data []byte) bool {
	return bytes.HasPrefix(data, []byte("%PDF-")) && bytes.HasSuffix(bytes.TrimSpace(data), []byte("%%EOF"))
}
