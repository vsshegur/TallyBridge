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
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Collection</TYPE><ID>TBProformaStock</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><TDL><TDLMESSAGE><COLLECTION NAME="TBProformaStock" ISMODIFY="No"><TYPE>StockItem</TYPE><NATIVEMETHOD>Name</NATIVEMETHOD><NATIVEMETHOD>Parent</NATIVEMETHOD><NATIVEMETHOD>BaseUnits</NATIVEMETHOD><NATIVEMETHOD>Description</NATIVEMETHOD><NATIVEMETHOD>AdditionalName</NATIVEMETHOD><NATIVEMETHOD>GSTHSNCode</NATIVEMETHOD><NATIVEMETHOD>HSNCode</NATIVEMETHOD><FETCH>GSTDetails.*,HSNDetails.*</FETCH><METHOD>TBHSN:$GSTItemHSNCodeEx</METHOD></COLLECTION></TDLMESSAGE></TDL></DESC></BODY></ENVELOPE>`, escXML(company)))
}
func companyDetailsXML(company string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Object</TYPE><SUBTYPE>Company</SUBTYPE><ID TYPE="Name">%s</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><FETCHLIST><FETCH>Name</FETCH><FETCH>MailingName</FETCH><FETCH>Address</FETCH><FETCH>City</FETCH><FETCH>StateName</FETCH><FETCH>CountryName</FETCH><FETCH>Pincode</FETCH></FETCHLIST></DESC></BODY></ENVELOPE>`, escXML(company), escXML(company)))
}
func taxUnitsXML(company string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Collection</TYPE><ID>TBProformaTaxUnits</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><TDL><TDLMESSAGE><COLLECTION NAME="TBProformaTaxUnits" ISMODIFY="No"><TYPE>Tax Unit</TYPE><FETCH>Name,GSTRegNumber,StateName,PinCode,AddressName,Address</FETCH></COLLECTION></TDLMESSAGE></TDL></DESC></BODY></ENVELOPE>`, escXML(company)))
}
func companyGSTXML(company string) []byte {
	return []byte(fmt.Sprintf(`<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Data</TYPE><ID>TBCompanyGSTReport</ID></HEADER><BODY><DESC><STATICVARIABLES><SVCURRENTCOMPANY>%s</SVCURRENTCOMPANY><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES><TDL><TDLMESSAGE><REPORT NAME="TBCompanyGSTReport"><FORM>TBCompanyGSTForm</FORM></REPORT><FORM NAME="TBCompanyGSTForm"><PART>TBCompanyGSTPart</PART></FORM><PART NAME="TBCompanyGSTPart"><LINE>TBCompanyGSTLine</LINE></PART><LINE NAME="TBCompanyGSTLine"><FIELDS>TBCompanyGSTIN</FIELDS></LINE><FIELD NAME="TBCompanyGSTIN"><SET>$GSTRegNumber:TaxUnit:($ExciseUnitName:Company:##SVCurrentCompany)</SET><XMLTAG>GSTIN</XMLTAG></FIELD></TDLMESSAGE></TDL></DESC></BODY></ENVELOPE>`, escXML(company)))
}
func hsnFromNode(n *xnode, asOf string) string {
	var rows []*xnode
	nodesByName(n, "HSNDETAILS.LIST", &rows)
	var best *xnode
	bestDate := ""
	for _, row := range rows {
		d := normISODate(firstDeep(row, "APPLICABLEFROM"))
		if d == "" || d <= asOf {
			if best == nil || d >= bestDate {
				best, bestDate = row, d
			}
		}
	}
	if best != nil {
		if v := firstDeep(best, "HSNCODE", "GSTHSNCODE"); v != "" {
			return strings.TrimSpace(v)
		}
	}
	if v := firstDeep(n, "GSTHSNCODE", "HSNCODE"); v != "" {
		return strings.TrimSpace(v)
	}
	return firstDeepTagMatch(n, func(tag string) bool {
		return strings.Contains(tag, "HSNCODE") && !strings.Contains(tag, "MASTER")
	})
}
func supplierDetailsFromXML(raw []byte) map[string]string {
	root, _ := parseTree(raw)
	if root == nil {
		return map[string]string{}
	}
	details := map[string]string{}
	if v := firstNonBlank(firstDeep(root, "GSTREGNUMBER", "GSTREGISTRATIONNUMBER", "GSTIN", "CMPGSTIN"), firstDeepTagMatch(root, func(tag string) bool { return strings.Contains(tag, "GST") && strings.HasSuffix(tag, "IN") })); v != "" {
		details["gstin"] = strings.ToUpper(strings.TrimSpace(v))
	}
	if a := allDeep(root, "ADDRESS"); len(a) > 0 {
		details["address"] = strings.Join(a, ", ")
	}
	for key, tags := range map[string][]string{
		"city": {"CITY"},
		"state": {"STATENAME"},
		"country": {"COUNTRYNAME"},
		"pincode": {"PINCODE"},
	} {
		if v := firstDeep(root, tags...); v != "" {
			details[key] = v
		}
	}
	if len(details["gstin"]) >= 2 {
		details["stateCode"] = details["gstin"][:2]
	}
	return details
}
func mergeSupplierDetails(dst, src map[string]string) {
	for k, v := range src {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if k == "gstin" || dst[k] == "" {
			dst[k] = v
		}
	}
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
		x := StockItemMaster{Name: firstDeep(n, "NAME"), Unit: firstDeep(n, "BASEUNITS"), Description: firstDeep(n, "DESCRIPTION", "ADDITIONALNAME"), HSN: firstNonBlank(firstDeep(n, "TBHSN"), hsnFromNode(n, asOf))}
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
	if !ok || r.URL.Query().Get("refresh") == "1" || time.Since(at) >= 0 {
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
	if !hasCompany || r.URL.Query().Get("refresh") == "1" || hasCompany {
		details = map[string]string{}
		if raw, _, _, err := s.tally.request("Read proforma company", c, companyDetailsXML(c), 0); err == nil {
			mergeSupplierDetails(details, supplierDetailsFromXML(raw))
		}
		// GST registration belongs to Tally's Tax Unit in current releases, not
		// reliably to the Company object. Read the company's Tax Unit(s) and
		// prefer that GSTIN while keeping the Company mailing address.
		if raw, _, _, err := s.tally.request("Read proforma tax unit", c, taxUnitsXML(c), 0); err == nil {
			mergeSupplierDetails(details, supplierDetailsFromXML(raw))
		}
		if details["gstin"] == "" {
			if raw, _, _, err := s.tally.request("Read company GSTIN", c, companyGSTXML(c), 0); err == nil {
				mergeSupplierDetails(details, supplierDetailsFromXML(raw))
			}
		}
		if len(details) > 0 {
			_ = s.cache.Put("proforma-company", c, "", details, time.Now())
		}
	}
	bank, _ := s.store.PDFBank(c)
	jsonOut(w, map[string]any{"company": c, "parties": idx.Ledgers, "stock": stocks, "supplier": details, "bank": bank})
}
