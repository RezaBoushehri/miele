# Build the Windows deliverables into dist\ (run on Windows with Go installed).
#
#   .\build.ps1            normal build
#   .\build.ps1 -Garble    obfuscated build (needs `garble`)
param([switch]$Garble)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot
$env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
New-Item -ItemType Directory -Force -Path dist | Out-Null

$ldGui = "-s -w -H windowsgui"
$ldCli = "-s -w"

if ($Garble) {
  Write-Host ">> obfuscated build (garble)"
  garble -tiny -literals build -trimpath -ldflags $ldGui -o dist\miele.exe .\cmd\miele
  garble -tiny -literals build -trimpath -ldflags $ldCli -o dist\keygen.exe .\cmd\keygen
} else {
  Write-Host ">> plain build"
  go build -trimpath -ldflags $ldGui -o dist\miele.exe .\cmd\miele
  go build -trimpath -ldflags $ldCli -o dist\keygen.exe .\cmd\keygen
}

Copy-Item cmd\miele\app.manifest dist\miele.exe.manifest -Force
Copy-Item examples\kitchen.csv dist\parts-template.csv -Force -ErrorAction SilentlyContinue

Write-Host "`nbuilt:"; Get-ChildItem dist | Format-Table Name, Length
$pub = Select-String -Path internal\license\pubkey.go -Pattern 'PublicKeyHex = "[0-9a-f]'
if (-not $pub) {
  Write-Warning "این build با کلید مجوز provision نشده. اول 'keygen.exe init' را اجرا و دوباره build کنید."
}
