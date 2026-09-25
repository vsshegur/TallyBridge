package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Bank preferences are separate from connection settings so an upgrade or a
// settings save cannot accidentally remove a company's PDF bank selection.
func (s *Store) pdfBanksLocked() map[string]BankDetails {
	banks := map[string]BankDetails{}
	b, e := os.ReadFile(filepath.Join(s.dir, "pdf-banks.json"))
	if e == nil {
		_ = json.Unmarshal(b, &banks)
	}
	if banks == nil {
		banks = map[string]BankDetails{}
	}
	return banks
}
func companyKey(c string) string { return strings.ToLower(strings.TrimSpace(c)) }
func (s *Store) PDFBank(c string) (BankDetails, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.pdfBanksLocked()[companyKey(c)]
	return b, ok && b.Ledger != ""
}
func (s *Store) PDFBankNames() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]string{}
	for c, b := range s.pdfBanksLocked() {
		out[c] = b.Ledger
	}
	return out
}
func (s *Store) SavePDFBank(c string, b BankDetails) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	banks := s.pdfBanksLocked()
	if b.Ledger == "" {
		delete(banks, companyKey(c))
	} else {
		banks[companyKey(c)] = b
	}
	data, e := json.MarshalIndent(banks, "", "  ")
	if e != nil {
		return e
	}
	return atomicWrite(filepath.Join(s.dir, "pdf-banks.json"), data, 0600)
}
func (s *Server) handlePDFBank(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		c := strings.TrimSpace(r.URL.Query().Get("company"))
		if c == "" {
			jsonErr(w, 400, errors.New("select a company"))
			return
		}
		idx, e := s.getLedgers(c, false)
		if e != nil {
			jsonErr(w, 404, errors.New("ledger index has not synced yet; wait for automatic sync"))
			return
		}
		banks := []string{}
		for _, l := range idx.Ledgers {
			if l.Name != "" {
				banks = append(banks, l.Name)
			}
		}
		selected, _ := s.store.PDFBank(c)
		jsonOut(w, map[string]any{"company": c, "banks": banks, "selected": selected})
		return
	}
	if r.Method != http.MethodPost {
		jsonErr(w, 405, errors.New("GET or POST required"))
		return
	}
	var q struct {
		Company string `json:"company"`
		Ledger  string `json:"ledger"`
	}
	if readJSON(r, &q) != nil || strings.TrimSpace(q.Company) == "" {
		jsonErr(w, 400, errors.New("company required"))
		return
	}
	var bank BankDetails
	if q.Ledger != "" {
		idx, e := s.getLedgers(q.Company, false)
		valid := false
		if e == nil {
			for _, l := range idx.Ledgers {
				if l.Name == q.Ledger {
					valid = true
					break
				}
			}
		}
		if !valid {
			jsonErr(w, 400, errors.New("choose a bank ledger from this company's saved index"))
			return
		}
		// Explicit desktop save/refresh reads the selected bank once. Sharing PDFs never does.
		b, _, _, e := s.tally.request("Save PDF bank", q.Company, ledgerDetailXML(q.Company, q.Ledger), 0)
		if e != nil {
			jsonErr(w, 503, e)
			return
		}
		details := parseLedgerDetails(b)
		if details["NAME"] != "" && !strings.EqualFold(details["NAME"], q.Ledger) {
			jsonErr(w, 503, errors.New("Tally returned a different bank ledger; previous default kept"))
			return
		}
		bank = bankDetailsFromMap(q.Ledger, details)
		if bank.AccountNumber == "" && bank.UPI == "" {
			jsonErr(w, 422, errors.New("this ledger returned no account number or UPI; previous PDF bank kept"))
			return
		}
	}
	if e := s.store.SavePDFBank(q.Company, bank); e != nil {
		jsonErr(w, 500, e)
		return
	}
	jsonOut(w, map[string]any{"ok": true, "selected": bank})
}
