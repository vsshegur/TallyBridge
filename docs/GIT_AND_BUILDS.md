# Git and automatic builds

This project is published at https://github.com/vsshegur/TallyBridge on branch `main`.
Windows and Android builds passed in Actions run 2. Its source
includes a GitHub Actions workflow that builds Windows executables and a native
Android preview APK when pushed to `main`, or when Run workflow is selected.

## Restore the provided Git bundle

With Git installed:

```
git clone TallyBridge.bundle TallyBridge
cd TallyBridge
```

The bundle contains committed source and history, not private owner settings,
Tally data, generated executables, Android signing keys or SDK tools.

## Publish a new private GitHub repository

Create an empty private repository named TallyBridge in your own GitHub account.
Do not initialize it with a README if pushing the bundle's existing history.
Then follow GitHub's displayed commands to add the repository as `origin` and
push `main`. GitHub Desktop can also add the restored local repository and
publish it privately. Never upload your `%LOCALAPPDATA%\TallyBridge` folder.

## Download CI build outputs

After the source is pushed, open the repository's Actions tab. Select
“Build Windows and Android preview”, then open its latest successful run.
Under Artifacts download:

- TallyBridge-Windows-Preview: Windows installer, main executable, uninstaller.
- TallyBridge-Android-Preview: native Android debug-signed APK.
- Android-Lint-Report: lint results for review.

If either job is red, open its failed step and resolve the error before using the
build. A green compile does not replace real phone, Cloudflare and Tally testing.

The CI APK is a debug-signed preview with a runner-generated key, so APKs from
separate runs may have different signing keys. Do not distribute it as a final
production release. A stable private release keystore is needed for reliable
updates; keep it outside Git and configure repository secrets for a release
workflow. Never publish the key or embed Cloudflare service credentials in an APK.
