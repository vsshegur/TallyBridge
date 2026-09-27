# TallyBridge Windows 2.0.7

Keep Android 2.0.6. No APK, Google SHA-1, or phone pairing change is needed.

Close TallyBridge from the Windows tray, run TallyBridge_Setup.exe over the existing installation, and reopen TallyBridge with the company open in Tally. Refresh the outstanding report on the phone before sharing; an already generated PDF or previously shared message is not changed by an update.

- Group outstanding excludes ledger balances that round to 0.00, including saved group reports and PDFs. Nonzero credit balances remain visible.
- Invoice dates such as 1-Apr-26 and 31-Dec-25 are parsed into real calendar dates. Invalid dates are not fabricated. Dates in older saved bill reports are normalised too.
- Outstanding text, PDF and phone views use oldest invoice first. Due for uses days since invoice date, not the payment due date.
- Saving a bank now tries explicitly named bank-account and IFSC methods when the full ledger export omits them. Leading account zeroes are retained. Responses for another ledger or conflicting accounts are not combined. Manual bank entry remains available.

After updating, open Desktop setup > PDF Bank, select the bank ledger, and click Read from Tally & save with manual entry unchecked. Verify the displayed account and IFSC. If Tally still omits them, retain the verified manual details; this build cannot validate every custom Tally export without a sample from that installation.

Regression tests cover short-year invoice XML, chronological sharing/PDF rows, invoice age, live and saved zero balances, and a bank export requiring the explicit-method fallback. CI also generates real PDFs on Windows.
