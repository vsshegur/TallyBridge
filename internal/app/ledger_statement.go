package app

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

func balanceSide(v float64) string {
	if v < 0 {
		return "Dr"
	}
	if v > 0 {
		return "Cr"
	}
	return ""
}
func balanceText(v float64) string {
	return strings.TrimSpace("INR " + indianNumber(abs(v)) + " " + balanceSide(v))
}
func floatPtr(v float64) *float64 { return &v }
func recognizedCollection(b []byte) bool {
	r, e := parseTree(b)
	if e != nil {
		return false
	}
	var ns []*xnode
	nodesByName(r, "COLLECTION", &ns)
	return len(ns) > 0
}
func hasLedgerBalances(b []byte) bool {
	r, e := parseTree(b)
	if e != nil {
		return false
	}
	var ns []*xnode
	nodesByName(r, "LEDGER", &ns)
	if len(ns) == 0 {
		return false
	}
	for _, n := range ns {
		if firstDeep(n, "NAME") == "" || firstDeep(n, "CLOSINGBALANCE") == "" {
			return false
		}
	}
	return true
}
func ledgerPeriodBalanceXML(c, l, from, to string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Object</TYPE><SUBTYPE>Ledger</SUBTYPE><ID TYPE="Name">%s</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVFROMDATE TYPE="Date">%s</SVFROMDATE><SVTODATE TYPE="Date">%s</SVTODATE><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><FETCHLIST><FETCH>Name</FETCH><FETCH>OpeningBalance</FETCH><FETCH>ClosingBalance</FETCH></FETCHLIST></DESC></BODY></ENVELOPE>`, escXML(l), escXML(c), from, to))
}
func statementRows(b []byte, ledger string) ([]VoucherRow, bool) {
	r, e := parseTree(b)
	if e != nil {
		return nil, false
	}
	var nodes []*xnode
	nodesByName(r, "VOUCHER", &nodes)
	rows := []VoucherRow{}
	complete := true
	for _, n := range nodes {
		v := VoucherRow{MasterID: firstDeep(n, "MASTERID"), Number: firstDeep(n, "VOUCHERNUMBER", "REFERENCE"), Date: normISODate(firstDeep(n, "DATE")), Party: firstDeep(n, "PARTYLEDGERNAME"), VoucherType: firstDeep(n, "VOUCHERTYPENAME")}
		if v.Number == "" && v.MasterID == "" {
			complete = false
			continue
		}
		var entries []*xnode
		nodesByName(n, "ALLLEDGERENTRIES.LIST", &entries)
		if len(entries) == 0 {
			nodesByName(n, "LEDGERENTRIES.LIST", &entries)
		}
		found := false
		net := 0.0
		for _, en := range entries {
			if strings.EqualFold(strings.TrimSpace(firstDeep(en, "LEDGERNAME")), strings.TrimSpace(ledger)) {
				raw := directChild(en, "AMOUNT")
				if raw != "" {
					net += parseAmount(raw)
					found = true
				}
			}
		}
		if found {
			v.Amount = abs(net)
			v.Debit = floatPtr(math.Max(-net, 0))
			v.Credit = floatPtr(math.Max(net, 0))
		} else {
			complete = false
			v.Amount = abs(parseAmount(directChild(n, "AMOUNT")))
		}
		rows = append(rows, v)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Date == rows[j].Date {
			return rows[i].Number < rows[j].Number
		}
		return rows[i].Date < rows[j].Date
	})
	return rows, complete
}
func (s *Server) getLedgerStatement(c, l, isoFrom, isoTo, tallyFrom, tallyTo string) (LedgerStatementData, error) {
	key := l + "|" + isoFrom + "|" + isoTo
	var old LedgerStatementData
	at, hasOld := s.cache.Get("ledger-statement", c, key, &old)
	saved := func(err error) (LedgerStatementData, error) {
		if hasOld {
			old.CommonData = CommonData{Source: "saved", Stale: true, FetchedAt: at, LiveError: err.Error()}
			return old, nil
		}
		return LedgerStatementData{}, err
	}
	b, d, _, e := s.tally.request("Ledger statement", c, ledgerStatementXML(c, l, tallyFrom, tallyTo), 0)
	if e != nil {
		return saved(e)
	}
	rows, complete := statementRows(b, l)
	if len(rows) == 0 && (!recognizedCollection(b) || hasOld && old.Count > 0) {
		return saved(errors.New("empty or unrecognized statement response; previous data kept"))
	}
	x := LedgerStatementData{CommonData: CommonData{Live: true, Source: "live", FetchedAt: time.Now(), DurationMS: d}, Company: c, Ledger: l, FromDate: isoFrom, ToDate: isoTo, Vouchers: rows, Count: len(rows)}
	for _, v := range rows {
		x.Total += v.Amount
		if v.Debit != nil {
			x.DebitTotal += *v.Debit
			x.CreditTotal += *v.Credit
		}
	}
	x.BalanceNote = "Opening/running balances unavailable. Do not use the transaction total as the ledger balance."
	if complete {
		// A second small object read, sequentially; never run after a failed first read.
		bb, _, _, be := s.tally.request("Ledger period balances", c, ledgerPeriodBalanceXML(c, l, tallyFrom, tallyTo), 0)
		if be == nil {
			root, pe := parseTree(bb)
			if pe == nil {
				name := firstDeep(root, "NAME")
				op, cl := firstDeep(root, "OPENINGBALANCE"), firstDeep(root, "CLOSINGBALANCE")
				if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(l)) && op != "" && cl != "" {
					opening, closing := parseAmount(op), parseAmount(cl)
					if math.Abs(opening-x.DebitTotal+x.CreditTotal-closing) < 0.011 {
						x.OpeningBalance = floatPtr(opening)
						x.ClosingBalance = floatPtr(closing)
						balance := opening
						for i := range x.Vouchers {
							balance += *x.Vouchers[i].Credit - *x.Vouchers[i].Debit
							x.Vouchers[i].Balance = floatPtr(balance)
						}
						x.BalanceNote = ""
					} else {
						x.BalanceNote = "Tally's period balances did not reconcile with the returned entries. Opening/running balances are withheld."
					}
				}
			}
		} else {
			x.BalanceNote = "Transactions loaded, but period balances could not be verified: " + be.Error()
		}
	} else {
		x.BalanceNote = "Some selected-ledger allocations were not returned. Debit/credit totals cover only recognized rows; balances are unavailable."
	}
	x.ShareText = ledgerStatementShareText(x)
	// Never downgrade a complete saved statement to an incomplete response.
	if hasOld && old.ClosingBalance != nil && x.ClosingBalance == nil {
		return saved(errors.New(x.BalanceNote))
	}
	_ = s.cache.Put("ledger-statement", c, key, x, x.FetchedAt)
	return x, nil
}

func hasXMLNode(b []byte, name string) bool {
	r, e := parseTree(b)
	if e != nil {
		return false
	}
	var nodes []*xnode
	nodesByName(r, name, &nodes)
	return len(nodes) > 0
}
