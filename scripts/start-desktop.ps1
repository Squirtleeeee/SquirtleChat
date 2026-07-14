# 启动桌面版 SquirtleChat（后端 + Vite + Electron）
# 用法: .\scripts\start-desktop.ps1

param([switch]$SkipBackend)

$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..")

if (-not $SkipBackend) {
  & "$PSScriptRoot\start-backend.ps1"
}

$feUp = $false
try {
  $r = Invoke-WebRequest "http://127.0.0.1:5173" -UseBasicParsing -TimeoutSec 2
  if ($r.StatusCode -eq 200) { $feUp = $true }
} catch {}

if (-not $feUp) {
  & "$PSScriptRoot\start-frontend.ps1"
}

$desktop = Join-Path $root "desktop"
if (-not (Test-Path (Join-Path $desktop "node_modules\electron"))) {
  Write-Host "Installing Electron..."
  Push-Location $desktop
  npm install
  Pop-Location
}

Write-Host "Launching Electron desktop app..."
Start-Process -FilePath "npm.cmd" -ArgumentList @("run", "dev") -WorkingDirectory $desktop -WindowStyle Hidden
Write-Host "Desktop app started."
Write-Host "Open Settings to switch layout: embedded / detached chat windows."
Write-Host "Stop API/Vite with .\scripts\stop-dev.ps1 then Quit from tray."
