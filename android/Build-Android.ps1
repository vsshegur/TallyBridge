# Downloads official build tools into this project's .tools directory.
# The project has no third-party Android app dependencies.
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$tools = Join-Path $PSScriptRoot '.tools'
New-Item -ItemType Directory -Force $tools | Out-Null
function Download([string]$Url,[string]$Destination) {
    if (-not (Test-Path $Destination)) {
        Write-Host "Downloading $(Split-Path $Destination -Leaf)..."
        Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile ($Destination + '.partial')
        Move-Item -Force ($Destination + '.partial') $Destination
    }
}
function Run-Checked([string]$Program,[string[]]$Arguments) {
    & $Program @Arguments
    if ($LASTEXITCODE -ne 0) { throw "Build command failed with exit code $LASTEXITCODE" }
}
# Prefer Android Studio's bundled JDK to keep downloads small.
$studioJdk = Join-Path $env:ProgramFiles 'Android\Android Studio\jbr'
if ($env:JAVA_HOME -and (Test-Path (Join-Path $env:JAVA_HOME 'bin\javac.exe'))) {
    $jdk = $env:JAVA_HOME
} elseif (Test-Path (Join-Path $studioJdk 'bin\javac.exe')) {
    $jdk = $studioJdk
} else {
    $jdkDir = Join-Path $tools 'jdk'
    if (-not (Test-Path $jdkDir)) {
        $jdkZip = Join-Path $tools 'jdk17.zip'
        Download 'https://api.adoptium.net/v3/binary/latest/17/ga/windows/x64/jdk/hotspot/normal/eclipse' $jdkZip
        Expand-Archive -Force $jdkZip $jdkDir
    }
    $jdk = (Get-ChildItem $jdkDir -Directory | Select-Object -First 1).FullName
}
$env:JAVA_HOME = $jdk
$env:Path = (Join-Path $jdk 'bin') + ';' + $env:Path
$sdk = Join-Path $tools 'android-sdk'
$manager = Join-Path $sdk 'cmdline-tools\latest\bin\sdkmanager.bat'
if (-not (Test-Path $manager)) {
    $sdkZip = Join-Path $tools 'android-commandline.zip'
    Download 'https://dl.google.com/android/repository/commandlinetools-win-13114758_latest.zip' $sdkZip
    $extract = Join-Path $tools 'commandline-extract'
    Expand-Archive -Force $sdkZip $extract
    New-Item -ItemType Directory -Force (Join-Path $sdk 'cmdline-tools') | Out-Null
    Move-Item -Force (Join-Path $extract 'cmdline-tools') (Join-Path $sdk 'cmdline-tools\latest')
}
$env:ANDROID_HOME = $sdk
Write-Host 'Review and accept the Android SDK licence prompts to continue.'
Run-Checked -Program $manager -Arguments @("--sdk_root=$sdk", '--licenses')
Run-Checked -Program $manager -Arguments @("--sdk_root=$sdk", 'platforms;android-35', 'build-tools;35.0.0')
$gradleZip = Join-Path $tools 'gradle-8.11.1-bin.zip'
$gradle = Join-Path $tools 'gradle-8.11.1\bin\gradle.bat'
if (-not (Test-Path $gradle)) {
    Download 'https://services.gradle.org/distributions/gradle-8.11.1-bin.zip' $gradleZip
    $sha = (Invoke-WebRequest -UseBasicParsing 'https://services.gradle.org/distributions/gradle-8.11.1-bin.zip.sha256').Content.Trim()
    if ((Get-FileHash $gradleZip -Algorithm SHA256).Hash.ToLowerInvariant() -ne $sha.ToLowerInvariant()) { throw 'Gradle checksum mismatch' }
    Expand-Archive -Force $gradleZip $tools
}
Run-Checked -Program $gradle -Arguments @('--no-daemon', 'assembleDebug', 'lintDebug')
New-Item -ItemType Directory -Force (Join-Path $PSScriptRoot 'output') | Out-Null
Copy-Item -Force (Join-Path $PSScriptRoot 'app\build\outputs\apk\debug\app-debug.apk') (Join-Path $PSScriptRoot 'output\TallyBridge-Android-Preview.apk')
Write-Host ''
Write-Host 'BUILD SUCCEEDED: output\TallyBridge-Android-Preview.apk' -ForegroundColor Green
Write-Host 'This is a debug-signed preview. Test login, pairing and every report before replacing your existing app.'
Write-Host 'Keep the debug signing key on this PC if you need compatible preview updates. Use a private release keystore for production.'
Start-Process explorer.exe (Join-Path $PSScriptRoot 'output')
