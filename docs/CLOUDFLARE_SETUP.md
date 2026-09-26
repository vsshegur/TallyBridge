> **Legacy 2.0 Access preview document.** For 2.0.1 direct Google sign-in without a Cloudflare payment card, follow [NO_CARD_SETUP.md](NO_CARD_SETUP.md). The Access/team/AUD instructions below do not apply to the new setup.

# TallyBridge: Cloudflare setup, step by step

Updated 25 September 2026. For the native Android preview at commit `88d7107`.
Windows tests/build and Android compilation/lint passed in [GitHub Actions run 2](https://github.com/vsshegur/TallyBridge/actions/runs/36102780940).
Real phone, Cloudflare login and Tally/Edge testing still need to be completed.

## Before you start

You need your Windows Tally PC, your Android phone (Android 9+ with a screen lock),
a domain you control, a Cloudflare account and the Google account you want to allow.
Keep a backup of your working installation before testing the preview.

Download both build artifacts from the successful run linked above. Extract the
Windows ZIP and run `TallyBridge_Setup.exe`. Extract the Android ZIP and install
`app-debug.apk` on your phone. These are preview builds. The APK uses debug signing;
future builds may require uninstalling this preview and pairing again.

Choose these values and keep them handy. All values below are examples:

| Item | Example | Where it is used |
|---|---|---|
| Your existing domain | `example.com` | Cloudflare DNS |
| New app subdomain | `tally.example.com` | Access application, tunnel, PC and phone |
| Cloudflare team name | `my-tally-team` | Zero Trust organization |
| Team domain | `my-tally-team.cloudflareaccess.com` | Google OAuth and PC settings |
| Allowed Google email | `owner@gmail.com` | Access policy, PC settings and phone sign-in |
| Access application AUD | Copy the full value from Cloudflare | PC settings only |

The team domain and your app subdomain are different addresses. Do not substitute
one for the other. Your Windows login email need not match: the relevant email is
the one you enter in TallyBridge's desktop setup and verify on Android.

## 1. Get the domain ready

If your domain already shows Active in Cloudflare, skip this step.
Otherwise, add your root domain to Cloudflare and review its imported DNS records.
Preserve records for your existing website and email, including MX and TXT records.
At your domain registrar, replace the nameservers with the exact pair Cloudflare
assigns. If old DNSSEC is enabled, follow Cloudflare's migration instructions before
switching, then re-enable it once the domain is active. Wait for Active status.
You keep the same domain registrar; this changes DNS hosting, not domain ownership.

Use a fresh subdomain such as `tally.example.com`. Do not reuse a hostname that
already serves another app. [Official domain setup](https://developers.cloudflare.com/dns/zone-setups/full-setup/setup/).

## 2. Create your Cloudflare Zero Trust organization

Open [Cloudflare Dashboard](https://dash.cloudflare.com), choose your account,
and open Zero Trust / Cloudflare One. Complete organization onboarding and select
a plan appropriate to the options shown in your account. Choose a team name.
Find **Settings → Team name and domain** and copy the full team domain ending in
`.cloudflareaccess.com`. Dashboard labels may vary; use its search if needed.

Do not change the team name later without also updating Google's callback and
TallyBridge's saved team domain. [Team domain reference](https://developers.cloudflare.com/cloudflare-one/glossary/).

## 3. Set up Google sign-in

1. Open [Google Cloud Console](https://console.cloud.google.com), create/select a
   project named `TallyBridge Access`, and open Google Auth Platform / OAuth consent.
2. Configure Branding with an app name, support email and contact email. Use
   External audience for a personal Gmail account. If the app is in Testing,
   add your allowed email under Audience → Test users.
3. Create an OAuth client of type **Web application**. Name it `Cloudflare Access`.
4. Set its JavaScript origin to `https://my-tally-team.cloudflareaccess.com`.
5. Set its redirect URI to
   `https://my-tally-team.cloudflareaccess.com/cdn-cgi/access/callback`.
6. Copy the client ID and secret. In Cloudflare open **Zero Trust → Integrations →
   Identity providers → Add new identity provider → Google**. Enter them and Save.
7. Select Test next to Google and check that your account signs in successfully.

Replace the sample team name with yours. Keep the client secret in Cloudflare's
provider configuration; it does not go in TallyBridge, GitHub or the APK. Choose
Google, not Google Workspace, unless you intentionally need that separate integration.

[Google integration](https://developers.cloudflare.com/cloudflare-one/integrations/identity-providers/google/) ·
[Google consent and test users](https://developers.google.com/workspace/guides/configure-oauth-consent).

## 4. Protect your app subdomain with Access

In Cloudflare go to **Zero Trust → Access controls → Applications → Create new
application**. Choose **Self-hosted and private**, then **Add public hostname**.
Use the following configuration:

| Setting | Value |
|---|---|
| Application name | `TallyBridge` |
| Subdomain / domain | `tally` / your root domain |
| Path | Empty, protecting the entire hostname |
| Policy name | `Only my Google account` |
| Policy action | Allow |
| Include selector | Emails |
| Email value | Your one exact Google email |
| Login methods | Only the Google provider you just configured |
| Accept all identity providers | Off, if shown |
| Application session duration | 24 hours as an initial choice |

Do not add Everyone, email-domain-wide access, Bypass or Service Auth policies.
Leave Cloudflare One Client authentication off for this setup. Instant
authentication is optional when only Google is selected. Save the application.
The application needs its Access policy even though TallyBridge also checks email.

[Access application setup](https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/self-hosted-public-app/).

## 5. Enable the Android OAuth flow and copy the AUD

Edit that Access application, open **Advanced settings**, and configure:

| Setting | Value for this app |
|---|---|
| Managed OAuth | On |
| Dynamic client registration | On |
| Allow loopback clients | On — required for `127.0.0.1` |
| Allow localhost clients | Not required by this implementation |
| Extra allowed redirect URIs | Not required for this loopback flow |
| Access token lifetime | 15 minutes |
| Grant session duration | 24 hours initially |

Save. Then open the application's **Additional settings** and copy its
**Application Audience (AUD) Tag** in full. It is not the Google client ID, tunnel
ID, account ID or zone ID.

The Android app opens the browser for login and receives the callback on a temporary
phone-local port. Do not add that changing port to Google's client: Google's callback
remains the fixed Cloudflare team-domain URL in step 3. Short-lived access tokens can
be renewed until the grant expires; the user may then need to sign in again.

[Managed OAuth](https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/managed-oauth/) ·
[Finding AUD](https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/authorization-cookie/validating-json/).

## 6. Save the matching settings on your Tally PC

Open TallyPrime and the intended company. Start TallyBridge, then open
`http://127.0.0.1:8765/admin` on that same PC. Select **Cloudflare** in the sidebar.

| TallyBridge field | What you enter |
|---|---|
| Your HTTPS subdomain | `https://tally.example.com` — no path or port |
| Cloudflare Access team domain | `my-tally-team.cloudflareaccess.com` — no `https://` |
| Access application audience (AUD) tag | The complete AUD copied in step 5 |
| Owner's allowed Google email | Exactly the email allowed in step 4 |

Click **Save connection settings**. This stores settings on the PC; it does not
create Cloudflare resources. Changing existing owner/connection settings revokes
previous phone approvals, so finish these settings before pairing.

Open **PDF Bank**, choose each company and save its default bank. The native
phone app uses this saved bank for PDFs without asking you to select it every time.

## 7. Install the tunnel connector on that PC

Open Cloudflare **Networking → Tunnels → Create a tunnel**. Choose cloudflared
if asked for a connector type, name it `Tally-PC`, and select Windows / 64-bit.
Follow its Install and Run instructions on the Tally PC. Open Command Prompt as
administrator for the service-install command. The command has this shape:

```bat
cloudflared.exe service install YOUR_PRIVATE_TUNNEL_TOKEN
```

Use the actual command Cloudflare displays. Do not type the placeholder or share
the command/token. Wait for the tunnel to become Healthy, then continue.

In the tunnel's **Routes → Add route → Published application**, enter:

| Field | Value |
|---|---|
| Hostname | `tally.example.com` |
| Path | Empty |
| Service type, if separate | HTTP |
| Service URL | `http://127.0.0.1:8766` |
| URL, if type is a separate dropdown | `127.0.0.1:8766` |

Save. Dashboard route creation normally creates the associated DNS record.
If a record already exists at this name, inspect it before replacing anything.
Some accounts show these options under Networks → Connectors / Published
application routes. Use the same field values.

[Official tunnel instructions](https://developers.cloudflare.com/tunnel/get-started/).

**Only port 8766 belongs in the tunnel.** Port 8765 is desktop setup; port 9000
is Tally. Neither should be published. The local HTTP hop stays on the PC; Android
uses the public HTTPS address. No router port forwarding is needed for this design.

## 8. Check the connection, then pair Android

1. In a private browser window, open
   `https://tally.example.com/api/v2/native/identity`. It should require Access
   authentication. Depending on how the request is classified, you may see a
   login page or an OAuth 401 challenge. After browser login, this identity endpoint
   can show your email; it does not expose accounting reports.
2. You can inspect
   `https://tally.example.com/.well-known/oauth-authorization-server` to confirm
   OAuth metadata exists. JSON metadata here is intentionally public, not Tally data.
3. Open Android TallyBridge. Unlock with your phone's screen lock, enter
   `https://tally.example.com`, and tap **Sign in with Google**.
4. Choose the exact allowed account. If the browser says “Sign-in received”,
   return to TallyBridge. The app should show your verified email.
5. On the PC, open **Android Phones** and generate a pairing code. Enter the
   six-digit code in Android within five minutes and request approval.
6. Refresh the PC phone list. Compare the fingerprint displayed on both devices.
   Approve only the matching phone; finish approval within ten minutes.
7. On Android tap **I've approved it — continue**, then select the company.
8. Switch the phone to mobile data and check a report and PDF against Tally.

Opening just the subdomain root may give 404 after login: this release exposes an
Android API there, not a phone website. Always open PC setup using the local URL.

## 9. Daily use and additional owners

Keep Windows, TallyBridge and cloudflared running, with Tally available for live
reports. Configure the PC not to sleep during the periods when remote access is
needed. The installer sets up the bridge for the Windows user; check its launch
and the tunnel service after a restart. Saving data on the PC does not make it
available while the PC or bridge is off.

A phone normally pairs once; reinstall/reset/new phone requires approval again.
Revoke a lost phone in desktop **Android Phones**. Revocation blocks future requests,
but cannot erase PDFs already exported to another app.

Each other owner uses their own PC, hostname, tunnel, Access application and allowed
email. Sharing the generic APK does not share your data. Do not share the installed
`%LOCALAPPDATA%\TallyBridge` directory or your tunnel token. Cloudflare processes
proxied traffic; the accounting store remains on the PC. This is not a guarantee
against every possible data leak.

## Troubleshooting

| What you see | What to check |
|---|---|
| Domain absent from the selector | Correct Cloudflare account; domain Active |
| Tunnel not Healthy | cloudflared service running; PC has Internet; correct connector token |
| Gateway / origin connection failure | Bridge running; service is HTTP at `127.0.0.1:8766` |
| Google `redirect_uri_mismatch` | Fixed team-domain callback matches step 3 exactly |
| Google blocks a test account | Test-user list and any Workspace administrator restrictions |
| Access denies the email | Exact email in Allow policy; correct Google provider selected |
| Android reports missing/invalid OAuth metadata | Managed OAuth enabled on the exact hostname's application |
| Client registration/redirect rejected | Dynamic registration and Allow loopback clients enabled |
| Login succeeds but bridge rejects identity | PC's team domain, AUD and email match this Access application; PC clock correct |
| Phone remains pending | Matching fingerprint approved on PC before expiry; refresh both screens |
| Old phone stops after setting changes | Expected revocation; sign in and pair again |
| PDF fails but reports load | Microsoft Edge installed; correct company bank configured; inspect PC diagnostics |
| APK update will not install | Different preview signing key; uninstall old preview only if ready to reset and pair again |

If a step fails, share the exact error and a screenshot with secrets hidden.
Do not disable Access, change the policy to Everyone, or expose the setup port to
work around a login failure. Keep the working older installation until you have
validated sign-in, report accuracy, PDFs, sharing and revocation on your devices.
