$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'

function Get-TallyBridgeSignTool {
    if ($env:TB_SIGNTOOL -and (Test-Path $env:TB_SIGNTOOL)) { return $env:TB_SIGNTOOL }
    $tool = Get-ChildItem "${env:ProgramFiles(x86)}\Windows Kits\10\bin\*\x64\signtool.exe" -ErrorAction SilentlyContinue |
        Sort-Object FullName -Descending | Select-Object -First 1
    if ($tool) { return $tool.FullName }
    return $null
}
function Sign-TallyBridgeFile([string]$Path) {
    if (!$env:TB_WINDOWS_SIGN_PFX) { return }
    if (!(Test-Path $env:TB_WINDOWS_SIGN_PFX)) { throw 'TB_WINDOWS_SIGN_PFX does not exist' }
    if (!$env:TB_WINDOWS_SIGN_PASSWORD) { throw 'TB_WINDOWS_SIGN_PASSWORD is required when signing is enabled' }
    $signtool = Get-TallyBridgeSignTool
    if (!$signtool) { throw 'signtool.exe was not found' }
    & $signtool sign /fd SHA256 /td SHA256 /tr http://timestamp.digicert.com /f $env:TB_WINDOWS_SIGN_PFX /p $env:TB_WINDOWS_SIGN_PASSWORD $Path
    if ($LASTEXITCODE -ne 0) { throw "Signing failed for $Path" }
    $status = (Get-AuthenticodeSignature $Path).Status
    if ($status -ne 'Valid') { throw "Authenticode verification failed for $Path : $status" }
}

function New-TallyBridgeStatusIcon([string]$Path, [string]$Color, [string]$Kind) {
    Add-Type -AssemblyName System.Drawing
    $bmp = New-Object System.Drawing.Bitmap 32,32
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
    $g.Clear([System.Drawing.Color]::Transparent)
    $brush = New-Object System.Drawing.SolidBrush ([System.Drawing.ColorTranslator]::FromHtml($Color))
    $pen = New-Object System.Drawing.Pen ([System.Drawing.Color]::White), 3
    $g.FillEllipse($brush, 2, 2, 28, 28)
    if ($Kind -eq 'ok') {
        $g.DrawLines($pen, [System.Drawing.Point[]]@(
            (New-Object System.Drawing.Point 9,17),
            (New-Object System.Drawing.Point 14,22),
            (New-Object System.Drawing.Point 24,11)
        ))
    } elseif ($Kind -eq 'warn') {
        $g.DrawLine($pen, 16, 8, 16, 19)
        $g.FillEllipse([System.Drawing.Brushes]::White, 14.5, 23, 3, 3)
    } else {
        $g.DrawLine($pen, 10, 10, 22, 22)
        $g.DrawLine($pen, 22, 10, 10, 22)
    }
    $hicon = $bmp.GetHicon()
    $icon = [System.Drawing.Icon]::FromHandle($hicon)
    $fs = [System.IO.File]::Open($Path, [System.IO.FileMode]::Create)
    try { $icon.Save($fs) } finally { $fs.Dispose(); $icon.Dispose(); $pen.Dispose(); $brush.Dispose(); $g.Dispose(); $bmp.Dispose() }
}

New-Item -ItemType Directory -Force build, cmd/setup/payload | Out-Null
New-TallyBridgeStatusIcon 'cmd/setup/payload/TallyBridge-green.ico' '#1CA772' 'ok'
New-TallyBridgeStatusIcon 'cmd/setup/payload/TallyBridge-yellow.ico' '#D6961F' 'warn'
New-TallyBridgeStatusIcon 'cmd/setup/payload/TallyBridge-red.ico' '#C63A3E' 'error'
foreach ($program in @(@('tallybridge','TallyBridge.exe'), @('uninstall','TallyBridge_Uninstall.exe'))) {
    $output = 'build/' + $program[1]
    & go build -buildvcs=false -trimpath '-ldflags=-s -w -H=windowsgui' -o $output ('./cmd/' + $program[0])
    if ($LASTEXITCODE -ne 0) { throw 'Windows compilation failed' }
    # Sign before copying into the installer payload so the installed binary is
    # the same signed binary that was verified here.
    Sign-TallyBridgeFile $output
    Copy-Item -Force $output ('cmd/setup/payload/' + $program[1])
}
& go build -buildvcs=false -trimpath '-ldflags=-s -w -H=windowsgui' -o build/TallyBridge_Setup.exe ./cmd/setup
if ($LASTEXITCODE -ne 0) { throw 'Installer compilation failed' }
# Sign the outer installer after it embeds the already-signed payload.
Sign-TallyBridgeFile 'build/TallyBridge_Setup.exe'
