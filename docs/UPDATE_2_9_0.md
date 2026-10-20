# TallyBridge 2.9.0 — Google sign-in for InvoiceTotally, approved on this PC

## Install

1. Close TallyBridge from its Windows tray menu and install `TallyBridge-2.9.0-Setup.exe` over the existing one. Settings, companies, phone pairing and the InvoiceTotally connection are kept.
2. The phone app is unchanged (2.8.0), so there is nothing to install on the phone.

## InvoiceTotally: Google sign-in instead of the password

InvoiceTotally now signs in the same way the phone app does:

- **Your Google account.** The one set under *Connection* (owner email), with the same Google Web client ID. TallyBridge passes both to InvoiceTotally when it connects, so there is nothing new to type.
- **Each new browser is approved on this PC.** When a browser signs in for the first time, it shows a 6-digit code. Here, **TallyBridge → InvoiceTotally** shows *Browsers waiting for approval* with the same code (the menu item shows a red count). Press **Approve** only if the codes match; **Deny** otherwise.
- **Signed-in browsers** are listed on the same page. **Sign out** ends that browser's access at once; it needs approval again.
- After a week, Google sign-in is asked again — one click on an approved browser, no new approval.
- The old InvoiceTotally password keeps working until the first browser has been approved, so nobody is locked out while setting up. After that only Google works, and the `APP_PASSWORD` secret can be deleted in Cloudflare. Changing the client ID or email under *Connection* signs every InvoiceTotally browser out.

**One-time step in Google Cloud Console:** APIs & Services → Credentials → open the **Web client** used under *Connection* → **Authorized JavaScript origins** → add your InvoiceTotally address (for example `https://invoicetotally.yourname.workers.dev`) → Save. Without it Google shows *origin not allowed* on the sign-in button.

Approving works only on this PC (the control center), never from a phone or another browser.

## Simpler menu

**Customers** and **Search** are gone from the PC menu. They duplicated what the phone app and Tally itself do. The menu is now:

- **Daily work:** Overview, Today, Orders, InvoiceTotally, Follow-ups
- **Business setup:** Companies, Payments, Company Health
- **System:** Diagnostics, Phones, Connection, Sync, Backup & Restore, Updates, Settings, About

Customer and search screens in the phone app are unchanged.

See [2.8.1](UPDATE_2_8_1.md) for the scrolling fix and [2.8.0](UPDATE_2_8_0.md) for the GSTR-2B check.
