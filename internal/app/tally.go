package app

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrBusy         = errors.New("another Tally read is already running")
	ErrCooldown     = errors.New("Tally is in safety cooldown after a failed/slow read")
	ErrTallyRequest = errors.New("Tally rejected the read request")
)

type TallyClient struct {
	store         *Store
	mu            sync.Mutex
	busy          bool
	busyReport    string
	cooldownUntil time.Time
	lastSuccess   time.Time
	lastError     string
	tallyOnline   bool
	companies     []Company
	logs          []RequestLog
}

func NewTallyClient(s *Store) *TallyClient { return &TallyClient{store: s} }
func (t *TallyClient) State() LiveState {
	t.mu.Lock()
	defer t.mu.Unlock()
	g := "READY"
	if t.busy {
		g = "BUSY"
	} else if time.Now().Before(t.cooldownUntil) {
		g = "COOLDOWN"
	}
	return LiveState{TallyOnline: t.tallyOnline, Companies: append([]Company(nil), t.companies...), GateState: g, BusyReport: t.busyReport, LastSuccess: t.lastSuccess, LastError: t.lastError}
}
func (t *TallyClient) Logs() []RequestLog {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]RequestLog(nil), t.logs...)
}
func (t *TallyClient) Pending() (reason string, retry time.Duration, pending bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.busy {
		return "busy", 750 * time.Millisecond, true
	}
	if time.Now().Before(t.cooldownUntil) {
		return "cooldown", time.Until(t.cooldownUntil), true
	}
	return "", 0, false
}
func validateLocalTallyURL(s string) error {
	u, e := url.Parse(s)
	if e != nil {
		return errors.New("invalid Tally URL")
	}
	h := strings.ToLower(u.Hostname())
	if h != "127.0.0.1" && h != "localhost" && h != "::1" {
		return errors.New("Tally URL must remain localhost only")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("Tally URL must use http/https")
	}
	return nil
}
func validateExportXML(b []byte) error {
	u := strings.ToUpper(string(b))
	if !strings.Contains(u, "<TALLYREQUEST>EXPORT</TALLYREQUEST>") {
		return errors.New("blocked: only TALLYREQUEST Export is allowed")
	}
	for _, x := range []string{"<TALLYREQUEST>IMPORT", `ACTION="CREATE"`, `ACTION="ALTER"`, `ACTION="DELETE"`, "CREATE TARGET", "ALTER TARGET", "DELETE TARGET", "MODIFY OBJECT"} {
		if strings.Contains(u, x) {
			return errors.New("blocked non-read-only Tally XML")
		}
	}
	return nil
}

var numericCharRefRE = regexp.MustCompile(`(?i)&#(?:x[0-9a-f]+|[0-9]+);`)

func xml10AllowedRune(r rune) bool {
	return r == 0x09 || r == 0x0A || r == 0x0D ||
		(r >= 0x20 && r <= 0xD7FF) ||
		(r >= 0xE000 && r <= 0xFFFD) ||
		(r >= 0x10000 && r <= 0x10FFFF)
}

func sanitizeXML(b []byte) ([]byte, int) {
	// Tally/custom TDLs sometimes leak XML 1.0-illegal C0 controls either as
	// literal bytes (for example 0x04) or as numeric character references
	// (for example &#4; / &#x4;). Go's encoding/xml rejects both. Remove only
	// those XML-illegal values; preserve all legal accounting text unchanged.
	out := make([]byte, 0, len(b))
	n := 0
	for _, c := range b {
		if c == 0x09 || c == 0x0A || c == 0x0D || c >= 0x20 {
			out = append(out, c)
		} else {
			n++
		}
	}
	clean := numericCharRefRE.ReplaceAllFunc(out, func(ref []byte) []byte {
		s := string(ref)
		var v int64
		var err error
		if len(s) >= 4 && (s[2] == 'x' || s[2] == 'X') {
			v, err = strconv.ParseInt(s[3:len(s)-1], 16, 32)
		} else {
			v, err = strconv.ParseInt(s[2:len(s)-1], 10, 32)
		}
		if err == nil && !xml10AllowedRune(rune(v)) {
			n++
			return nil
		}
		return ref
	})
	return clean, n
}
func tallyError(b []byte) error {
	s := string(b)
	for _, tag := range []string{"ERRORMSG", "LINEERROR"} {
		re := regexp.MustCompile(`(?is)<` + tag + `[^>]*>(.*?)</` + tag + `>`)
		if m := re.FindStringSubmatch(s); len(m) > 1 && strings.TrimSpace(stripTags(m[1])) != "" {
			return fmt.Errorf("%w: %s", ErrTallyRequest, strings.TrimSpace(stripTags(m[1])))
		}
	}
	if regexp.MustCompile(`(?is)<STATUS>\s*0\s*</STATUS>`).MatchString(s) {
		return fmt.Errorf("%w: Tally returned status 0", ErrTallyRequest)
	}
	return nil
}
func stripTags(s string) string { return regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "") }
func (t *TallyClient) request(report, company string, body []byte, timeout time.Duration) ([]byte, int64, int, error) {
	if err := validateExportXML(body); err != nil {
		return nil, 0, 0, err
	}
	t.mu.Lock()
	if t.busy {
		t.mu.Unlock()
		return nil, 0, 0, ErrBusy
	}
	if time.Now().Before(t.cooldownUntil) {
		t.mu.Unlock()
		return nil, 0, 0, ErrCooldown
	}
	t.busy = true
	t.busyReport = report
	t.mu.Unlock()
	start := time.Now()
	success := false
	san := 0
	var respBytes int
	var finalErr error
	defer func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.busy = false
		t.busyReport = ""
		d := time.Since(start)
		lg := RequestLog{At: time.Now(), Report: report, Company: company, DurationMS: d.Milliseconds(), Success: success, ResponseBytes: respBytes, SanitizedChars: san}
		if finalErr != nil {
			lg.Error = finalErr.Error()
		}
		t.logs = append([]RequestLog{lg}, t.logs...)
		if len(t.logs) > 100 {
			t.logs = t.logs[:100]
		}
	}()
	cfg := t.store.Settings()
	if timeout <= 0 {
		timeout = time.Duration(cfg.TimeoutSec) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TallyURL, bytes.NewReader(body))
	if err != nil {
		finalErr = err
		return nil, 0, 0, err
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Close = true
	tr := &http.Transport{DisableKeepAlives: true, MaxIdleConns: 0}
	client := &http.Client{Transport: tr}
	res, err := client.Do(req)
	if err != nil {
		finalErr = err
		t.markNetworkFailure(err)
		return nil, time.Since(start).Milliseconds(), 0, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	respBytes = len(raw)
	if err != nil {
		finalErr = err
		t.markNetworkFailure(err)
		return nil, time.Since(start).Milliseconds(), 0, err
	}
	clean, n := sanitizeXML(raw)
	san = n
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		err = fmt.Errorf("Tally HTTP %s", res.Status)
		finalErr = err
		t.markNetworkFailure(err)
		return nil, time.Since(start).Milliseconds(), san, err
	}
	if err = tallyError(clean); err != nil {
		finalErr = err
		t.mu.Lock()
		t.tallyOnline = true
		t.lastError = err.Error()
		t.mu.Unlock()
		return nil, time.Since(start).Milliseconds(), san, err
	}
	// XML must at least parse enough to avoid saving malformed response.
	dec := xml.NewDecoder(bytes.NewReader(clean))
	for {
		_, e := dec.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			err = fmt.Errorf("Tally returned malformed XML: %w", e)
			finalErr = err
			t.markNetworkFailure(err)
			return nil, time.Since(start).Milliseconds(), san, err
		}
	}
	success = true
	t.mu.Lock()
	t.tallyOnline = true
	t.lastSuccess = time.Now()
	t.lastError = ""
	t.mu.Unlock()
	return clean, time.Since(start).Milliseconds(), san, nil
}
func (t *TallyClient) markNetworkFailure(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	cfg := t.store.Settings()
	t.tallyOnline = false
	t.lastError = err.Error()
	t.cooldownUntil = time.Now().Add(time.Duration(cfg.CooldownSec) * time.Second)
}

