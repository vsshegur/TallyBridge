#!/usr/bin/env bash
# Builds the dependency-free native app using only JDK 17 and Android SDK 35.
set -euo pipefail
cd "$(dirname "$0")"
: "${ANDROID_HOME:?Set ANDROID_HOME to an SDK with platforms/android-35 and build-tools/35.0.0}"
TB_BUILD_TOOLS="$ANDROID_HOME/build-tools/35.0.0"
TB_ANDROID_JAR="$ANDROID_HOME/platforms/android-35/android.jar"
TB_OUT="$PWD/.offline-build"
mkdir -p "$TB_OUT/resources" "$TB_OUT/generated" "$TB_OUT/classes" "$TB_OUT/dex" output
"$TB_BUILD_TOOLS/aapt2" compile --dir app/src/main/res -o "$TB_OUT/resources.zip"
sed 's/<manifest /<manifest package="in.tallybridge.app" /' app/src/main/AndroidManifest.xml > "$TB_OUT/AndroidManifest.xml"
"$TB_BUILD_TOOLS/aapt2" link -o "$TB_OUT/base.apk" -I "$TB_ANDROID_JAR" \
 --manifest "$TB_OUT/AndroidManifest.xml" --java "$TB_OUT/generated" \
 --min-sdk-version 28 --target-sdk-version 35 --version-code 20000 --version-name 2.0.0-preview "$TB_OUT/resources.zip"
find app/src/main/java "$TB_OUT/generated" -name '*.java' > "$TB_OUT/sources.txt"
java -m jdk.compiler/com.sun.tools.javac.Main -encoding UTF-8 -source 17 -target 17 -classpath "$TB_ANDROID_JAR" -d "$TB_OUT/classes" @"$TB_OUT/sources.txt"
find "$TB_OUT/classes" -name '*.class' > "$TB_OUT/classes.txt"
"$TB_BUILD_TOOLS/d8" --lib "$TB_ANDROID_JAR" --min-api 28 --output "$TB_OUT/dex" @"$TB_OUT/classes.txt"
cp "$TB_OUT/base.apk" "$TB_OUT/unsigned.apk"
(cd "$TB_OUT/dex" && zip -q "$TB_OUT/unsigned.apk" classes*.dex)
"$TB_BUILD_TOOLS/zipalign" -f 4 "$TB_OUT/unsigned.apk" "$TB_OUT/aligned.apk"
TB_PREVIEW_KEY="$TB_OUT/preview.jks"
if [[ ! -f "$TB_PREVIEW_KEY" ]]; then
 keytool -genkeypair -keystore "$TB_PREVIEW_KEY" -storepass android -keypass android -alias androiddebugkey -keyalg RSA -keysize 2048 -validity 10000 -dname 'CN=TallyBridge Preview,O=TallyBridge,C=IN'
fi
"$TB_BUILD_TOOLS/apksigner" sign --ks "$TB_PREVIEW_KEY" --ks-pass pass:android --ks-key-alias androiddebugkey --key-pass pass:android --out output/TallyBridge-Android-Preview.apk "$TB_OUT/aligned.apk"
"$TB_BUILD_TOOLS/apksigner" verify --verbose output/TallyBridge-Android-Preview.apk
printf '%s\n' 'Built output/TallyBridge-Android-Preview.apk. Keep .offline-build/preview.jks private for compatible preview updates.'
