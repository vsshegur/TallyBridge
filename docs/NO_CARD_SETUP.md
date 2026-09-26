> For the redesigned 2.0.2 APK, use the fingerprint in [UPDATE_2_0_2.md](UPDATE_2_0_2.md). The older run 4 fingerprint below only applies to the old APK.

# TallyBridge 2.0.1: setup without Cloudflare Access

This version uses Cloudflare Tunnel for HTTPS connectivity and native Google sign-in for identity. It does not need a Zero Trust team name, Access audience, Access subscription or payment-card signup. Your already healthy tunnel can be reused. This is an installable preview; live phone/Google/Tally testing is still required.

## 1. Install the matching Windows and Android builds

Download the Windows and Android artifacts from the successful GitHub Actions run linked in README. Extract both ZIPs. Back up your existing Windows data folder (%LOCALAPPDATA%\TallyBridge), close TallyBridge and run TallyBridge_Setup.exe. Install app-debug.apk on an Android 9+ phone with Google Play services and a screen lock.

If Android says the new APK conflicts with the installed app, uninstall the previous preview, then install this one and pair again. Existing exported PDFs are not revoked by uninstalling.

The Android artifact contains SIGNING-REPORT.txt. Use the SHA1 under Variant: debug when creating the Google Android client below. The fingerprint identifies this exact APK, not your phone.

For the supplied [run 4 APK](https://github.com/vsshegur/TallyBridge/actions/runs/36214327650), the SHA-1 is:

```text
6A:C7:23:16:44:12:95:74:B3:C6:6D:5F:74:A9:A5:FB:6C:45:D0:1F
```

## 2. Create Google's sign-in configuration

Open https://console.cloud.google.com/ and create/select a project named TallyBridge. You only need Google Auth Platform OAuth configuration; do not enable a paid cloud service or start a billing trial for this setup.

1. Open Google Auth Platform (or search for OAuth consent screen). Click Get started if shown.
2. Set app name to TallyBridge and choose your support/contact email.
3. Choose External audience for a personal Gmail account. Keep Testing status and add the exact owner Google email under Audience > Test users. A Workspace organization may instead use Internal if appropriate.
4. Under Clients, choose Create client > Web application. Name it TallyBridge PC. For this native ID-token flow, leave JavaScript origins and redirect URIs empty.
5. Copy the Web client ID ending in .apps.googleusercontent.com. This goes into desktop setup. You do not need the client secret.
6. In the SAME project, create another client of type Android.
7. Package name: in.tallybridge.app
8. SHA-1 certificate fingerprint: copy the debug SHA1 from SIGNING-REPORT.txt supplied with your APK.
9. Save. The Android client ID stays in Google; do not paste it into TallyBridge's Web client ID field.

Use a Gmail account or a Google Workspace account with a Google-hosted email domain. This version intentionally rejects third-party addresses used merely as Google account usernames.

Official guidance: https://codelabs.developers.google.com/sign-in-with-google-android and https://developer.android.com/identity/sign-in/credential-manager-siwg-implementation

## 3. Configure TallyBridge on the Windows PC

Open http://127.0.0.1:8765/admin on the PC running TallyBridge. Open Cloudflare and enter:

| Field | Value |
|---|---|
| Public URL | https://tally.packpilot.in |
| Google Web client ID | Web application client ID from step 2 |
| Allowed Google email | Exact Google account permitted to use this PC |

Save. These settings are local to your PC; the form does not create Google or Cloudflare resources. Changing these settings revokes existing approved phones.

Open TallyPrime and the required company. Configure/test the Tally connection in desktop setup. Save your default PDF bank once per company.

## 4. Add the route to your existing Cloudflare Tunnel

Your screenshot showed a healthy connector but zero routes. Open that same tunnel, then Routes > Add route. Choose a Published application / public hostname route if prompted, rather than a private network/CIDR route.

Enter:
- Subdomain: tally
- Domain: packpilot.in
- Path: leave blank
- Service type: HTTP
- Service URL: 127.0.0.1:8766

If a single URL field is shown, enter http://127.0.0.1:8766. Save. The connector must run on the same Windows PC as TallyBridge.

Do not route to 8765 (desktop setup) or 9000 (Tally). Do not choose a Zero Trust plan or configure Access for this flow. If an Access application was already created for this exact hostname, its login screen will block the native flow; remove that hostname's Access requirement only after saving the new Google configuration. Leave unrelated Access applications untouched.

The tunnel name itself does not create a public hostname. Check that the saved route shows tally.packpilot.in.

To check connectivity, open https://tally.packpilot.in/api/v2/native/config in a browser. It should show only the public Google client ID and server URL. It must not show accounting data. A Cloudflare login page means Access still protects the hostname; 502 usually means the local bridge/listener is not running.

## 5. Connect and approve Android

1. Open TallyBridge on your phone and enter https://tally.packpilot.in.
2. Tap Google sign-in and select the exact email saved on the desktop.
3. On desktop setup, open Android Phones and generate a pairing code.
4. Enter that code on the phone within five minutes.
5. Compare the phone's fingerprint with the pending phone on the PC, then approve it on the PC.
6. Unlock with the phone's screen lock when prompted, then test reports and PDF sharing.

The code alone does not grant access. The server checks the Google identity, the phone key and your explicit approval. An unapproved phone cannot read reports.

## Everyday use and limits

Keep the PC, TallyBridge, Tally company and Cloudflare connector running. The PC needs Internet access to verify Google signing keys. You can revoke a phone from desktop setup.

Google sign-in currently expires periodically (typically about one hour); sign in again when requested. Your existing phone approval remains, so a new pairing code should not be required unless you revoke/reset it or change owner settings.

Each owner uses their own PC, hostname, Google configuration and allowed email. There is no central accounting database. Google handles sign-in and Cloudflare proxies the connection; this is not an end-to-end private VPN.

These preview APKs use a fresh debug signing key for each CI run. A future APK may require another Android OAuth client fingerprint, uninstall/reinstall and re-pairing. A stable private release-signing key must be configured before dependable public updates. Never publish a signing keystore or password.

## Troubleshooting

- Google sign-in fails: ensure both clients are in the same project, the desktop uses the Web client ID, the Android package/SHA1 match this APK, and your account is an allowed test user. Allow time for Google configuration changes to propagate.
- Wrong email / authorization rejected: use exactly the configured Gmail or Workspace email. A Google account using a non-Google-hosted email is unsupported here.
- Configuration unavailable: save the new Google configuration in the updated Windows app; the old Access-only EXE is incompatible with this flow.
- Pairing code rejected: generate a fresh code, use it once, check PC/phone clocks and retry after a minute if you hit the attempt limit.
- No reports: approve the matching fingerprint, open your Tally company and test the desktop Tally connection.
- PDF not created: Windows PDF rendering and Edge still need testing on your PC; compare the result against Tally before relying on it.
