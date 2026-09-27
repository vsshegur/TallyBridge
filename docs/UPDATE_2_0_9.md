# TallyBridge 2.0.9 — temporary proforma and accurate sharing

## Install

1. Close TallyBridge from its Windows tray menu and install the new setup over the existing installation. Do not uninstall Windows TallyBridge; company settings and pairing are retained.
2. Install the new Android APK. Preview signing remains in use as requested: register this build's SHA-1 from SIGNING-REPORT.txt in your Google Android OAuth client for package in.tallybridge.app. If Android rejects the signature, uninstall the previous APK, install this one and pair again. Phone-local favourites and preferences are removed by uninstalling; Tally data is not touched.
3. Reopen affected invoices and refresh before sharing. Old generated PDFs and previously sent messages cannot be updated. Old invoice caches are deliberately refreshed to remove incorrect totals and fetch item descriptions.

## Bank details

Desktop > PDF Bank now shows the saved account, holder, IFSC and UPI, and labels the form Edit saved bank details. Update the fields and save to retain verified manual details. Uncheck Edit saved bank details to read the selected ledger from Tally again. Settings are separate for each company.

Saved bank details appear in invoice text/PDF, ledger and group outstanding text/PDF, reminders, and proforma text/PDF. No bank is included if none has been saved for the selected company.

## Existing invoices

The total comes from the recorded customer ledger or an explicit voucher total, never the largest ledger amount. Percentage discounts are not treated as rupee amounts and line discounts are not subtracted twice. Repeated identical stock rows are retained. Additional descriptions returned in BASICUSERDESCRIPTION are printed beneath the item and included in text sharing. The PC refuses an invoice export when it cannot verify a total from Tally.

## Create and share a proforma

1. On Android open Reports > Create proforma invoice. TallyBridge reads the selected company's parties and stock masters.
2. Check the proforma number/date, supplier GSTIN/address and supplier state. Maharashtra (27) is the default for your business; a returned supplier GSTIN sets its state.
3. Choose a Tally party or enter a temporary buyer name, GSTIN, city and address. This does not create a party in Tally.
4. Confirm place of supply/delivery state. The buyer GSTIN initially sets it: Maharashtra 27 yields CGST + SGST for a Maharashtra supplier; another state yields IGST. For a buyer without GSTIN, select the delivery state. These calculations are for standard domestic supplies; place of supply can differ from the buyer's registered state.
5. Add Tally stock items. Enter quantity, unit price BEFORE GST, discount percentage and GST percentage. Explicit, current stock-item GST rates are suggested when Tally returns them. Inherited/custom rates that are not returned remain blank and must be confirmed; no universal tax rate is assumed. Additional descriptions and HSN are editable on the temporary quotation.
6. Add dispatch/LR details and notes. Review totals, then share text or preview/share the PDF. Discounts are applied before GST; line amounts and tax components are rounded to paise.

The document is labelled PROFORMA — NOT A TAX INVOICE. It is not posted to Tally and no proforma history or draft database is created. Drafts exist only in Android process memory; closing the process or Discard draft clears them. PDF sharing requires a temporary phone-cache file, removed on next app start. Files/messages sent to WhatsApp or another app remain in that receiving app. The PC deletes rendering temporary files and does not retain proforma PDF cache entries.

## Checks and limits

Automated tests cover invoice discount/GST corrections, additional-description escaping, zero outstanding/date sorting, both proforma tax splits, rounding, invalid inputs, company-bank isolation, authentication requirements and read-only requests. CI builds Windows/Android, checks Android lint, generates a real proforma PDF, and runs native UI/PDF tests. Live Tally customisations and your actual invoice still need comparison on your installation; a sample original invoice and matching read-only XML export can identify any remaining field differences.
