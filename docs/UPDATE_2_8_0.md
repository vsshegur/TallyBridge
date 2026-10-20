# TallyBridge 2.8.0 — GSTR-2B check and a tidier app

## Install

1. Close TallyBridge from its Windows tray menu and install the new setup over the existing one. Settings, companies, phone pairing and the InvoiceTotally connection are kept.
2. Install the new Android APK the same way as earlier updates. Widgets already on the home screen keep working.

## GSTR-2B ↔ purchase register (with InvoiceTotally)

In InvoiceTotally, open a company → **GSTR-2B** and upload the month's GSTR-2B from the GST portal (the JSON, the ZIP it comes in, or the Excel). InvoiceTotally asks this PC for the purchase register and compares the two, invoice by invoice, tax head by tax head.

TallyBridge 2.8 adds the one read this needs:

- **Purchases with their GST.** Purchase vouchers, debit notes and credit notes, plus journals and payments that carry a GST ledger, for the return month and a few months around it. Each comes with the supplier's GSTIN, the supplier's invoice number and date (*Reference*), and every ledger amount, so CGST, SGST, IGST and cess can be checked separately.
- **Read-only.** Nothing is written to Tally for this. Cancelled and optional vouchers are left out.

In InvoiceTotally the result is sorted into: matched, needs a look (amount, date, GSTIN or tax head differs), in GSTR-2B but not in Tally, and in Tally but not in GSTR-2B. You can link a voucher by hand, accept a small difference, mark an invoice to follow up with the supplier, copy a WhatsApp message for each supplier, and download the whole check as Excel.

## PC control center

- The **Purchase bills** page is now called **InvoiceTotally**, because it covers purchase bills, bank statements and the GSTR-2B check. The **Posting to Tally** switch is on this page.
- When Tally sends back an empty or unreadable reply, the message now says so plainly — "Tally sent no ledgers — the last saved list is kept" — instead of a technical error, and the figures already saved are kept.

## Android

- **Today's work** shows only figures: pending and dispatched orders, today's sales, what customers owe and what you owe. Each figure opens its screen. With the PC off it still shows the orders saved on the phone, and money shows "—" instead of ₹0.00. The quick-action buttons stay on Home.
- **One header everywhere.** Search, Customer, GSM calculator, Scan bill and Save photo order now have the same header as the main screens, with a back arrow instead of a "Back" button.
- **Drop-downs** (company, inch/cm, state) look like the text fields and show a ▾ arrow, so it is clear they can be changed.
- **Dialogs follow Light and Dark.** Appearance, Share, Delete and the other pop-ups have rounded corners and the app's own colours instead of the phone's plain grey.
- **"Tally unavailable" is shown in amber**, not green.
- Search and Customer say *needs the PC* with a **Try again** button when the PC can't be reached, instead of a technical error.
- Clearer wording: "1 photo • up to 5", "2 pending bills", "1 ledger sent no bill-wise details".

## Checked in both themes

The design check now takes 57 screenshots on an Android emulator, in Light and Dark: Home, Parties, Orders and the order form, Reports, Outstanding, Sync, More, Widgets, Today's work, Search, Customer, GSM calculator, Scan bill, proforma and every widget.
