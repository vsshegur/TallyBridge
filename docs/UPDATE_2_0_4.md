# TallyBridge 2.0.4 — session and report recovery

Install the Windows update first, then the Android update. Sign in once and complete desktop phone approval if asked. Existing desktop settings and saved bank details are retained.

Approved phones receive an encrypted-on-phone session, bound to their Android Keystore private key. Every API request still requires its signature, timestamp and a fresh nonce. Sessions expire after 30 days without use. Revocation and changing the allowed desktop email disable access; a new Google sign-in is required to pair or obtain a new session. Short-lived Google ID token expiry no longer ends an approved session.

Desktop synchronization retries failed cycles every 30 seconds after the safety cooldown, while successful cycles keep the 3-minute interval. One unsupported report does not block the other reports. Reopening the phone asks the desktop to refresh stale data. The foreground phone retries temporary connection failures. Background PC synchronization continues when the phone app is closed; this does not force Android to run in the background.

Outstanding parsing supports bill references followed by separate pending balances and due dates, filters other parties, and prioritizes closing over original amounts. Text and PDF include debit/credit labels. If Tally genuinely supplies no bill references, only its known ledger balance can be shown.

PDF generation detects Edge and Chrome installations and accepts complete printed files even if browser shutdown fails. Errors remain visible with a Retry action in Android. Invoice document colours are preserved.

This preview APK still needs its own signing fingerprint registered in the existing Google project. Use the delivered SIGNING-REPORT.txt; keep the existing Web client ID. If installation conflicts with the previous preview, reinstall and pair again.

Physical-PC Tally and third-party WhatsApp sharing must still be checked on the user's devices. This update does not restart Tally or change its accounting data.
