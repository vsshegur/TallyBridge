# TallyBridge production signing

TallyBridge 2.1 supports permanent Android release signing and optional trusted Windows Authenticode signing without storing private keys in the repository.

## Android — one permanent identity

Create one Android release keystore and keep an offline backup. Add these repository Actions secrets:

- `ANDROID_KEYSTORE_BASE64` — base64 of the complete keystore file.
- `ANDROID_KEYSTORE_PASSWORD`
- `ANDROID_KEY_ALIAS`
- `ANDROID_KEY_PASSWORD`

The signed release workflow restores the keystore only inside the temporary GitHub Actions runner, builds `app-release.apk`, verifies its signature, records SHA-1/SHA-256 certificate fingerprints, and deletes the runner after the job.

The SHA-1 from this permanent keystore is the one that should be registered in the Google OAuth Android client. Do not use a debug-runner SHA for production.

## Windows — trusted code signing

For Windows SmartScreen/trust improvements, obtain a real code-signing certificate from a trusted CA or signing service. If using a PFX, add:

- `WINDOWS_PFX_BASE64`
- `WINDOWS_PFX_PASSWORD`

When configured, the release workflow signs every shipped EXE with SHA-256 and a public timestamp, then verifies the Authenticode signature before publishing.

If these Windows secrets are absent, the workflow still builds Windows artifacts but marks them clearly as unsigned. A self-signed certificate is intentionally not used because it does not provide normal Windows trust.

## Publishing

Push a version tag such as `v2.1.0` after the normal branch build is green. The signed workflow builds both platforms, emits SHA-256 checksums, and publishes a GitHub Release only after the Android signed build and Windows build jobs succeed.

Private signing credentials must never be committed to Git.
