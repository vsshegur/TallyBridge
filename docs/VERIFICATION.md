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
