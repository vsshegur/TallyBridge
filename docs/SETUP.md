# Owner setup — each installation is independent

Do not install the 2.0 Windows preview over your working 1.6 setup until the
Android APK has been built. This build does not include an APK.

## 1. Prepare the owner's PC

- Use Windows x64 with TallyPrime and Microsoft Edge installed.
- Keep Tally's HTTP server on localhost, normally port 9000, and open the company.
- Back up `%LOCALAPPDATA%\TallyBridge` before testing an upgrade. That directory
  contains private settings and saved reports; never distribute it to others.
- Install `build/TallyBridge_Setup.exe` when ready to test Android.
- Desktop setup opens locally on `http://127.0.0.1:8765/admin`.
- Use PDF Bank to save the default bank separately for each company.

## 2. Create your own Cloudflare connection

Each owner needs their own Cloudflare-controlled hostname and Access application.
Sharing the APK is fine. Sharing a live tunnel token, owner settings, local data
folder, or Access credentials is not part of distribution.

In Cloudflare, create a tunnel/connector for this Windows PC and install
cloudflared using the official connector instructions. Configure the published
hostname (for example, `tally.your-domain.com`) to use:

`http://127.0.0.1:8766`

**Never forward localhost:8765 (desktop setup) or localhost:9000 (Tally).**
No router port forwarding is required. Keep cloudflared running, preferably as a
Windows service configured using Cloudflare's instructions.

Protect the entire hostname with a self-hosted Cloudflare Access application:

1. Configure Google as the only identity provider offered for this application.
   A personal Google account is supported; Google Workspace is not required.
2. Add an Allow policy for exactly the owner email chosen in desktop setup.
3. Do not add Everyone, Bypass, or service-token policies to this application.
4. Enable **Managed OAuth**, dynamic client registration, and **Allow loopback
   clients**. Android uses a short-lived listener on its own 127.0.0.1 address to
   receive the browser's authorization code, protected by state and PKCE S256.
5. Start with a 15-minute access-token lifetime and a suitable grant/session
   duration. The app uses refresh tokens when provided; expired sessions require
   Google sign-in again without losing the phone key.
6. Copy the application's audience (AUD) tag and your team's
   `your-team.cloudflareaccess.com` domain.

In desktop setup → Cloudflare, enter:

- HTTPS subdomain, without an extra path or port;
- Access team domain;
- Access application audience (AUD);
- the exact allowed Google email.

These are owner-selected settings, not values embedded in the APK. Saving them
configures the bridge; it does not provision DNS, Google OAuth or Cloudflare
resources in the owner's account. The bridge fails closed until configured and
independently validates Cloudflare's signed assertion on every phone request.

Reference documentation:
- https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/managed-oauth/
- https://developers.cloudflare.com/cloudflare-one/integrations/identity-providers/google/
- https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/authorization-cookie/application-token/
- https://developers.cloudflare.com/tunnel/

## 3. Approve Android

1. Build and install the preview APK on Android 9 or newer. Enable a phone screen
   lock. Android may ask you to permit APK installation from your chosen app.
2. Open TallyBridge and unlock with fingerprint or the phone's screen-lock PIN.
3. Enter your owner's HTTPS subdomain and choose Sign in with Google.
4. Sign in using the exact email configured on that PC. The app shows the verified
   email before asking for a pairing code. A different email is rejected.
5. On desktop setup → Android Phones, generate a code. Enter it in the phone app.
6. Refresh the desktop phone list. Compare the fingerprint on both devices, then
   approve the matching phone. Tap “I've approved it — continue” on Android.
7. Choose your Tally company. Reports will show live or last-updated status.

Once approved, normal use does not require a new code every time. The app locks
when backgrounded. A new phone, reinstall, or reset requires a new code and
approval. The PC can revoke a phone; changing owner/connection settings also
revokes old approvals.

## 4. Verify before everyday use

- Try a wrong Google email; it must not reach reports or pairing.
- Sign in correctly without a code/approval; reports must remain blocked.
- Compare the dashboard and each report with Tally.
- Test a long invoice, Marathi names, discounts, taxes, date statements and PDFs.
- Switch companies; confirm the company-specific default bank is used.
- Share details, PDF, and PDF + details through your installed WhatsApp version.
  The receiving app decides how it displays an attachment's caption/message.
- Revoke the phone while it is open; the next authorization check/request must
  block it. Already displayed or exported information cannot be remotely erased.
- Restart the PC and phone; confirm settings, approval and login renewal work.
- Turn Tally off while the bridge stays on; check saved timestamps are clear.
- Turn the bridge off; Android must report it unavailable and retain pairing.

The Cloudflare/Google browser round trip and Windows PDF engine were not tested
live in the build environment. Resolve those gates before using this as a
production replacement or distributing an installable release.
