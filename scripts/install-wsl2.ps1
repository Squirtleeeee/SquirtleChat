# SquirtleChat - WSL2 安装脚本
# 右键 -> 使用 PowerShell 运行（或以管理员打开 PowerShell 后执行）

#Requires -RunAsAdministrator

$ErrorActionPreference = "Stop"

Write-Host "=== 安装 WSL2 ===" -ForegroundColor Cyan

# 1. 启用 Windows 功能
Write-Host "[1/4] 启用 WSL 与虚拟机平台..."
dism.exe /online /enable-feature /featurename:Microsoft-Windows-Subsystem-Linux /all /norestart | Out-Null
dism.exe /online /enable-feature /featurename:VirtualMachinePlatform /all /norestart | Out-Null

# 2. 用 winget 安装/更新 WSL 内核（失败则回退 wsl --install）
Write-Host "[2/4] 安装 WSL 组件..."
$winget = Get-Command winget -ErrorAction SilentlyContinue
if ($winget) {
    winget install --id Microsoft.WSL -e --accept-source-agreements --accept-package-agreements
} else {
    wsl --install --no-launch
}

# 3. 默认 WSL2
Write-Host "[3/4] 设置 WSL2 为默认..."
wsl --set-default-version 2 2>$null

# 4. 可选：安装 Ubuntu（不需要可注释掉下一行）
Write-Host "[4/4] 安装 Ubuntu 发行版（可选）..."
wsl --install -d Ubuntu --no-launch 2>$null

Write-Host ""
Write-Host "完成。请重启电脑，然后运行: wsl" -ForegroundColor Green
Write-Host "验证: wsl -l -v" -ForegroundColor Green
Read-Host "按 Enter 退出"
