# 一键后台启动：后端 + 前端（无 PowerShell 弹窗）
# 用法: .\scripts\start-dev.ps1
# 停止: .\scripts\stop-dev.ps1

$ErrorActionPreference = "Stop"

& "$PSScriptRoot\start-backend.ps1"
& "$PSScriptRoot\start-frontend.ps1"

Write-Host ""
Write-Host "SquirtleChat is up:"
Write-Host "  App      http://localhost:5173"
Write-Host "  API      http://localhost:8080"
Write-Host "  WS       :8081"
Write-Host "Stop with: .\scripts\stop-dev.ps1"
