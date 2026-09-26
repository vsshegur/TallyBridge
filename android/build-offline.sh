#!/usr/bin/env bash
set -euo pipefail
echo "Direct Google sign-in requires AndroidX and Google libraries. Use ./build.sh with Gradle or Build-Android.cmd on Windows." >&2
exit 1
