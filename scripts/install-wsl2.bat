@echo off
chcp 65001 >nul
title SquirtleChat - WSL2 安装

:: 检查管理员权限，没有则请求提升
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo 正在请求管理员权限，请在 UAC 窗口点击「是」...
    powershell -NoProfile -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
    exit /b
)

echo ========================================
echo   SquirtleChat WSL2 安装
echo ========================================
echo.

echo [1/4] 启用 WSL 功能...
dism.exe /online /enable-feature /featurename:Microsoft-Windows-Subsystem-Linux /all /norestart
dism.exe /online /enable-feature /featurename:VirtualMachinePlatform /all /norestart

echo.
echo [2/4] 安装 WSL（可能需要几分钟）...
where winget >nul 2>&1
if %errorlevel% equ 0 (
    winget install --id Microsoft.WSL -e --accept-source-agreements --accept-package-agreements
) else (
    wsl --install --no-launch
)

echo.
echo [3/4] 设置 WSL2 为默认...
wsl --set-default-version 2

echo.
echo [4/4] 安装 Ubuntu（可选）...
wsl --install -d Ubuntu --no-launch 2>nul

echo.
echo ========================================
echo   安装完成！请重启电脑。
echo   重启后运行: wsl -l -v
echo ========================================
echo.
pause
