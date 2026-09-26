# Windows 2.0.5 — PDF browser fallback

Install TallyBridge_Setup.exe over the existing Windows installation. Keep Android 2.0.4, its Google OAuth settings and existing pairing. No APK or SHA-1 change is needed.

The renderer now tries installed Chrome and Edge automatically instead of stopping at the first detected browser. Each attempt has an isolated temporary profile. It waits for a completed PDF, including when a Windows browser launcher exits before its child writes the file. It rejects missing or incomplete PDFs and limits total rendering time.

Tests cover delayed child output, missing files and complete output despite browser shutdown errors. Windows CI also generates an actual PDF and repeats the test with a first browser executable that exits successfully without writing a PDF, proving fallback to the real browser.

If no installed browser succeeds, the phone shows which executable failed. Install or update Chrome on the PC and retry. This cannot override Windows/browser enterprise policies that prohibit headless printing. No Tally data is modified.
