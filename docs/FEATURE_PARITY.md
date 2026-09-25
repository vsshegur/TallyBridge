# Native phone feature inventory

All entries below describe implemented source, not device-verified behavior.
The Android app is written in Java with native Android Views. No accounting
screen loads a webpage. The system browser is used only for Google/Access login.

| Previous phone feature | Native implementation |
|---|---|
| PC pairing and authorization | Verified owner email, temporary code, matching-key approval, signed requests |
| App PIN lock | Android fingerprint/device screen-lock authentication |
| Company selection | Company chooser in the header, isolated request/cache context |
| Overview | Receivable, payable, month sales, sync and connection status |
| Last-updated data | Live/saved banners, original timestamps and read-failure message |
| Ledger index | Recycled searchable native list |
| Ledger groups | Searchable groups, group balances and ledger navigation |
| Ledger details | Contact, address, GST/PAN and bank fields returned by Tally |
| Ledger date presets | Today, this month, last 30 days, Indian financial year |
| Custom statement dates | Native Android date pickers |
| Ledger statement | Opening/closing, debit/credit, running balance, voucher detail |
| Receivable/payable | Separate party lists and party outstanding drill-down |
| Bill-wise outstanding | Bill/date/due date/age, Dr/Cr, verified summary-only fallback |
| Sales and purchase registers | Financial year → month → searchable vouchers → detail |
| Invoice detail | Customer, products, rates, amounts, dispatch, discount/tax/total |
| Omit zero discount from shared details | Uses existing server-generated share text |
| Invoice PDF | Existing PC PDF engine and company-specific saved bank |
| Actual PDF preview | Android PdfRenderer, previous/next page, pinch zoom/pan |
| Details-only sharing | Native Android text share sheet |
| PDF sharing | Temporary read-only content URI with Android share sheet |
| Statements/outstanding/group PDFs | Details/PDF sharing through existing endpoints |
| Universal search | Saved ledgers, customers, invoice numbers and known LR/RR |
| Today's dispatch | Today's vouchers and invoice drill-down |
| Ageing | Saved bill buckets with explicit partial-data coverage |
| Customer 360 | Balance, overdue, last payment, invoice/activity drill-down, PDF/text |
| Day Book, Trial Balance, P&L, Balance Sheet | Native report rows from existing normalization |
| Bank selection | Desktop-only per-company saved default; no phone chooser |
| Automatic sync | Existing PC sequential worker; native status polling |
| Revocation/unpair | Desktop revoke; Android reset with reapproval |
| New convenience additions | Favourite ledgers; PDF + details share; save PDF to selected folder |

The historical web-phone assets are retained for regression/reference only.
There is no new phone website endpoint. The native source resides in
`android/app/src/main/java/in/tallybridge/app`:

- MainActivity: native navigation, lists, authentication UI, reports and PDF actions.
- AccessLogin: public OAuth client, browser launch, loopback callback, PKCE, refresh.
- CryptoStore: Android Keystore signing and encrypted token storage.
- BridgeApi, Http, Protocol: HTTPS, exact request signing and API handling.
- PdfPageView, PdfProvider: native preview and narrowly scoped file sharing.

App use requires the PC bridge online; it does not create a phone-side offline
accounting database. Unknown balances are displayed as unavailable/not updated,
not silently converted into verified zero balances. Search/ageing retain the
same saved-data coverage limits as the existing backend.
