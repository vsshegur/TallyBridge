# TallyBridge 2.0 — start here

## What is ready

The Windows x64 installer and committed Git source are included. Android source
and automated build scripts are included. **An APK has not yet been compiled.**
This is a preview, not a verified final release. Keep your working version until
the Android build and real PC/phone checks below pass.

## Files

- `TallyBridge_Setup.exe`: installer; normally this is the only EXE you run.
- `TallyBridge.exe`: standalone bridge executable, also installed by Setup.
- `TallyBridge_Uninstall.exe`: removes the installed program.
- `TallyBridge.bundle`: complete committed source repository, portable Git backup.
- `TallyBridge_v2.0.0_Native_Source.zip`: source for browsing or building.
- `SHA256SUMS.txt`: integrity checks for supplied files.

No owner data, Google credentials, Cloudflare tunnel secrets, or signing keys are
included. Do not share your installed data folder with other users.

## Get the APK on your Windows computer

1. Extract the source ZIP to a normal folder, for example Documents\TallyBridge.
2. Open its `android` folder and double-click `Build-Android.cmd`.
3. The build needs Internet access. It downloads Java, Android SDK and Gradle
   from their official distributors. Read and accept SDK licenses if you agree.
4. Wait for the build to finish successfully. The APK is produced at
   `android\app\build\outputs\apk\debug\app-debug.apk`.
5. If it fails, keep the error output. Do not treat a failed build as ready.
6. Copy the APK to your phone and open it to install. Android may request
   permission to install APKs from the file manager you used.

This is a debug preview. Keep its local build/signing files private; a stable
release signing key is required before public distribution and reliable updates.

## Put the project on GitHub and build there instead

1. Install Git, then open a terminal in the folder containing the bundle.
2. Run `git clone TallyBridge.bundle TallyBridge`.
3. On GitHub create a new **private**, empty repository called `TallyBridge`.
   Do not add a README, license or .gitignore during creation.
4. In the terminal run `cd TallyBridge` and the commands below, replacing
   YOUR_USERNAME with your GitHub username:

```
git remote add origin https://github.com/YOUR_USERNAME/TallyBridge.git
git push -u origin main
```

5. Complete Git's sign-in when asked. Never paste tokens or passwords into chat.
6. Open your repository's Actions tab, then the latest
   `Build Windows and Android preview` run.
7. When both jobs succeed, download `TallyBridge-Windows-Preview` and
   `TallyBridge-Android-Preview` under Artifacts. Extract each downloaded ZIP.
8. If a job fails, open the red step to see the build error.

GitHub Actions APKs use temporary debug signing keys. For repeated installations
from separate runs you may need to uninstall the old preview and pair again.
A release-signing workflow is still required for production distribution.

## Configure your PC, domain and phone

Follow `docs/SETUP.md` in the source ZIP for the full instructions. The order is:

1. Back up `%LOCALAPPDATA%\TallyBridge` if upgrading.
2. With an APK ready, install the Windows preview. Open TallyPrime and your company.
3. Open `http://127.0.0.1:8765/admin` on that PC. Save the default PDF bank for
   each company once; Android uses that saved selection.
4. Create a Cloudflare Tunnel to `http://127.0.0.1:8766` for your subdomain.
   Do not expose port 8765 or Tally's port 9000.
5. Protect the hostname with Cloudflare Access, Google login, and an Allow rule
   for exactly your chosen Google email. Enable Managed OAuth, dynamic client
   registration and Allow loopback clients.
6. Save the hostname, Access team domain, application AUD and matching email in
   desktop setup's Cloudflare page. These fields do not create Cloudflare resources.
7. On Android enter the hostname, sign in with the same Google email, and enter
   the temporary code generated on the desktop's Android Phones page.
8. Compare the device fingerprint on both screens, then approve it on the PC.
9. Compare reports/PDFs with Tally, test sharing and revoke/re-pair a test phone.
   Keep the PC and its Cloudflare connector running for remote access.

Each person who uses the tool sets up their own PC, hostname and email. Their
installation keeps its own data and device approvals. Matching email alone does
not grant access: the PC code, device key and explicit approval are also required.

## Verification status

Backend tests, race checks, Go vet, desktop DOM checks and Java protocol checks
passed during development. Android compilation/lint/device use, live Google and
Cloudflare login, and Windows/Tally/PDF integration remain unverified. No software
can be guaranteed bug-free; resolve these checks before everyday use.
