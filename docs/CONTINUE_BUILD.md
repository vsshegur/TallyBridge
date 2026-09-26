> **Legacy 2.0 Access preview document.** For 2.0.1 direct Google sign-in without a Cloudflare payment card, follow [NO_CARD_SETUP.md](NO_CARD_SETUP.md). The Access/team/AUD instructions below do not apply to the new setup.

# Continue this build

User requirement: distribute the same tool to any owner. Owner chooses one
allowed Google email on their PC. Android must use that verified same email,
then authenticate with a code from that PC and desktop device approval. All
accounting data stays in each independent PC's local store. Cloudflare processes
proxied requests; no shared central accounting database exists. Web UI is desktop
setup only. Android must be truly native and cover every former phone feature.

Implemented in this tree:
- native Java Android UI and all mapped reports; see FEATURE_PARITY.md;
- Cloudflare Managed OAuth public client with state/PKCE and Android loopback
  callback (requires dynamic registration + Allow loopback clients in Access);
- Keystore P-256 request signatures and AES-GCM login token storage;
- server JWT verification and exact allowed-email/audience/issuer enforcement;
- single-use PC code, pending approval fingerprint, server-side revocation;
- owner/connection changes revoke old phones; independent-owner regression test;
- setup-only listener :8765 and strict mobile-only listener :8766;
- preserved Tally gate/cache/PDF/bank behavior;
- Windows preview binaries built and checked.

GitHub repository: https://github.com/vsshegur/TallyBridge
Build 2 passed both platforms: https://github.com/vsshegur/TallyBridge/actions/runs/36102780940
Android and Windows artifacts are available. Detailed owner steps are in
CLOUDFLARE_SETUP.md. The local SDK remains unavailable, but CI builds successfully.

Next required work:
1. Establish a stable private release signing key before production distribution.
2. Test Google/Access login on an owner's actual configured domain. Check dynamic
   registration, PKCE, token refresh and expiry. No service secret belongs in APK.
3. Test native UI on Android, including system back, background lock, lifecycle,
   slow/cancelled requests, pairing/reset, company changes, native PDFs and share.
4. Test against real Windows Tally/Edge, compare accounting results and defaults.
5. Only then release/install as the replacement for the existing 1.6 phone app.

Original 1.6 source and release remain separate in the workspace. The new preview
installer uses the existing TallyBridge app directory, preserving settings and
cache but replacing its executable. Do not install it until Android is ready.
No live settings, credentials, Tally cache or signing keys are in this package.