func escXML(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
func companyListXML() []byte {
	return []byte(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Collection</TYPE><ID>TBCompanies</ID></HEADER><BODY><DESC><STATICVARIABLES><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><TDL><TDLMESSAGE><COLLECTION NAME="TBCompanies" ISMODIFY="No"><TYPE>Company</TYPE><NATIVEMETHOD>Name</NATIVEMETHOD><NATIVEMETHOD>GUID</NATIVEMETHOD></COLLECTION></TDLMESSAGE></TDL></DESC></BODY></ENVELOPE>`)
}
func ledgerIndexXML(company string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Collection</TYPE><ID>TBLiveLedgers</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><TDL><TDLMESSAGE><COLLECTION NAME="TBLiveLedgers" ISMODIFY="No"><TYPE>Ledger</TYPE><NATIVEMETHOD>Name</NATIVEMETHOD><NATIVEMETHOD>Parent</NATIVEMETHOD><NATIVEMETHOD>GUID</NATIVEMETHOD></COLLECTION></TDLMESSAGE></TDL></DESC></BODY></ENVELOPE>`, escXML(company)))
}
func partyBalancesXML(company, group, id string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Collection</TYPE><ID>%s</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><TDL><TDLMESSAGE><COLLECTION NAME="%s" ISMODIFY="No"><TYPE>Ledger</TYPE><CHILDOF>%s</CHILDOF><BELONGSTO>Yes</BELONGSTO><NATIVEMETHOD>Name</NATIVEMETHOD><NATIVEMETHOD>Parent</NATIVEMETHOD><NATIVEMETHOD>GUID</NATIVEMETHOD><NATIVEMETHOD>ClosingBalance</NATIVEMETHOD></COLLECTION></TDLMESSAGE></TDL></DESC></BODY></ENVELOPE>`, id, escXML(company), id, group))
}
func voucherListXML(company, vtype, from, to string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Collection</TYPE><ID>TBLiveVouchers</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVFROMDATE TYPE="Date">%s</SVFROMDATE><SVTODATE TYPE="Date">%s</SVTODATE><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><TDL><TDLMESSAGE><COLLECTION NAME="TBLiveVouchers" ISMODIFY="No"><TYPE>Vouchers : VoucherType</TYPE><CHILDOF>%s</CHILDOF><BELONGSTO>Yes</BELONGSTO><NATIVEMETHOD>Date</NATIVEMETHOD><NATIVEMETHOD>VoucherNumber</NATIVEMETHOD><NATIVEMETHOD>PartyLedgerName</NATIVEMETHOD><NATIVEMETHOD>VoucherTypeName</NATIVEMETHOD><NATIVEMETHOD>Amount</NATIVEMETHOD><NATIVEMETHOD>MasterID</NATIVEMETHOD></COLLECTION></TDLMESSAGE></TDL></DESC></BODY></ENVELOPE>`, escXML(company), from, to, escXML(vtype)))
}
func voucherDetailCollectionXML(company, masterID string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Collection</TYPE><ID>TBVoucherDetail</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><TDL><TDLMESSAGE><COLLECTION NAME="TBVoucherDetail" ISMODIFY="No"><TYPE>Voucher</TYPE><FILTER>TBVoucherMasterIDFilter</FILTER><FETCH>MasterID,Date,VoucherNumber,VoucherTypeName,PartyLedgerName,PartyName,Narration,BasicBuyerName,BasicBuyerAddress.*,BasicBuyerCity,PartyGSTIN,ConsigneeGSTIN,BasicShippedBy,BasicShipDocumentNo,BasicShipDocumentDate,BasicFinalDestination,BasicShipVesselNo,BillOfLadingNo,BillOfLadingDate,EWayBillNo,Amount,AllInventoryEntries.*,AllLedgerEntries.*,LedgerEntries.*</FETCH></COLLECTION><SYSTEM TYPE="Formulae" NAME="TBVoucherMasterIDFilter" ISMODIFY="No">$MasterID = %s</SYSTEM></TDLMESSAGE></TDL></DESC></BODY></ENVELOPE>`, escXML(company), masterID))
}
func voucherDetailObjectXML(company, masterID string) []byte {
	// Do not use the fragile Voucher object identifier form (ID:'123'). Some
	// TallyPrime builds reject a valid register MasterID as "Could not find
	// Voucher:ID:'123'". A one-voucher filtered collection returns the same
	// object data without changing or writing anything in Tally.
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Collection</TYPE><ID>TBVoucherDetailFull</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><TDL><TDLMESSAGE><COLLECTION NAME="TBVoucherDetailFull" ISMODIFY="No"><TYPE>Voucher</TYPE><FILTER>TBVoucherFullMasterIDFilter</FILTER><FETCH>*</FETCH><FETCH>AllInventoryEntries.*</FETCH><FETCH>AllLedgerEntries.*</FETCH><FETCH>LedgerEntries.*</FETCH></COLLECTION><SYSTEM TYPE="Formulae" NAME="TBVoucherFullMasterIDFilter" ISMODIFY="No">$MasterID = %s</SYSTEM></TDLMESSAGE></TDL></DESC></BODY></ENVELOPE>`, escXML(company), masterID))
}
func voucherDetailHintXML(company string, hint VoucherRow) []byte {
	date := strings.ReplaceAll(normISODate(hint.Date), "-", "")
	if len(date) != 8 || strings.TrimSpace(hint.Number) == "" {
		return nil
	}
	vtype := strings.TrimSpace(hint.VoucherType)
	if vtype == "" { vtype = "Sales" }
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Collection</TYPE><ID>TBVoucherByIdentity</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVFROMDATE TYPE="Date">%s</SVFROMDATE><SVTODATE TYPE="Date">%s</SVTODATE><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><TDL><TDLMESSAGE><COLLECTION NAME="TBVoucherByIdentity" ISMODIFY="No"><TYPE>Vouchers : VoucherType</TYPE><CHILDOF>%s</CHILDOF><BELONGSTO>Yes</BELONGSTO><FILTER>TBVoucherNumberFilter</FILTER><FETCH>*</FETCH><FETCH>AllInventoryEntries.*</FETCH><FETCH>AllLedgerEntries.*</FETCH><FETCH>LedgerEntries.*</FETCH></COLLECTION><SYSTEM TYPE="Formulae" NAME="TBVoucherNumberFilter" ISMODIFY="No">$VoucherNumber = &quot;%s&quot;</SYSTEM></TDLMESSAGE></TDL></DESC></BODY></ENVELOPE>`, escXML(company), date, date, escXML(vtype), escXML(strings.TrimSpace(hint.Number))))
}

func voucherDayXML(company, date string) []byte {
	d := strings.ReplaceAll(normISODate(date), "-", "")
	if len(d) != 8 { return nil }
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Collection</TYPE><ID>TBVoucherDay</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVFROMDATE TYPE="Date">%s</SVFROMDATE><SVTODATE TYPE="Date">%s</SVTODATE><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><TDL><TDLMESSAGE><COLLECTION NAME="TBVoucherDay" ISMODIFY="No"><TYPE>Voucher</TYPE><FETCH>MasterID,Date,VoucherNumber,VoucherTypeName,PartyLedgerName,PartyName,Narration,BasicBuyerName,BasicBuyerAddress.*,BasicBuyerCity,PartyGSTIN,ConsigneeGSTIN,BasicShippedBy,BasicShipDocumentNo,BasicShipDocumentDate,BasicFinalDestination,BasicShipVesselNo,BillOfLadingNo,BillOfLadingDate,EWayBillNo,Amount,AllInventoryEntries.*,AllLedgerEntries.*,LedgerEntries.*</FETCH></COLLECTION></TDLMESSAGE></TDL></DESC></BODY></ENVELOPE>`, escXML(company), d, d))
}

