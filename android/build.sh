#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
: "${ANDROID_HOME:?Set ANDROID_HOME to an Android SDK containing platform 35 and build-tools 35.0.0}"
if ! command -v gradle >/dev/null; then
  echo 'Install Gradle 8.11.1, or open this project in Android Studio.' >&2
  exit 1
fi
gradle --no-daemon assembleDebug lintDebug
mkdir -p output
cp app/build/outputs/apk/debug/app-debug.apk output/TallyBridge-Android-Preview.apk
