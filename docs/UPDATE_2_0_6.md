# TallyBridge 2.0.6

## Install

1. Close TallyBridge from its tray menu and run the new Windows setup. No uninstall is required; existing connection, pairing and company bank preferences are retained.
2. Install the Android preview APK. This release continues preview signing as requested. Read SIGNING-REPORT.txt from this build and register its SHA-1 in the Google Android OAuth client for package `in.tallybridge.app`. Keep the existing web OAuth client used by desktop.
3. If Android reports an incompatible signature, uninstall the old preview app first, install this APK, sign in and approve/pair the phone again on desktop. Uninstalling removes phone-local preferences, favourites and pairing; desktop/Tally data is retained.
4. In Windows setup, open PDF Bank, choose the company and bank ledger, then Read from Tally & save. It now requests the full selected ledger and recognises alternate bank field names. If Tally omits the details, enable manual entry, verify the account and IFSC, and save. This does not edit the Tally ledger. An invalid replacement preserves the previous bank.

## Outstanding and sharing

- Bills are ordered oldest invoice first. Due for counts calendar days since the invoice date; future invoices show zero and unknown dates are labelled unavailable.
- Filter by invoice number/amount, invoice date, pending amount or days pending. Select up to 150 bills after narrowing the report.
- Text, selected totals and PDF use the same view. A changed report requires refreshing and selecting again.
- Details use short WhatsApp-friendly invoice blocks. Review details and payment reminders before choosing a sharing app.
- Payment reminders include the saved company bank and are available for a net receivable with bill details.
- PDF sharing opens the native PDF preview, with PDF-only, PDF plus details, and save options.
- The dashboard and More screen link to sync status, last update, errors, manual sync and connection testing. Light, dark and system themes remain available.

## Verification and limits

Backend regression checks cover bank aliases, invalid-save retention, company isolation, oldest-first ordering, selected net balances, date/amount validation and stale selections. CI builds Windows installers and the APK, checks Android lint, generates real browser PDFs on Windows, and runs native Android PDF/layout checks.

Live Tally ledger exports and Google login must also be checked on your PC and phone after installation. A successful automated build cannot guarantee every Tally customisation or device behaves identically.
