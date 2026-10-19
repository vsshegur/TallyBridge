# TallyBridge 2.6.0 — purchase bills with InvoiceTotally

TallyBridge now works with [InvoiceTotally](https://github.com/vsshegur/invoicetotally): photograph a supplier's bill on the phone, InvoiceTotally reads it, you review it on the PC, and TallyBridge posts it to Tally. The same Windows app replaces the separate InvoiceTotally Bridge.

## Install

1. Close TallyBridge from its Windows tray menu and install the new setup over the existing one. Settings, companies and phone pairing are kept.
2. Install the new Android APK the same way as earlier updates.
3. If the **InvoiceTotally Bridge** runs on this PC, keep it until step 4, then quit it (tray icon → Quit) and remove it from Windows startup. Run only one of the two.

## Connect InvoiceTotally (once, on the PC)

1. Open TallyBridge → **Purchase bills**.
2. If the InvoiceTotally Bridge was set up on this PC, press **Use these settings**. Otherwise paste the InvoiceTotally web address and each company's bridge token (InvoiceTotally → company → *Tally bridge*), one per line, and press **Test & save**. Every token is checked before it is saved.
3. Press **Turn on** under **Posting to Tally** when you are ready.

Until posting is on, TallyBridge only *reads* masters (groups, units, ledgers, stock items) for InvoiceTotally. When it is on, it posts only what you push in InvoiceTotally:

- one new Purchase or Sales voucher, or one new ledger, stock item or unit at a time,
- only into that company's own Tally company (a job for a company that is not open waits),
- never an alteration or deletion — anything else is refused and shown in the activity list,
- never twice: each job is written to a local posting log before it is sent, so if InvoiceTotally hands the same job out again (for example after the internet dropped), TallyBridge reports the earlier result instead of posting again — even after a restart.

All phone requests stay read-only exactly as before. Only the Purchase bills connection on the PC can post.

## Send a bill from the phone

1. Android → **Home → Scan purchase bill** (also under **Tools**). Or share photos/PDFs from WhatsApp, Gallery or Files and choose **Purchase bill (TallyBridge)**.
2. Choose the company and **Purchase** or **Sales**.
3. **Take photo** for each page, or **Photos / PDF** (every page of a PDF is added). Tap ✕ to drop a page. Up to 40 pages at once; one upload can hold several bills — InvoiceTotally finds where each one starts.
4. **Send to PC.** The phone keeps the bill until the PC confirms it, then removes its copy. The PC keeps the pages until InvoiceTotally has them, retrying automatically if the internet is down, then removes them.
5. **Sent bills** shows each bill's progress: *Reading → Ready to review → In Tally*, with bill number, party and total. Review and push in InvoiceTotally in the browser on the PC.

If the PC is not reachable, the bill waits on the phone and is sent the next time TallyBridge connects. Bridge tokens never leave the PC; the phone only sees company names.

## Also fixed

- Photo orders larger than about 1.5 MB failed to upload ("request too large").
- Tally replies in Unicode (UTF-16) were garbled; names with ₹ or Indian-language text were lost.
- Saved settings could be emptied by a power cut right after saving.
- The PC control center never loaded **Today, Customers, Search and Follow-ups**, and its menu showed empty groups.
- Backups now include the InvoiceTotally connection (the backup file contains connection keys — keep it private).
