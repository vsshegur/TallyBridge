package app

import (
	"fmt"
	"strings"
)

func bankText(bank BankDetails) string {
	if bank.AccountNumber == "" && bank.UPI == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n*Payment bank details*\n")
	for _, f := range [][2]string{{"Bank", firstNonBlank(bank.BankName, bank.Ledger)}, {"Account holder", bank.AccountHolder}, {"Account number", bank.AccountNumber}, {"IFSC", bank.IFSC}, {"Branch", bank.Branch}, {"UPI", bank.UPI}} {
		if f[1] != "" {
			fmt.Fprintf(&b, "%s: %s\n", f[0], f[1])
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
func (s *Server) companyBankText(c string) string { b, _ := s.store.PDFBank(c); return bankText(b) }
func descriptionHTML(s string) string {
	if s == "" {
		return ""
	}
	return `<div class="muted" style="white-space:pre-wrap;margin-top:1mm">` + he(s) + `</div>`
}
