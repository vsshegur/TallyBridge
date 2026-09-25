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

Blocked: no Android SDK/platform or build tools in the build environment; Google
SDK/Maven download hosts time out. Do not pretend Java syntax parsing constitutes
an Android build. No APK was generated. SDK platform 35 and Linux build-tools
35.0.0 are sufficient to try build-offline.sh (no Gradle dependencies required).
A JDK17 runtime with jdk.compiler exists; `java -m jdk.compiler/com.sun.tools.javac.Main`
works even though the javac launcher executable is missing. For normal builds,
use Android Studio or the included Windows bootstrap helper.

Next required work:
1. Obtain authorized SDK tools in the build environment, compile, fix API errors,
   run lint and sign a preview APK. Keep signing keys private and persistent.
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
