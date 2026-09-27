# Windows 2.0.8 — invoice amounts

Keep Android 2.0.6. No APK, SHA-1 or pairing change is required.

Close TallyBridge from the Windows tray and install this setup over the existing version. Keep your company open in Tally. Close and reopen the Android app, open the affected invoice, and refresh it before sharing a new PDF/message. Previously generated PDFs and messages remain unchanged.

The invoice parser previously selected the largest ledger amount as the total. That can incorrectly select sales before discount instead of the amount posted to the customer. This version uses the recorded party ledger total (or an explicit voucher-level total) and never substitutes a product amount or a phone-supplied hint for a verified invoice total.

Item amounts are already net of item discounts. Percentage discounts are no longer counted as rupee amounts or deducted again. Identical product rows are retained. Invoice-level discount, GST, freight and round-off are shown separately in the PDF; other adjustments reconcile the breakdown with the recorded invoice total. Tax rates are not recalculated by TallyBridge. The PDF uses “Net items after discount” rather than claiming that all invoice adjustments share one taxable base.

Old invoice caches must be refreshed from Tally once. If a verified total cannot be read, sharing fails with a refresh message rather than exporting the old incorrect amount. Other report caches and settings are retained.

Regression coverage: goods 1000 - discount 200 + GST 144 = total 944; item percentage discounts; repeated identical items; negative round-off; monetary snapshot enrichment; invoice list total; and rejection of legacy invoice PDF caches. Windows CI generates real PDFs using the browser renderer.

The user's live invoice was not available during testing. If the new output still differs, compare it with the original Tally PDF and obtain a read-only XML export of that same voucher to identify custom ledger names or export fields. Account numbers and personal details may be redacted while preserving the amount and ledger structure.
