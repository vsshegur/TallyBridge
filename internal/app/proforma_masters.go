package app

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

type StockItemMaster struct {
	Name        string `json:"name"`
	Unit        string `json:"unit"`
	HSN         string `json:"hsn"`
	Description string `json:"description"`
	GST         string `json:"gst"`
}
type StockItemsData struct {
	CommonData
	Items []StockItemMaster `json:"items"`
}

func stockItemsXML(company string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Collection</TYPE><ID>TBProformaStock</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><TDL><TDLMESSAGE><COLLECTION NAME="TBProformaStock"><TYPE>StockItem</TYPE><FETCH>Name,BaseUnits,Description,AdditionalName,GSTDetails.*,HSNDetails.*,GSTHSNCode</FETCH></COLLECTION></TDLMESSAGE></TDL></DESC></BODY></ENVELOPE>`, escXML(company)))
}
func companyDetailsXML(company string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Object</TYPE><SUBTYPE>Company</SUBTYPE><ID TYPE="Name">%s</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><FETCHLIST><FETCH>Name</FETCH><FETCH>Address</FETCH><FETCH>StateName</FETCH><FETCH>GSTIN</FETCH><FETCH>GSTRegistrationNumber</FETCH></FETCHLIST></DESC></BODY></ENVELOPE>`, escXML(company), escXML(company)))
}
func parseStockItems(raw []byte, asOf string) []StockItemMaster {
	root, e := parseTree(raw)
	if e != nil {
		return nil
	}
	var nodes []*xnode
	nodesByName(root, "STOCKITEM", &nodes)
	out := []StockItemMaster{}
	for _, n := range nodes {
		x := StockItemMaster{Name: firstDeep(n, "NAME"), Unit: firstDeep(n, "BASEUNITS"), Description: firstDeep(n, "DESCRIPTION", "ADDITIONALNAME"), HSN: firstDeep(n, "GSTHSNCODE", "HSNCODE")}
		if x.Name == "" {
			continue
		}
		var details []*xnode
		nodesByName(n, "GSTDETAILS.LIST", &details)
		var best *xnode
		latest := ""
		for _, d := range details {
			date := normISODate(firstDeep(d, "APPLICABLEFROM"))
			if date <= asOf && (best == nil || date >= latest) {
				best = d
				latest = date
			}
		}
		if best != nil {
			x.HSN = firstNonBlank(firstDeep(best, "HSNCODE", "GSTHSNCODE"), x.HSN)
			kind := strings.ToLower(firstDeep(best, "TAXABILITY"))
			if kind == "exempt" || kind == "nil rated" || kind == "nilrated" {
				x.GST = "0"
			} else {
				var rates []*xnode
				nodesByName(best, "RATEDETAILS.LIST", &rates)
				cg, sg := "", ""
				for _, r := range rates {
					v := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(firstDeep(r, "GSTRATE")), "%"))
					valuation := firstDeep(r, "GSTRATEVALUATIONTYPE")
					if valuation != "" && !strings.EqualFold(valuation, "Based on Value") {
						continue
					}
					if _, e := decimal(v, 100); e != nil {
						continue
					}
					head := strings.ToLower(firstDeep(r, "GSTRATEDUTYHEAD"))
					if v == "" {
						continue
					}
					if head == "integrated tax" {
						x.GST = v
					}
					if head == "central tax" {
						cg = v
					}
					if head == "state tax" || head == "ut tax" {
						sg = v
					}
				}
				if x.GST == "" && cg != "" && sg != "" {
					x.GST = fmt.Sprintf("%g", parseAmount(cg)+parseAmount(sg))
				}
				if x.GST != "" {
					x.GST = fmt.Sprintf("%g", parseAmount(x.GST))
					if _, e := decimal(x.GST, 100); e != nil {
						x.GST = ""
					}
				}
			}
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}
func (s *Server) handleProformaMasters(w http.ResponseWriter, r *http.Request) {
	c := strings.TrimSpace(r.URL.Query().Get("company"))
	if c == "" {
		jsonErr(w, 400, errors.New("select a company"))
		return
	}
	idx, e := s.getLedgers(c, false)
	if e != nil {
		jsonErr(w, 503, errors.New("sync the selected company's ledgers first"))
		return
	}
	var stocks StockItemsData
	at, ok := s.cache.Get("stock-items", c, "", &stocks)
	if !ok || r.URL.Query().Get("refresh") == "1" || time.Since(at) > 5*time.Minute {
		raw, ms, _, err := s.tally.request("Read proforma stock masters", c, stockItemsXML(c), 0)
		if err != nil {
			if !ok {
				jsonErr(w, 503, err)
				return
			}
			stocks.Stale = true
			stocks.Live = false
			stocks.LiveError = err.Error()
		} else {
			stocks = StockItemsData{CommonData: CommonData{Live: true, Source: "live", FetchedAt: time.Now(), DurationMS: ms}, Items: parseStockItems(raw, time.Now().Format("2006-01-02"))}
			_ = s.cache.Put("stock-items", c, "", stocks, stocks.FetchedAt)
		}
	}
	details := map[string]string{}
	_, hasCompany := s.cache.Get("proforma-company", c, "", &details)
	if !hasCompany || r.URL.Query().Get("refresh") == "1" {
		raw, _, _, err := s.tally.request("Read proforma company", c, companyDetailsXML(c), 0)
		if err == nil {
			root, _ := parseTree(raw)
			if root != nil {
				details = map[string]string{"gstin": firstDeep(root, "GSTIN", "GSTREGISTRATIONNUMBER"), "address": strings.Join(allDeep(root, "ADDRESS"), ", "), "state": firstDeep(root, "STATENAME")}
				_ = s.cache.Put("proforma-company", c, "", details, time.Now())
			}
		}
	}
	bank, _ := s.store.PDFBank(c)
	jsonOut(w, map[string]any{"company": c, "parties": idx.Ledgers, "stock": stocks, "supplier": details, "bank": bank})
}
