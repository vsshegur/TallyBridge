# TallyBridge 2.0 Native Preview

**Status: installable preview; Windows and Android CI builds passed.**
[Download both build artifacts](https://github.com/vsshegur/TallyBridge/actions/runs/36102780940).
Android compilation, APK packaging/signing and lint passed; Windows backend tests,
vet and installer compilation passed. Real-device, Google/Cloudflare login and
live Tally/Edge verification remain outstanding.

Start with [the detailed Cloudflare setup guide](docs/CLOUDFLARE_SETUP.md).

Keep your working 1.6 installation until the native APK is tested on your devices. The
2.0 Windows installer replaces the browser-phone workflow with Android access.
It retains PC settings, bank defaults and report cache, but old browser pairing
credentials cannot authorize a native phone.

## What is included

- `android/`: native Java Android app (Android 9/API 28 or newer), using platform
  Views, ListView recycling, Android Keystore, screen-lock authentication,
  PdfRenderer, system share sheet, and the Android file picker. No WebView,
  Capacitor, React Native, HTML accounting screens or external app libraries.
- `internal/app/`: the read-only Go Tally bridge, existing report/PDF engines,
  isolated Android listener and new authorization checks.
- GitHub Actions artifacts: Windows x64 installer, bridge, uninstaller and Android APK.
- `docs/SETUP.md`: owner setup and Cloudflare requirements.
- `docs/FEATURE_PARITY.md`: feature inventory and implementation locations.
- `docs/VERIFICATION.md`: exactly what was and was not verified.

## Owner email + PC code authorization

This is reusable software, not a shared accounting service. Each owner installs
it on their own Windows PC and configures their own subdomain, Cloudflare Access
application and allowed Google email. No owner account or production data is
bundled in the distributable.

1. On desktop setup, the owner enters the one Google email that may access this PC.
2. The same exact email must be allowed in the owner's Cloudflare Access policy,
   with Google as the only login method.
3. Android signs in through the system browser. The PC verifies the signed
   Cloudflare assertion, issuer, audience, expiry, token type and exact email.
4. Android displays that verified email, then requests the PC's temporary code.
5. The code is single-use and expires after five minutes. At most five pairing
   attempts are accepted per minute.
6. The PC shows the phone's public-key fingerprint for approval. Match it against
   Android. An unapproved phone cannot read reports. Approval expires after ten
   minutes if not completed.
7. Approved requests require that phone's P-256 signature and a valid Google/
   Access session. Timestamp checks and one-use nonces reject replay attempts.
8. Revocation blocks subsequent server requests. Changing owner email, subdomain,
   team domain or Access audience revokes every previously approved phone.

Each independent PC maintains its own device allowlist, settings, cache and bank
choices. A phone approved by one installation is not approved by another. There
is no central Tally database in this project. The APK is generic; connection
settings are entered on each phone. One server connection is active per app
installation; reset pairing before changing to a different server.

Exported PDFs/messages cannot be recalled from other apps. Cloudflare processes
proxied HTTP content at its edge; this is not the same privacy model as
Tailscale's end-to-end encrypted device network. Windows account security and
correct Cloudflare configuration still matter. No system can promise zero risk.

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

For an offline Linux builder with SDK platform 35 and build-tools 35.0.0 already
installed, `android/build-offline.sh` uses javac, aapt2, D8, zipalign and apksigner
without Gradle or external app dependencies. Set ANDROID_HOME first.

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
