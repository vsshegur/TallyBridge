package app

import (
	"strconv"
	"strings"
)

// Master rows sent to InvoiceTotally after a master export (groups, units,
// ledgers, stock items). Same shape the InvoiceTotally Bridge sends.
type itMaster struct {
	Name    string   `json:"name"`
	Parent  string   `json:"parent"`
	GSTIN   string   `json:"gstin,omitempty"`
	State   string   `json:"state,omitempty"`
	HSN     string   `json:"hsn,omitempty"`
	GSTRate *float64 `json:"gst_rate,omitempty"`
	Unit    string   `json:"unit,omitempty"`
}

var itExportTags = map[string]string{"export_groups": "GROUP", "export_ledgers": "LEDGER", "export_stock": "STOCKITEM", "export_units": "UNIT"}

func xName(n *pnode) string {
	if v := strings.TrimSpace(n.Attr["NAME"]); v != "" {
		return v
	}
	if l := n.kid("NAME.LIST"); l != nil {
		if k := l.kid("NAME"); k != nil && strings.TrimSpace(k.Text) != "" {
			return strings.TrimSpace(k.Text)
		}
	}
	if k := n.kid("NAME"); k != nil {
		return strings.TrimSpace(k.Text)
	}
	return ""
}

// xFirst returns the first non-empty text of any of tags at or below n, in tag order.
func xFirst(n *pnode, tags ...string) string {
	for _, t := range tags {
		out := ""
		n.walk(func(x *pnode) bool {
			if out != "" {
				return false
			}
			if x.Name == t && strings.TrimSpace(x.Text) != "" {
				out = strings.TrimSpace(x.Text)
				return false
			}
			return true
		})
		if out != "" {
			return out
		}
	}
	return ""
}

func xGSTRate(n *pnode) *float64 {
	var best *float64
	var found *float64
	n.walk(func(x *pnode) bool {
		if found != nil {
			return false
		}
		if x.Name != "RATEDETAILS.LIST" {
			return true
		}
		head := ""
		rate := ""
		if h := x.kid("GSTRATEDUTYHEAD"); h != nil {
			head = strings.ToLower(strings.TrimSpace(h.Text))
		}
		if r := x.kid("GSTRATE"); r != nil {
			rate = strings.TrimSpace(r.Text)
		}
		v, err := strconv.ParseFloat(rate, 64)
		if err != nil {
			return false
		}
		if strings.Contains(head, "igst") || strings.Contains(head, "integrated") {
			found = &v
		} else if (strings.Contains(head, "cgst") || strings.Contains(head, "central")) && best == nil {
			d := v * 2
			best = &d
		}
		return false
	})
	if found != nil {
		return found
	}
	return best
}

func parseITMasters(kind string, b []byte) ([]itMaster, error) {
	tag := itExportTags[kind]
	root, err := parseXTree(b, false)
	if err != nil {
		return nil, err
	}
	out := []itMaster{}
	root.walk(func(n *pnode) bool {
		if n.Name != tag {
			return true
		}
		name := xName(n)
		if name == "" {
			return false
		}
		row := itMaster{Name: name, Parent: xFirst(n, "PARENT")}
		switch tag {
		case "LEDGER":
			row.GSTIN = xFirst(n, "PARTYGSTIN", "GSTIN")
			row.State = xFirst(n, "LEDSTATENAME", "STATENAME", "STATE")
		case "STOCKITEM":
			row.HSN = xFirst(n, "HSNCODE", "HSN")
			row.GSTRate = xGSTRate(n)
			row.Unit = xFirst(n, "BASEUNITS")
		}
		out = append(out, row)
		return false
	})
	return out, nil
}