func ledgerDetailXML(company, ledger string) []byte {
	// Fetch the complete single Ledger plus the modern mailing/GST aggregates.
	// TallyPrime 3+ moved several party fields into LedMailingDetails and
	// LedGSTRegDetails, while older companies may still expose top-level methods.
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Object</TYPE><SUBTYPE>Ledger</SUBTYPE><ID TYPE="Name">%s</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><FETCHLIST><FETCH>*</FETCH><FETCH>City</FETCH><FETCH>Pincode</FETCH><FETCH>LedStateName</FETCH><FETCH>PartyGSTIN</FETCH><FETCH>Address.*</FETCH><FETCH>LedMailingDetails.*</FETCH><FETCH>LedGSTRegDetails.*</FETCH><FETCH>LedMultiAddressList.*</FETCH><FETCH>GSTDetails.*</FETCH><FETCH>BankDetails.*</FETCH></FETCHLIST></DESC></BODY></ENVELOPE>`, escXML(ledger), escXML(company)))
}
func ledgerDetailCollectionXML(company, ledger string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Collection</TYPE><ID>TBLedgerDetailFallback</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><TDL><TDLMESSAGE><COLLECTION NAME="TBLedgerDetailFallback" ISMODIFY="No"><TYPE>Ledger</TYPE><FILTER>TBLedgerNameFilter</FILTER><NATIVEMETHOD>*</NATIVEMETHOD><NATIVEMETHOD>Name</NATIVEMETHOD><NATIVEMETHOD>MailingName</NATIVEMETHOD><NATIVEMETHOD>Address</NATIVEMETHOD><NATIVEMETHOD>City</NATIVEMETHOD><NATIVEMETHOD>Pincode</NATIVEMETHOD><NATIVEMETHOD>LedStateName</NATIVEMETHOD><NATIVEMETHOD>PartyGSTIN</NATIVEMETHOD><FETCH>Address.*,LedMailingDetails.*,LedMultiAddressList.*,LedGSTRegDetails.*,GSTDetails.*</FETCH><METHOD>TBGSTIN:$PartyGSTIN</METHOD><METHOD>TBSTATE:$LedStateName</METHOD><METHOD>TBCITY:$City</METHOD><METHOD>TBPINCODE:$Pincode</METHOD></COLLECTION><SYSTEM TYPE="Formulae" NAME="TBLedgerNameFilter" ISMODIFY="No">$Name = &quot;%s&quot;</SYSTEM></TDLMESSAGE></TDL></DESC></BODY></ENVELOPE>`, escXML(company), escXML(strings.TrimSpace(ledger))))
}

