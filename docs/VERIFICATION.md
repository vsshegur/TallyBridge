# Design/startup update — 26 September 2026

Windows backend tests/vet and installer build passed. Android compilation and
lint passed. The Android 11 emulator installed the app and passed the fixture
screen test (unlock, dashboard with long company/large balance, reports, connect).
The CI tools now use one explicit AVD directory and app-owned screenshot files.

The first screenshot review found balance text wrapping on very narrow screens.
Dashboard amounts now resize to fit one line. Final screenshots were reviewed at a phone-size display configuration.
Run 11 (36229848684), source e924949be5ded543d981656f01d1fa1be1f29758,
passed both Windows and Android jobs including emulator checks and screenshot export.
Downloaded artifacts matched GitHub SHA-256 digests.

Actual Windows restart/sign-in and physical-device interaction remain to be
checked. The owner reported the preceding version's Google/Tally flow working;
authentication and report request logic are unchanged in this design update.

---

# Direct Google update — 26 September 2026

[GitHub Actions run 4](https://github.com/vsshegur/TallyBridge/actions/runs/36214327650),
source 2ac5874ce339adfeb4b338e05dc7fc034dda1f85: Windows tests/vet and installer
build passed; Android compilation, signing and lint passed. Both artifact ZIP
SHA-256 values matched GitHub's recorded digests. Nonblocking lint warnings remain.

Passed locally: Go package tests, race detector and vet; desktop JavaScript syntax.
New authentication tests cover Google signature/claims, exact owner email, hosted
Workspace identity, phone-key token binding, request tampering, nonce replay,
pair-key substitution, pending approval, successful approval and revocation.
The public configuration endpoint is tested not to expose the owner email.

Android now uses AndroidX/Google dependencies. The old raw offline SDK build is
intentionally disabled; use Gradle or Build-Android.cmd. The tests use simulated
Google keys, not a live Google account. Live sign-in, token-expiry UX, phone
Keystore/biometrics, Tally and Windows Edge PDF rendering remain unverified.

The older verification record below applies to the previous Access preview;
its browser callback/Managed OAuth steps are not part of the new flow.

---

# Verification record — 25 September 2026

## Passed in the build environment

- Go unit/regression tests, including the existing Tally parsing, export-only
  gate, timeout/cooldown, bank persistence, ledger balances and PDF-template tests.
- Go race detector and go vet.
- Native identity tests: wrong email, wrong audience/issuer, expired/not-yet-valid
  tokens, and service identity rejected.
- Native phone tests: pending phones blocked from data; correct fingerprint
  approval; correct key accepted; wrong key, changed URI, replay and revoked key
  rejected; revocation persisted across reload.
- Listener isolation tests: no desktop, browser phone, legacy pairing, assets or
  admin endpoint reachable through the Android listener, including spoofed Host.
- Local setup tests reject non-loopback clients and cross-origin/proxy requests.
- Owner isolation test: a device approved on PC A cannot authenticate to PC B,
  even with the same simulated Google email and Access application.
- Owner-change test: prior phone approvals are revoked when owner settings change.
- Pair-code attempt limit and invalid configuration tests.
- Desktop JavaScript syntax plus jsdom checks: owner settings save, revocation
  notice, temporary code generation, matching fingerprint approval and saved bank.
- Java compiler parser accepted all native source files (syntax only).
- JVM protocol checks: HTTPS address validation, canonical request/body hash,
  P-256 signing and tamper rejection.
- Android manifest XML parsed; source references match backend endpoint inventory.
- Windows x64 GUI binaries cross-compiled. Installer's embedded main program and
  uninstaller exactly match the standalone outputs.

## Passed in GitHub Actions

[Run 2](https://github.com/vsshegur/TallyBridge/actions/runs/36102780940), source
commit `88d7107884c796e7b18c4ecfc7d215891e11a266`:
- Windows backend tests, vet and all three executable builds.
- Android SDK 35 compilation, D8, debug APK packaging/signing and lint.
- Both build artifacts downloaded; archive SHA-256 matched GitHub's artifact digests.
- Lint errors fixed using named layout constants; explicit backup/transfer exclusions
  added and failed preference clearing now reported. Nonblocking warnings remain.

## Not passed / still required

- Build-Android.cmd/ps1 and build-offline.sh were provided but not executed against
  a real Android SDK. They are reproducible build entry points, not proof of a
  successful Android build.
- No Android emulator/physical-device test, screenshot review, accessibility
  interaction test, biometric/Keystore test, or installed WhatsApp share test.
- No live Google/Cloudflare Managed OAuth registration, browser callback, session
  renewal or configured-domain test. Owner setup is required before that test.
- No Windows execution or live TallyPrime/Edge PDF test in this Linux environment.
  Existing PDF tests validate templates/data, not the Windows runtime here.

The implementation remains a preview until these release gates are completed.
Do not describe it as bug-free, production-ready, fully tested or a production release.
