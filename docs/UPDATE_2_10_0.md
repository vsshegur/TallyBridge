# TallyBridge 2.10.0

InvoiceTotally is now a Windows app of its own — **InvoiceTotally for Windows**
([invoicetotally-desktop](https://github.com/vsshegur/invoicetotally-desktop)). It reads invoices, posts them to
TallyPrime itself and handles bank statements and GSTR-2B, so it no longer needs TallyBridge.

## What changed

- **PC app:** the **InvoiceTotally** page is removed (connection, posting switch, browser approvals). TallyBridge
  is read-only again from end to end: the only requests it sends to Tally are Exports.
- **Phone app:** **Scan bill** is removed — the Home tile, the *Purchase bills* row in More, the Scan bill widget,
  the Bill button on the Quick dock widget and the long-press shortcut. A Scan bill widget already on the home
  screen stops working: remove it (long-press → Remove).
- Bills that were waiting on the phone to be sent are not sent any more. Upload them in InvoiceTotally for Windows.
- The old `invoicetotally.json` settings on the PC are left alone (not used).

Everything else — reports, outstanding, orders, proforma, PDF bank, widgets, Google sign-in for phones — is unchanged.
