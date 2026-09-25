$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
New-Item -ItemType Directory -Force build, cmd/setup/payload | Out-Null
foreach ($program in @(@('tallybridge','TallyBridge.exe'), @('uninstall','TallyBridge_Uninstall.exe'))) {
    & go build -buildvcs=false -trimpath '-ldflags=-s -w -H=windowsgui' -o ('build/' + $program[1]) ('./cmd/' + $program[0])
    if ($LASTEXITCODE -ne 0) { throw 'Windows compilation failed' }
    Copy-Item -Force ('build/' + $program[1]) ('cmd/setup/payload/' + $program[1])
}
& go build -buildvcs=false -trimpath '-ldflags=-s -w -H=windowsgui' -o build/TallyBridge_Setup.exe ./cmd/setup
if ($LASTEXITCODE -ne 0) { throw 'Installer compilation failed' }
