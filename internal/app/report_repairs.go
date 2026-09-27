package app

import "strings"

// Clean saved group reports too, so an upgrade does not need a live Tally read.
func nonzeroGroup(x GroupBalanceData) GroupBalanceData {
	rows := make([]GroupLedgerRow, 0, len(x.Ledgers))
	total := 0.0
	for _, r := range x.Ledgers {
		if abs(r.Balance) < 0.005 {
			continue
		}
		rows = append(rows, r)
		if r.DrCr == "Dr" {
			total -= r.Balance
		} else {
			total += r.Balance
		}
	}
	x.Ledgers = rows
	x.Count = len(rows)
	x.Total = total
	x.ShareText = groupBalanceShareText(x)
	return x
}

// Some Tally exports omit calculated bank methods when only '*' is requested.
// Request those methods explicitly on the same named ledger, never another ledger.
func bankMethodsXML(company, ledger string) []byte {
	fields := `<FETCH>*</FETCH><FETCH>BankAccountNumber</FETCH><FETCH>BankAccNo</FETCH><FETCH>BankAccountNo</FETCH><FETCH>IFSCCode</FETCH><FETCH>IFSCode</FETCH><FETCH>BankName</FETCH><FETCH>BankAccHolderName</FETCH><FETCH>BranchName</FETCH><FETCH>UPIID</FETCH><FETCH>BankDetails.*</FETCH><FETCH>BankDetails.BankAccountNumber</FETCH><FETCH>BankDetails.IFSCCode</FETCH>`
	return []byte(strings.Replace(string(bankLedgerXML(company, ledger)), "<FETCH>*</FETCH>", fields, 1))
}
