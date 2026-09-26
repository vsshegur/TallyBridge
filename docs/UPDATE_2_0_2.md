# 2.0.2: Android design and Windows startup

## Changes

- Native Android dashboard with a prominent receivable card, separate payable and sales cards, and quick actions.
- Consistent typography, spacing, rounded report cards, search fields, company selector and vector navigation icons.
- Scrollable setup/approval forms for small screens and keyboard use.
- Dashboard background refresh retains its scroll position.
- Windows installer registers TallyBridge for automatic startup for the installing Windows user.
- Automatic launches use --background: the service runs without opening the setup browser. Opening the desktop shortcut still opens setup.
- Uninstall removes the startup registration, whether you keep or remove saved data.

Automatic startup happens after Windows sign-in, not before it. Cloudflared must also be configured to start automatically. TallyPrime and the company must be open for live reports; this installer does not launch or configure TallyPrime.

## Install

Back up the existing installation, close TallyBridge and run the updated Setup EXE.
Settings, bank defaults, cached reports and existing desktop phone approvals are kept.
Windows Task Manager > Startup apps can enable/disable the TallyBridge startup entry.

The preview Android APK uses a CI-generated signing key. If Android cannot update
the old APK in place, uninstall it and install the new APK, add a Google
Android OAuth client in the SAME Google project with package in.tallybridge.app
and the SHA1 from the new signing report, and pair again. Your Web
client ID, allowed email and Cloudflare route remain the same.

## Verification

Backend regression tests run locally and in Windows CI. Android compilation and
lint run in CI. The visual test uses synthetic company data on an emulator to
render the lock, dashboard, reports and connection screens; it does not test
live Google sign-in or Tally. A separate instrumentation test APK changes fixture
state; no demo mode or authentication bypass is included in the shipped APK.

The prior version's Google/Tally workflow was reported working by the owner.
Authentication and report request logic are unchanged by this design update.
The emulator screen test passed on Android 11 in run 11.
A reboot/sign-in check on the owner's Windows PC is still needed.

## Downloads and Google fingerprint

Build: https://github.com/vsshegur/TallyBridge/actions/runs/36229848684

For this build, create an additional Android OAuth client in your existing Google
project with package `in.tallybridge.app` and this SHA1:

```text
5B:82:65:F2:5A:D3:F6:43:2D:B2:2F:49:1C:07:85:A3:B7:59:41:90
```

Keep the same Web client ID and Cloudflare settings. The previous Android client
can remain registered for your previous APK.
