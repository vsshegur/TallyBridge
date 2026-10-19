# TallyBridge 2.7.0 — bank statements and an easier phone app

## Install

1. Close TallyBridge from its Windows tray menu and install the new setup over the existing one. Settings, companies, phone pairing and the InvoiceTotally connection are kept.
2. Install the new Android APK the same way as earlier updates. Widgets already on the home screen keep working.

## Bank statements → Tally (with InvoiceTotally)

Upload a bank statement in InvoiceTotally (company → **Bank statements**): a downloaded PDF, a scanned PDF or photos, or the bank's Excel/CSV export. Any bank works, including co-operative banks. InvoiceTotally checks every entry against the running balance.

TallyBridge 2.7 adds what the matching needs from Tally:

- **Pending invoices**: Bills Receivable and Bills Payable for every party. An entry whose narration has an invoice number and the same amount is matched to that invoice.
- **Entries already in the bank ledger**: these are flagged *Already in Tally* and never posted again.

After you review the statement in InvoiceTotally and press **Push**, TallyBridge posts:

- **Receipt** for money in, **Payment** for money out and **Contra** for cash or a transfer to another bank account;
- each one *Agst Ref* to the matched invoices, with any remainder *On Account*.

The rules for posting are the same as for purchase bills:

- **Opt-in.** Only after **Posting to Tally** is turned on (TallyBridge → Purchase bills).
- **New vouchers only.** Only new vouchers are created, and only in that company's own Tally company. Anything else is refused.
- **Once only.** Each entry is logged before it is sent, so it is never posted twice.
- **Phone stays read-only.** Phone access is unchanged.

## Android: easier to find things

The bottom bar is now **Home · Parties · Orders · Reports · More**.

- **Home** has the following:
  - Big buttons for the jobs done every day: **New order**, **Voice order**, **Scan bill**, **Search**, **Proforma** and **Today's work**.
  - What customers owe you, what you owe suppliers, and this month's sales.
  - Your favourite parties.
  - Any purchase bills still waiting to reach the PC.
- **Parties** is for customers and suppliers:
  - Search by name, GSTIN or mobile.
  - Favourites.
  - Who needs to pay, whom you need to pay, and overdue ageing.
  - Every ledger and group.
- **Reports** has the following:
  - Sales, purchase and proforma registers, and today's dispatch.
  - Outstanding and ageing.
  - Ledger statements and the Day Book.
- **More** combines the old *Tools* and *System* tabs: proforma, purchase bills, payments & QR, GSM calculator, widgets, company health, sync, updates, appearance and security.

**Long-press the app icon** for New order, Voice order, Scan bill and Search.

## Widgets

- **Clear names.** Every widget now has its own name and description in the widget picker, instead of seven entries all called "TallyBridge". On Android 12 and newer the picker also shows a preview.
- **Add from the app.** More → **Home-screen widgets** lists every widget. On most phones one tap places the widget on the home screen.
- **Order desk:**
  - A **+** button for a new order.
  - Shows when it was last updated (for example *Synced 10:42 AM*, or *Saved on phone* when the PC can't be reached).
  - Shows *Updating…* while it refreshes.
  - A refresh started from the home screen is now allowed to finish.
  - Follows the app's Light/Dark setting.
- **Quick actions bar:**
  - New order, Voice order, Scan bill, Search, Proforma and GSM in one row.
  - Narrowed to four columns, it keeps the four most used buttons, so they stay easy to tap.
- **New Scan bill widget (1×1).** Opens the camera straight away for a supplier bill.
- **Icons on every phone.** All widgets use drawn icons, so none show an empty box on phones that lack a symbol font.
