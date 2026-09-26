# TallyBridge 2.0.2 native design preview

Upgrading? Read [2.0.2 installation and design changes](docs/UPDATE_2_0_2.md).

Start with [the no-card setup guide](docs/NO_CARD_SETUP.md). This version uses
Cloudflare Tunnel plus direct native Google sign-in. Cloudflare Access, its team
name and its payment-card signup are not required.

**Windows and Android builds passed:** [download run 11 artifacts](https://github.com/vsshegur/TallyBridge/actions/runs/36229848684). Android compilation/signing/lint and Windows tests/vet/build passed.

## What is included

- Native Java Android app, Android 9+, using platform Views, AndroidX Credential
  Manager, Google sign-in, Android Keystore, screen-lock authentication, native
  PDF rendering and sharing. Accounting screens do not use a WebView.
- Read-only Go Tally bridge with local desktop setup on 8765 and an isolated
  Android listener on 8766.
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
