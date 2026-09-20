# PureSend Installer for Windows (PowerShell)
# Usage: irm https://raw.githubusercontent.com/Baaaki/PureSend/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

$repo = "Baaaki/PureSend"
$binary = "puresend.exe"

Write-Host "==> PureSend Windows kurulumu baslatiliyor..." -ForegroundColor Cyan

# 1. Check latest version from GitHub API
try {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest"
    $tag = $release.tag_name
} catch {
    $tag = "v1.0.0"
}

$version = $tag.TrimStart("v")
$zipName = "puresend_${version}_windows_x86_64.zip"
$downloadUrl = "https://github.com/$repo/releases/download/$tag/$zipName"

$installDir = "$env:LOCALAPPDATA\PureSend"
$zipPath = "$env:TEMP\$zipName"

Write-Host "==> En son surum indiriliyor: $tag..." -ForegroundColor Cyan
Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath

if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

Write-Host "==> Dosyalar cikariliyor..." -ForegroundColor Cyan
Expand-Archive -Path $zipPath -DestinationPath $installDir -Force
Remove-Item -Path $zipPath -Force

# Add to user PATH if not already present
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    Write-Host "==> PATH ortamina ekleniyor: $installDir" -ForegroundColor Cyan
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", "User")
    $env:Path += ";$installDir"
}

Write-Host "==> PureSend ($tag) basariyla kuruldu!" -ForegroundColor Green
Write-Host ""
Write-Host "Kullanim:"
Write-Host "  Dosya gondermek icin: puresend send <dosya_veya_klasor>"
Write-Host "  Dosya almak icin:     puresend receive <kod>"
Write-Host ""
