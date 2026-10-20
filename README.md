# TallyBridge

**New in 2.9.0:** InvoiceTotally signs in with your Google account and each new browser is approved on this PC; Customers and Search removed from the PC menu — see [2.9.0](docs/UPDATE_2_9_0.md).

**2.8.1:** the PC control center scrolls properly on smaller screens — see [2.8.1](docs/UPDATE_2_8_1.md).

**2.8.0:** check GSTR-2B against the purchase register in InvoiceTotally (TallyBridge reads purchases with their GST, read-only); a tidier Android app with one header, clearer drop-downs and dialogs in Light and Dark — see [2.8.0](docs/UPDATE_2_8_0.md).

**2.7.0:** match bank statements (any bank, including co-operative banks) with Tally through InvoiceTotally and post Receipts, Payments and Contras after review; a simpler Android app (Home · Parties · Orders · Reports · More) and better widgets — see [2.7.0](docs/UPDATE_2_7_0.md).

**2.6.0:** send purchase bills from the phone to [InvoiceTotally](https://github.com/vsshegur/invoicetotally) and post the reviewed bills to Tally from this PC — see [2.6.0 purchase bills](docs/UPDATE_2_6_0.md).

Upgrading from 2.0.x? Read [2.0.2 installation and design changes](docs/UPDATE_2_0_2.md).

Start with [the no-card setup guide](docs/NO_CARD_SETUP.md). This version uses
Cloudflare Tunnel plus direct native Google sign-in. Cloudflare Access, its team
name and its payment-card signup are not required.

**Downloads:** every build of `main` attaches the Windows installer (`TallyBridge-<version>-Setup.exe`) and the unsigned Android APK to the private **preview-<version>** release under Releases. The APK is signed with the permanent TallyBridge key when the repository has the four signing secrets (see [release signing](docs/RELEASE_SIGNING.md)); otherwise it is unsigned and must be signed before installing.

## What is included

- Native Java Android app, Android 9+, using platform Views, AndroidX Credential
  Manager, Google sign-in, Android Keystore, screen-lock authentication, native
  PDF rendering and sharing. Accounting screens do not use a WebView.
- Go Tally bridge with local desktop setup on 8765 and an isolated
  Android listener on 8766. Everything the phone can do is read-only.
- Optional InvoiceTotally connection (Purchase bills): bills photographed on the
  phone are read by InvoiceTotally and, once reviewed there, posted to Tally by
  this PC. Posting is off until switched on and accepts only new vouchers and
  masters for the expected company.
- Saved default PDF bank per company, reports and existing PDF engines.
- Windows installer and Android preview APK built by GitHub Actions.

## Owner email and PC approval

Each owner runs their own PC and configures their own hostname, Google Web client
ID and exact allowed Gmail or Workspace email. The native app signs in with
Google; the PC verifies Google's signature, issuer, audience, expiry and email.
The Google token is bound to the phone key, and requests require signatures.

A new phone also needs a one-use five-minute PC code and matching fingerprint
approval on the PC. Unapproved/revoked phones cannot read reports. Configuration
changes revoke previous approvals. Google sign-in must be repeated when the
short-lived token expires; existing device approval remains.

There is no central accounting database. Each PC keeps its own data and device
allowlist. Exported PDFs cannot be recalled. Cloudflare proxies HTTP content at
its edge; this is not an end-to-end private VPN.

This is a preview: live Google sign-in, physical-device use and Windows/Tally PDF
integration still require testing. Keep your working installation backed up.
CI uses temporary debug signing keys, so future APKs may require a new Google
Android OAuth SHA1 entry and reinstall/re-pairing. Stable release signing is
still required for dependable public updates.

## Build Android on a Windows PC

Extract this entire package. Open `android` and double-click `Build-Android.cmd`.
It uses an existing JDK or downloads Eclipse Temurin JDK 17, downloads Google's
SDK tools and Gradle 8.11.1, prompts for SDK licence acceptance, then runs
`assembleDebug` and `lintDebug`. A successful build places the preview APK in
`android/output/TallyBridge-Android-Preview.apk`.

This helper has not been executed on Windows in this environment. If a step
fails, retain the output and resolve it before treating the APK as tested.
Android Studio can also open the `android` directory directly. The pinned build
is AGP 8.9.2 / Gradle 8.11.1 / compile and target SDK 35 / JDK 17.

The raw offline SDK builder is disabled because native Google sign-in now needs
AndroidX and Google dependencies. Use Gradle or Build-Android.cmd.

Preview builds use a locally generated debug/preview signing key. Do not publish
a preview as a production release. For a release, retain a private signing key
outside the distributable and set TB_KEYSTORE, TB_STORE_PASSWORD, TB_KEY_ALIAS
and TB_KEY_PASSWORD before `gradle assembleRelease`. Never bundle the private
signing key or live owner settings.

## Build Windows

With Go 1.23 or newer, from this source root:

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags='-s -w -H=windowsgui' -o build/TallyBridge.exe ./cmd/tallybridge
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags='-s -w -H=windowsgui' -o build/TallyBridge_Uninstall.exe ./cmd/uninstall
cp build/TallyBridge.exe cmd/setup/payload/TallyBridge.exe
cp build/TallyBridge_Uninstall.exe cmd/setup/payload/TallyBridge_Uninstall.exe
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags='-s -w -H=windowsgui' -o build/TallyBridge_Setup.exe ./cmd/setup
```

The production entry point uses `DesktopHandler` on localhost:8765 and
`NativeHandler` on localhost:8766. The older combined Handler and browser-phone
assets remain only as regression/reference code; neither production listener
serves the browser phone UI or legacy pairing. There is no Tailscale dependency
in the new startup workflow.

The PC and TallyBridge must be running for both live and PC-cached reports. The
phone retains pairing and encrypted login tokens, not an offline accounting
database. PDF generation still requires Microsoft Edge on Windows. Tally requests
remain Export-only, sequential, time-limited and protected by recovery cooldowns.