// Fetch the selected bank object completely: bank methods vary across Tally versions.
func bankLedgerXML(company, ledger string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Object</TYPE><SUBTYPE>Ledger</SUBTYPE><ID TYPE="Name">%s</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><FETCHLIST><FETCH>*</FETCH></FETCHLIST></DESC></BODY></ENVELOPE>`, escXML(ledger), escXML(company)))
}
func ledgerStatementXML(company, ledger, from, to string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Collection</TYPE><ID>TBLiveLedgerVouchers</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVFROMDATE TYPE="Date">%s</SVFROMDATE><SVTODATE TYPE="Date">%s</SVTODATE><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><TDL><TDLMESSAGE><COLLECTION NAME="TBLiveLedgerVouchers" ISMODIFY="No"><TYPE>Vouchers : Ledger</TYPE><CHILDOF>%s</CHILDOF><NATIVEMETHOD>Date</NATIVEMETHOD><NATIVEMETHOD>VoucherNumber</NATIVEMETHOD><NATIVEMETHOD>PartyLedgerName</NATIVEMETHOD><NATIVEMETHOD>VoucherTypeName</NATIVEMETHOD><NATIVEMETHOD>Amount</NATIVEMETHOD><FETCH>AllLedgerEntries.LedgerName,AllLedgerEntries.Amount</FETCH><NATIVEMETHOD>MasterID</NATIVEMETHOD></COLLECTION></TDLMESSAGE></TDL></DESC></BODY></ENVELOPE>`, escXML(company), from, to, escXML(ledger)))
}
func groupBalancesXML(company, group string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Collection</TYPE><ID>TBGroupLedgers</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><TDL><TDLMESSAGE><COLLECTION NAME="TBGroupLedgers" ISMODIFY="No"><TYPE>Ledger</TYPE><CHILDOF>%s</CHILDOF><BELONGSTO>Yes</BELONGSTO><NATIVEMETHOD>Name</NATIVEMETHOD><NATIVEMETHOD>Parent</NATIVEMETHOD><NATIVEMETHOD>GUID</NATIVEMETHOD><NATIVEMETHOD>ClosingBalance</NATIVEMETHOD></COLLECTION></TDLMESSAGE></TDL></DESC></BODY></ENVELOPE>`, escXML(company), escXML(group)))
}
func partyOutstandingXML(company, ledger string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Data</TYPE><ID>Ledger Outstandings</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><LEDGERNAME>%s</LEDGERNAME><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT><EXPLODEFLAG>Yes</EXPLODEFLAG></STATICVARIABLES></DESC></BODY></ENVELOPE>`, escXML(company), escXML(ledger)))
}
func genericReportXML(company, name string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Data</TYPE><ID>%s</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES></DESC></BODY></ENVELOPE>`, escXML(name), escXML(company)))
}

// generic XML tree for tolerant parsing across Tally versions/configurations.
type xnode struct {
	Name     string
	Text     string
	Children []*xnode
}

func parseTree(b []byte) (*xnode, error) {
	root := &xnode{Name: "ROOT"}
	stack := []*xnode{root}
	d := xml.NewDecoder(bytes.NewReader(b))
	for {
		tok, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		switch z := tok.(type) {
		case xml.StartElement:
			n := &xnode{Name: strings.ToUpper(z.Name.Local)}
			if n.Name == "LEDGER" || n.Name == "COMPANY" || n.Name == "STOCKITEM" {
				for _, a := range z.Attr {
					if strings.EqualFold(a.Name.Local, "NAME") {
						n.Children = append(n.Children, &xnode{Name: "NAME", Text: a.Value})
					}
				}
			}
			p := stack[len(stack)-1]
			p.Children = append(p.Children, n)
			stack = append(stack, n)
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].Text += string(z)
			}
		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return root, nil
}
func nodesByName(n *xnode, name string, out *[]*xnode) {
	if n.Name == strings.ToUpper(name) {
		*out = append(*out, n)
	}
	for _, c := range n.Children {
		nodesByName(c, name, out)
	}
}
func firstDeep(n *xnode, names ...string) string {
	m := map[string]bool{}
	for _, x := range names {
		m[strings.ToUpper(x)] = true
	}
	var walk func(*xnode) string
	walk = func(x *xnode) string {
		if m[x.Name] {
			if v := strings.TrimSpace(x.Text); v != "" {
				return v
			}
		}
		for _, c := range x.Children {
			if v := walk(c); v != "" {
				return v
			}
		}
		return ""
	}
	return walk(n)
}

func normalizedTagName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// firstDeepTagMatch is a conservative fallback for Tally customisations/UDFs.
// Some companies expose standard Dispatch Details through namespaced/custom
// XML tag names. We match only the tag name and return its text; no sensitive
// diagnostic payload is logged. Exact standard fields are always tried first.
func firstDeepTagMatch(n *xnode, accept func(string) bool) string {
	var walk func(*xnode) string
	walk = func(x *xnode) string {
		name := normalizedTagName(x.Name)
		if accept(name) {
			if v := strings.TrimSpace(x.Text); v != "" {
				return v
			}
		}
		for _, c := range x.Children {
			if v := walk(c); v != "" {
				return v
			}
		}
		return ""
	}
	return walk(n)
}

func directChild(n *xnode, names ...string) string {
	m := map[string]bool{}
	for _, x := range names {
		m[strings.ToUpper(x)] = true
	}
	for _, c := range n.Children {
		if m[c.Name] {
			if v := strings.TrimSpace(c.Text); v != "" {
				return v
			}
		}
	}
	return ""
}
func allDeep(n *xnode, name string) []string {
	var out []string
	var walk func(*xnode)
	want := strings.ToUpper(name)
	walk = func(x *xnode) {
		if x.Name == want {
			if v := strings.TrimSpace(x.Text); v != "" {
				out = append(out, v)
			}
		}
		for _, c := range x.Children {
			walk(c)
		}
	}
	walk(n)
	return out
}
func parseCompanies(b []byte) []Company {
	r, e := parseTree(b)
	if e != nil {
		return nil
	}
	var ns []*xnode
	nodesByName(r, "COMPANY", &ns)
	seen := map[string]bool{}
	var out []Company
	for _, n := range ns {
		name := directChild(n, "NAME")
		if name == "" {
			name = firstDeep(n, "NAME")
		}
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		out = append(out, Company{ID: firstDeep(n, "GUID"), Name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func parseLedgers(b []byte) []LedgerIndexRow {
	r, e := parseTree(b)
	if e != nil {
		return nil
	}
	var ns []*xnode
	nodesByName(r, "LEDGER", &ns)
	var out []LedgerIndexRow
	seen := map[string]bool{}
	for _, n := range ns {
		name := directChild(n, "NAME")
		if name == "" {
			name = firstDeep(n, "NAME")
		}
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		out = append(out, LedgerIndexRow{name, firstDeep(n, "PARENT"), firstDeep(n, "GUID")})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}
func parsePartyBalances(b []byte) []OutstandingParty {
	r, e := parseTree(b)
	if e != nil {
		return nil
	}
	var ns []*xnode
	nodesByName(r, "LEDGER", &ns)
	var out []OutstandingParty
	for _, n := range ns {
		name := firstDeep(n, "NAME")
		if name == "" {
			continue
		}
		amt := parseAmount(firstDeep(n, "CLOSINGBALANCE", "AMOUNT"))
		if amt == 0 {
			continue
		}
		out = append(out, OutstandingParty{Party: name, Amount: abs(amt), DrCr: balanceSide(amt), Parent: firstDeep(n, "PARENT"), GUID: firstDeep(n, "GUID")})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Amount > out[j].Amount })
	return out
}
func parseGroupBalances(b []byte) []GroupLedgerRow {
	r, e := parseTree(b)
	if e != nil {
		return nil
	}
	var ns []*xnode
	nodesByName(r, "LEDGER", &ns)
	var out []GroupLedgerRow
	seen := map[string]bool{}
	for _, n := range ns {
		name := firstDeep(n, "NAME")
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		bal := parseAmount(firstDeep(n, "CLOSINGBALANCE", "AMOUNT"))
		if abs(bal) < 0.005 {
			continue
		}
		drcr := ""
		if bal < 0 {
			drcr = "Dr"
		} else if bal > 0 {
			drcr = "Cr"
		}
		out = append(out, GroupLedgerRow{Ledger: name, Parent: firstDeep(n, "PARENT"), Balance: abs(bal), DrCr: drcr, GUID: firstDeep(n, "GUID")})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Ledger) < strings.ToLower(out[j].Ledger) })
	return out
}

func parseVouchers(b []byte) []VoucherRow {
	r, e := parseTree(b)
	if e != nil {
		return nil
	}
	var ns []*xnode
	nodesByName(r, "VOUCHER", &ns)
	if len(ns) == 0 {
		nodesByName(r, "TALLYMESSAGE", &ns)
	}
	var out []VoucherRow
	seen := map[string]bool{}
	for _, n := range ns {
		num := firstDeep(n, "VOUCHERNUMBER", "REFERENCE")
		mid := firstDeep(n, "MASTERID")
		date := normISODate(firstDeep(n, "DATE", "VOUCHERDATE"))
		if num == "" && mid == "" {
			continue
		}
		k := mid + "|" + num + "|" + date
		if seen[k] {
			continue
		}
		seen[k] = true
		party := firstDeep(n, "PARTYLEDGERNAME", "PARTYNAME", "BASICBUYERNAME")
		amount, _ := invoicePartyTotal(n, party)
		out = append(out, VoucherRow{MasterID: mid, Date: date, Number: num, Party: party, Amount: abs(amount), VoucherType: firstDeep(n, "VOUCHERTYPENAME")})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date == out[j].Date {
			return out[i].Number < out[j].Number
		}
		return out[i].Date < out[j].Date
	})
	return out
}
func parseLedgerDetails(b []byte) map[string]string {
	r, e := parseTree(b)
	if e != nil { return nil }
	var ledgers []*xnode
	nodesByName(r, "LEDGER", &ledgers)
	if len(ledgers) > 1 { return nil }
	return parseLedgerDetailsFor(b, "")
}
func parseLedgerDetailsFor(b []byte, wanted string) map[string]string {
	r, e := parseTree(b)
	if e != nil { return nil }
	var ledgers []*xnode
	nodesByName(r, "LEDGER", &ledgers)
	if len(ledgers) > 0 {
		chosen := (*xnode)(nil)
		for _, candidate := range ledgers {
			name := strings.TrimSpace(firstDeep(candidate, "NAME"))
			if wanted != "" && strings.EqualFold(name, strings.TrimSpace(wanted)) { chosen = candidate; break }
		}
		if chosen == nil && len(ledgers) == 1 { chosen = ledgers[0] }
		if chosen == nil && wanted == "" { chosen = ledgers[0] }
		if chosen == nil { return nil }
		r = chosen
	}
	fields := []string{"NAME", "MAILINGNAME", "PARENT", "PARTYGSTIN", "GSTIN", "GSTREGISTRATIONNUMBER", "INCOMETAXNUMBER", "LEDGERPHONE", "EMAIL", "CITY", "COUNTRYNAME", "STATENAME", "LEDSTATENAME", "PINCODE", "CLOSINGBALANCE", "BANKNAME", "BANKACCOUNTNUMBER", "BANKACCHOLDERNAME", "IFSCCODE", "BRANCHNAME", "UPIID"}
	m := map[string]string{}
	for _, f := range fields {
		if v := firstDeep(r, f); v != "" {
			m[f] = v
		}
	}
	if m["PARTYGSTIN"] == "" {
		m["PARTYGSTIN"] = firstNonBlank(firstDeep(r, "TBGSTIN"), m["GSTIN"], m["GSTREGISTRATIONNUMBER"], firstDeep(r, "GSTREGNUMBER", "GSTREGISTRATIONNO"), firstDeepTagMatch(r, func(tag string) bool { return tag == "GSTIN" || strings.HasSuffix(tag, "GSTIN") }))
	}
	if m["STATENAME"] == "" {
		m["STATENAME"] = firstNonBlank(firstDeep(r, "TBSTATE"), m["LEDSTATENAME"], firstDeep(r, "PLACEOFSUPPLY", "STATE"))
	}
	if m["CITY"] == "" {
		m["CITY"] = firstNonBlank(firstDeep(r, "TBCITY"), firstDeep(r, "LEDGERCITY", "MAILINGCITY", "CITYNAME", "DISTRICT", "DISTRICTNAME", "PLACE"))
	}
	if m["PINCODE"] == "" {
		m["PINCODE"] = firstNonBlank(firstDeep(r, "TBPINCODE"), firstDeep(r, "LEDGERPINCODE", "MAILINGPINCODE"))
	}
	for canonical, tags := range map[string][]string{
		"BANKACCOUNTNUMBER": {"BANKACNO", "BANKACCOUNTNO", "BANKACCOUNTNUM", "ACCOUNTNUMBER", "ACCOUNTNO", "ACNO"},
		"IFSCCODE":          {"IFSC", "BANKIFSC", "BANKIFSCCODE"},
		"BANKACCHOLDERNAME": {"ACCOUNTHOLDERNAME", "ACCOUNTHOLDER", "BANKACCHOLDER"},
		"UPIID":             {"UPIADDRESS", "VPA", "VIRTUALPAYMENTADDRESS"},
	} {
		if m[canonical] == "" {
			for _, tag := range tags {
				if value := firstDeep(r, tag); value != "" {
					m[canonical] = value
					break
				}
			}
		}
	}
	// Tally customisations and bank ledgers use slightly different tag names.
	// Keep the response parser tolerant without ever making a second request.
	aliases := map[string]func(string) bool{
		"BANKNAME": func(n string) bool {
			return strings.Contains(n, "BANK") && strings.Contains(n, "NAME") && !strings.Contains(n, "ACCOUNT") && !strings.Contains(n, "HOLDER") && !strings.Contains(n, "BRANCH")
		},
		"BANKACCOUNTNUMBER": func(n string) bool {
			return strings.Contains(n, "ACCOUNT") && (strings.Contains(n, "NUMBER") || strings.Contains(n, "NO"))
		},
		"BANKACCHOLDERNAME": func(n string) bool { return strings.Contains(n, "HOLDER") && strings.Contains(n, "NAME") },
		"IFSCCODE":          func(n string) bool { return strings.Contains(n, "IFSC") },
		"BRANCHNAME":        func(n string) bool { return strings.Contains(n, "BRANCH") },
		"UPIID": func(n string) bool {
			return strings.Contains(n, "UPI") && (strings.Contains(n, "ID") || strings.Contains(n, "VPA"))
		},
	}
	for key, accept := range aliases {
		if m[key] == "" {
			m[key] = firstDeepTagMatch(r, accept)
		}
	}
	if a := allDeep(r, "ADDRESS"); len(a) > 0 {
		clean := make([]string, 0, len(a))
		seen := map[string]bool{}
		for _, line := range a {
			line = strings.TrimSpace(line)
			key := strings.ToLower(line)
			if line != "" && !seen[key] {
				seen[key] = true
				clean = append(clean, line)
			}
		}
		m["ADDRESS"] = strings.Join(clean, ", ")
		// TallyPrime's modern LedMailingDetails schema has Address/State/Country/PIN
		// but no separate City method. Many ledgers therefore store city as the
		// final address line. Use it only when there are at least two address lines.
		if m["CITY"] == "" && len(clean) >= 2 {
			candidate := strings.TrimSpace(clean[len(clean)-1])
			if len(candidate) <= 80 && candidate != m["STATENAME"] && candidate != m["COUNTRYNAME"] && candidate != m["PINCODE"] {
				m["CITY"] = candidate
			}
		}
	}
	return m
}
func parsePartyBills(b []byte, party string) []BillRow {
	root, e := parseTree(b)
	if e != nil {
		return nil
	}
	// Some report exports emit BILLFIXED followed by sibling amount/date fields.
	// Group only adjacent bill fields; never borrow values from another bill or party.
	var normalize func(*xnode)
	normalize = func(n *xnode) {
		var children []*xnode
		for i := 0; i < len(n.Children); i++ {
			c := n.Children[i]
			if c.Name == "BILLFIXED" {
				bill := &xnode{Name: "BILL", Children: append([]*xnode(nil), c.Children...)}
				for i+1 < len(n.Children) {
					next := n.Children[i+1]
					if next.Name != "BILLCL" && next.Name != "BILLOP" && next.Name != "BILLDUE" && next.Name != "BILLOVERDUE" {
						break
					}
					bill.Children = append(bill.Children, next)
					i++
				}
				children = append(children, bill)
			} else {
				normalize(c)
				children = append(children, c)
			}
		}
		n.Children = children
	}
	normalize(root)
	var rows []BillRow
	// Read only the innermost bill containers. Broad ancestor scans can pair unrelated names/amounts.
	var walk func(*xnode) bool
	walk = func(n *xnode) bool {
		childBill := false
		for _, c := range n.Children {
			if walk(c) {
				childBill = true
			}
		}
		if childBill {
			return true
		}
		allowed := n.Name == "BILL" || n.Name == "BILLCLOSING" || n.Name == "OUTSTANDING" || n.Name == "BILLALLOCATIONS.LIST" || n.Name == "DSPACCNAME"
		if !allowed {
			return false
		}
		reportedParty := firstDeep(n, "BILLPARTY", "PARTYLEDGERNAME")
		if reportedParty != "" && !strings.EqualFold(strings.TrimSpace(reportedParty), strings.TrimSpace(party)) {
			return false
		}
		bn := firstDeep(n, "BILLREF", "BILLNAME", "DSPDISPNAME", "VOUCHERNUMBER", "REFERENCE", "NAME")
		raw := ""
		// Closing/pending balance must take precedence over opening/original amounts.
		for _, tag := range []string{"PENDINGAMOUNT", "BILLCL", "CLOSINGBALANCE", "DSPCLAMT", "DSPCLAMOUNT", "AMOUNT"} {
			if raw = firstDeep(n, tag); raw != "" {
				break
			}
		}
		if bn == "" || raw == "" || strings.EqualFold(strings.TrimSpace(bn), strings.TrimSpace(party)) {
			return false
		}
		amt := parseAmount(raw)
		if amt == 0 {
			return false
		}
		date := normISODate(firstDeep(n, "BILLDATE", "DSPBILLDATE", "INVOICEDATE", "VOUCHERDATE", "DATE"))
		due := normISODate(firstDeep(n, "DUEDATE", "DSPDUEDATE", "BILLDUE"))
		age := 0
		if dt := parseISOOrZero(firstNonBlank(due, date)); !dt.IsZero() && time.Now().After(dt) {
			age = int(time.Since(dt).Hours() / 24)
		}
		rows = append(rows, BillRow{Party: party, BillNumber: bn, Date: date, DueDate: due, PendingAmount: abs(amt), SignedAmount: amt, DrCr: balanceSide(amt), Ageing: age, OriginalAmount: abs(parseAmount(firstDeep(n, "ORIGINALAMOUNT", "OPENINGAMOUNT", "DSPORIGINALAMT", "BILLOP")))})
		return true
	}
	walk(root)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Date < rows[j].Date })
	return rows
}

func parseGenericRows(b []byte) []map[string]string {
	r, e := parseTree(b)
	if e != nil {
		return nil
	}
	var out []map[string]string
	var walk func(*xnode)
	walk = func(n *xnode) {
		sc := 0
		for _, c := range n.Children {
			if strings.TrimSpace(c.Text) != "" {
				sc++
			}
		}
		if sc >= 2 {
			m := map[string]string{}
			for _, c := range n.Children {
				if v := strings.TrimSpace(c.Text); v != "" && len(v) < 500 {
					m[c.Name] = v
				}
			}
			if len(m) >= 2 {
				out = append(out, m)
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(r)
	if len(out) > 250 {
		out = out[:250]
	}
	return out
}
func parseVoucherDetail(b []byte, company, masterID string, hint VoucherRow) VoucherDetail {
	r, _ := parseTree(b)
	return parseVoucherDetailNode(r, company, masterID, hint)
}
func parseVoucherDetailNode(r *xnode, company, masterID string, hint VoucherRow) VoucherDetail {
	x := VoucherDetail{Company: company, FirmName: company, MasterID: masterID, Items: []InvoiceItem{}, CustomerAddress: []string{}}
	if r == nil {
		return x
	}
	x.MasterID = firstNonBlank(firstDeep(r, "MASTERID"), masterID)
	x.InvoiceNumber = firstNonBlank(firstDeep(r, "VOUCHERNUMBER", "REFERENCE"), hint.Number)
	if x.InvoiceNumber == "" {
		x.InvoiceNumber = firstDeepTagMatch(r, func(n string) bool {
			return strings.Contains(n, "INVOICENO") || strings.Contains(n, "INVOICENUMBER") || strings.Contains(n, "VOUCHERNO")
		})
	}
	x.InvoiceDate = firstNonBlank(normISODate(firstDeep(r, "DATE", "VOUCHERDATE")), hint.Date)
	if x.InvoiceDate == "" {
		x.InvoiceDate = normISODate(firstDeepTagMatch(r, func(n string) bool {
			return strings.Contains(n, "INVOICEDATE") || strings.Contains(n, "VOUCHERDATE")
		}))
	}
	x.VoucherType = firstDeep(r, "VOUCHERTYPENAME")
	x.CustomerName = firstNonBlank(firstDeep(r, "BASICBUYERNAME", "PARTYLEDGERNAME", "PARTYNAME", "BASICBASEPARTYNAME"), hint.Party)
	x.CustomerCity = firstDeep(r, "BASICBUYERCITY", "CONSIGNEECITY", "CITY")
	x.GSTIN = firstDeep(r, "PARTYGSTIN", "BUYERGSTIN", "CONSIGNEEGSTIN")
	// Tally's standard invoice fields commonly expose Through as BasicShippedBy and LR as BillOfLadingNo.
	x.DispatchedThrough = firstDeep(r, "BASICSHIPPEDBY", "DISPATCHEDTHROUGH", "BASICTRANSPORTERNAME", "TRANSPORTERNAME", "SHIPPEDBY", "DISPATCHTHROUGH", "CARRIERNAME")
	if x.DispatchedThrough == "" {
		x.DispatchedThrough = firstDeepTagMatch(r, func(n string) bool {
			return strings.Contains(n, "SHIPPEDBY") || strings.Contains(n, "DISPATCHEDTHROUGH") || strings.Contains(n, "DISPATCHTHROUGH") || strings.Contains(n, "TRANSPORTERNAME") || strings.Contains(n, "CARRIERNAME")
		})
	}
	x.LRNumber = firstDeep(r, "BILLOFLADINGNO", "BASICSHIPDOCUMENTNO", "LRNO", "LRNUMBER", "LRRRNO", "LRRRNUMBER", "TRANSPORTDOCNO", "TRANSPORTDOCUMENTNO", "BASICSHIPDOCUMENTNUMBER")
	if x.LRNumber == "" {
		x.LRNumber = firstDeepTagMatch(r, func(n string) bool {
			if strings.Contains(n, "DATE") {
				return false
			}
			return strings.Contains(n, "BILLOFLADING") || strings.Contains(n, "LRRR") || strings.Contains(n, "LRNO") || strings.Contains(n, "LRNUMBER") || ((strings.Contains(n, "SHIPDOCUMENT") || strings.Contains(n, "TRANSPORTDOC")) && (strings.Contains(n, "NO") || strings.Contains(n, "NUMBER")))
		})
	}
	x.LRDate = normISODate(firstDeep(r, "BILLOFLADINGDATE", "BASICSHIPDOCUMENTDATE", "LRDATE", "TRANSPORTDOCDATE", "TRANSPORTDOCUMENTDATE"))
	if x.LRDate == "" {
		x.LRDate = normISODate(firstDeepTagMatch(r, func(n string) bool {
			return strings.Contains(n, "DATE") && (strings.Contains(n, "BILLOFLADING") || strings.Contains(n, "LR") || strings.Contains(n, "SHIPDOCUMENT") || strings.Contains(n, "TRANSPORTDOC"))
		}))
	}
	x.Destination = firstDeep(r, "BASICFINALDESTINATION", "DESTINATION", "PLACEOFSUPPLY")
	x.VehicleNumber = firstDeep(r, "BASICSHIPVESSELNO", "VEHICLENO", "VEHICLENUMBER")
	x.EWayBillNumber = firstDeep(r, "EWAYBILLNO", "EWAYBILLNUMBER")
	x.Narration = firstDeep(r, "NARRATION")
	x.CustomerAddress = allDeep(r, "ADDRESS")
	var inv []*xnode
	nodesByName(r, "ALLINVENTORYENTRIES.LIST", &inv)
	nodesByName(r, "INVENTORYENTRIES.LIST", &inv)

	for _, n := range inv {
		name := firstDeep(n, "STOCKITEMNAME", "ITEMNAME", "NAME")
		if name == "" {
			continue
		}
		qty := firstDeep(n, "BILLEDQTY", "ACTUALQTY", "QUANTITY")
		rate := firstDeep(n, "RATE")
		amt := abs(parseAmount(directChild(n, "AMOUNT")))
		if amt == 0 {
			amt = abs(parseAmount(firstDeep(n, "AMOUNT")))
		}
		x.Items = append(x.Items, InvoiceItem{Name: name, Description: strings.Join(allDeep(n, "BASICUSERDESCRIPTION"), "\n"), HSN: firstDeep(n, "HSN", "HSNCODE", "HSNSAC"), Quantity: qty, Rate: rate, Amount: amt, Discount: abs(parseAmount(directChild(n, "DISCOUNTAMOUNT")))})
	}

	applyInvoiceAmounts(&x, r, hint)
	x.ShareText = dispatchShareText(x)
	return x
}
func parseVoucherDayDetail(b []byte, company, masterID string, hint VoucherRow) (VoucherDetail, bool) {
	root, err := parseTree(b)
	if err != nil || root == nil { return VoucherDetail{}, false }
	var vouchers []*xnode
	nodesByName(root, "VOUCHER", &vouchers)
	if len(vouchers) == 0 { return VoucherDetail{}, false }
	wantNum := strings.TrimSpace(hint.Number)
	wantDate := normISODate(hint.Date)
	wantType := strings.TrimSpace(hint.VoucherType)
	wantParty := strings.TrimSpace(hint.Party)
	bestScore := -1
	var best *xnode
	for _, v := range vouchers {
		num := strings.TrimSpace(firstDeep(v, "VOUCHERNUMBER", "REFERENCE"))
		date := normISODate(firstDeep(v, "DATE", "VOUCHERDATE"))
		vMID := strings.TrimSpace(firstDeep(v, "MASTERID"))
		idMatch := masterID != "" && vMID == masterID
		if wantNum != "" && !strings.EqualFold(num, wantNum) && !idMatch { continue }
		if wantDate != "" && date != "" && date != wantDate { continue }
		score := 100
		if idMatch { score += 100 }
		if wantType != "" && strings.EqualFold(strings.TrimSpace(firstDeep(v, "VOUCHERTYPENAME")), wantType) { score += 30 }
		party := strings.TrimSpace(firstDeep(v, "PARTYLEDGERNAME", "PARTYNAME", "BASICBUYERNAME"))
		if wantParty != "" && strings.EqualFold(party, wantParty) { score += 20 }
		if hint.Amount != 0 {
			if total, ok := invoicePartyTotal(v, firstNonBlank(party, wantParty)); ok && abs(abs(total)-abs(hint.Amount)) < 0.02 { score += 10 }
		}
		if score > bestScore { bestScore, best = score, v }
	}
	if best == nil { return VoucherDetail{}, false }
	return parseVoucherDetailNode(best, company, masterID, hint), true
}

func dispatchShareText(x VoucherDetail) string {
	customer := strings.TrimSpace(x.CustomerName)
	if x.CustomerCity != "" {
		customer += ", " + x.CustomerCity
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Dispatch Details\n1. Firm Name - %s\n2. Customer Name - %s\n3. Invoice Number - %s\n4. Invoice Date - %s\n5. Dispatched Through - %s\n6. Bill of Lading / LR-RR No - %s\n\nProduct Details\n", dash(x.FirmName), dash(customer), dash(x.InvoiceNumber), dash(displayDate(x.InvoiceDate)), dash(x.DispatchedThrough), dash(x.LRNumber))
	if len(x.Items) == 0 {
		b.WriteString("No product details available\n")
	} else {
		for i, it := range x.Items {
			fmt.Fprintf(&b, "%d. %s - Qty: %s - Rate: %s\n", i+1, dash(it.Name), dash(it.Quantity), dash(it.Rate))
			if it.Description != "" {
				fmt.Fprintf(&b, "   %s\n", it.Description)
			}
		}
	}
	if x.DiscountAmount != 0 {
		fmt.Fprintf(&b, "\nDiscount Amount - ₹%s", indianNumber(x.DiscountAmount))
	}
	fmt.Fprintf(&b, "\nTotal Amount - ₹%s", indianNumber(x.TotalAmount))
	return b.String()
}
func ledgerStatementShareText(x LedgerStatementData) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Ledger Statement\nCompany - %s\nLedger - %s\nPeriod - %s to %s\n", dash(x.Company), dash(x.Ledger), displayDate(x.FromDate), displayDate(x.ToDate))
	if x.OpeningBalance != nil {
		fmt.Fprintf(&b, "Opening Balance - %s\n", balanceText(*x.OpeningBalance))
	}
	for i, v := range x.Vouchers {
		fmt.Fprintf(&b, "\n%d. %s | %s | %s | ₹%s", i+1, displayDate(v.Date), dash(v.Number), dash(v.VoucherType), indianNumber(v.Amount))
		if v.Debit != nil && v.Credit != nil {
			fmt.Fprintf(&b, " | Debit ₹%s | Credit ₹%s", indianNumber(*v.Debit), indianNumber(*v.Credit))
		}
		if v.Balance != nil {
			fmt.Fprintf(&b, " | Balance %s", balanceText(*v.Balance))
		}
	}
	fmt.Fprintf(&b, "\n\nTotal Debit - ₹%s\nTotal Credit - ₹%s", indianNumber(x.DebitTotal), indianNumber(x.CreditTotal))
	if x.ClosingBalance != nil {
		fmt.Fprintf(&b, "\nClosing Balance - %s", balanceText(*x.ClosingBalance))
	}
	if x.BalanceNote != "" {
		fmt.Fprintf(&b, "\nNote: %s", x.BalanceNote)
	}
	return b.String()
}

func ledgerOutstandingShareText(x LedgerOutstandingData) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*Outstanding Balance*\n%s\nCustomer: %s\n", dash(x.Company), dash(x.Ledger))
	if x.FilterLabel != "" {
		fmt.Fprintf(&b, "View: %s\n", x.FilterLabel)
	}
	if len(x.Bills) == 0 {
		if x.SummaryOnly {
			b.WriteString("\nBill-wise breakup unavailable. Summary from saved party balance.\n")
		} else {
			b.WriteString("\nNo pending bills in this view.\n")
		}
	} else {
		for i, v := range sortedBills(x.Bills) {
			fmt.Fprintf(&b, "\n*%d. %s*\nInvoice date: %s\nDue for: %s\nPending: ₹%s %s\n", i+1, dash(v.BillNumber), displayDate(v.Date), invoiceDaysLabel(v.Date), indianNumber(v.PendingAmount), v.DrCr)
		}
	}
	fmt.Fprintf(&b, "\n*Total Outstanding - ₹%s %s*", indianNumber(x.Total), x.DrCr)
	if !x.FetchedAt.IsZero() {
		fmt.Fprintf(&b, "\nLast updated: %s", x.FetchedAt.Local().Format("02 Jan 2006, 3:04 PM"))
	}
	return b.String()
}
func groupBalanceShareText(x GroupBalanceData) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Ledger Group Details\nCompany - %s\nGroup - %s\n", dash(x.Company), dash(x.Group))
	if len(x.Ledgers) == 0 {
		b.WriteString("\nNo ledgers found in this group.")
	} else {
		b.WriteString("\nLedgers\n")
		for i, v := range x.Ledgers {
			fmt.Fprintf(&b, "%d. %s - ₹%s %s\n", i+1, v.Ledger, indianNumber(v.Balance), v.DrCr)
		}
	}
	fmt.Fprintf(&b, "\nTotal Balance - ₹%s", indianNumber(x.Total))
	return b.String()
}

func bankDetailsFromMap(ledger string, m map[string]string) BankDetails {
	return BankDetails{Ledger: ledger, BankName: firstNonBlank(m["BANKNAME"], ledger), AccountHolder: m["BANKACCHOLDERNAME"], AccountNumber: m["BANKACCOUNTNUMBER"], IFSC: m["IFSCCODE"], Branch: m["BRANCHNAME"], UPI: m["UPIID"]}
}

func customer360ShareText(x Customer360Data) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Customer 360\nCompany - %s\nCustomer - %s\nOutstanding - ₹%s\nOverdue - ₹%s\nPending Bills - %d\n", dash(x.Company), dash(x.Ledger), indianNumber(x.Outstanding), indianNumber(x.Overdue), x.PendingBillCount)
	if x.LastPayment != nil {
		fmt.Fprintf(&b, "Last Payment - %s | %s | ₹%s\n", displayDate(x.LastPayment.Date), dash(x.LastPayment.Number), indianNumber(x.LastPayment.Amount))
	}
	if len(x.LatestInvoices) > 0 {
		b.WriteString("\nLatest Invoices\n")
		for i, v := range x.LatestInvoices {
			fmt.Fprintf(&b, "%d. %s | %s | ₹%s\n", i+1, displayDate(v.Date), dash(v.Number), indianNumber(v.Amount))
		}
	}
	return strings.TrimSpace(b.String())
}

func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return strings.TrimSpace(s)
}
func firstNonBlank(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return strings.TrimSpace(x)
		}
	}
	return ""
}
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
func parseAmount(s string) float64 {
	s = strings.TrimSpace(s)
	neg := strings.HasSuffix(strings.ToLower(s), " dr") || strings.HasPrefix(s, "-")
	s = strings.ReplaceAll(s, ",", "")
	re := regexp.MustCompile(`[-+]?[0-9]*\.?[0-9]+`)
	m := re.FindString(s)
	if m == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(m, 64)
	if neg && v > 0 {
		v = -v
	}
	return v
}
func normISODate(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\u00a0", " "))
	for _, layout := range []string{"2006-01-02", "20060102", "2-Jan-2006", "2-Jan-06", "2-Jan-2006", "2-January-2006", "2-January-06", "2 Jan 2006", "2 Jan 06", "2 January 2006", "2/1/2006", "2/1/06", "2-1-2006", "2-1-06", "2.1.2006", "2006/1/2", "02012006", time.RFC3339, "2006-01-02 15:04:05"} {
		if t, e := time.Parse(layout, s); e == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

func displayDate(s string) string {
	if t, e := time.Parse("2006-01-02", s); e == nil {
		return t.Format("02-01-2006")
	}
	return s
}
func indianNumber(v float64) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	raw := fmt.Sprintf("%.2f", v)
	parts := strings.SplitN(raw, ".", 2)
	whole := parts[0]
	if len(whole) > 3 {
		last := whole[len(whole)-3:]
		rest := whole[:len(whole)-3]
		groups := []string{}
		for len(rest) > 2 {
			groups = append([]string{rest[len(rest)-2:]}, groups...)
			rest = rest[:len(rest)-2]
		}
		if rest != "" {
			groups = append([]string{rest}, groups...)
		}
		whole = strings.Join(groups, ",") + "," + last
	}
	return sign + whole + "." + parts[1]
}
func financialYearRange(now time.Time) (string, string) {
	y := now.Year()
	if now.Month() < 4 {
		y--
	}
	return fmt.Sprintf("%04d0401", y), fmt.Sprintf("%04d0331", y+1)
}
func monthRange(now time.Time) (string, string) {
	f := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	t := f.AddDate(0, 1, -1)
	return f.Format("20060102"), t.Format("20060102")
}
