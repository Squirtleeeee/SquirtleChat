# 本地 MySQL 初始化（无 Docker 时用）
# 用法: .\scripts\setup-local-mysql.ps1 -RootPassword "你的root密码"

param(
    [Parameter(Mandatory=$true)]
    [string]$RootPassword
)

$schema = Join-Path $PSScriptRoot "..\deploy\init\mysql\001_schema.sql"

mysql -u root -p$RootPassword -e @"
CREATE DATABASE IF NOT EXISTS squirtlechat CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'squirtle'@'localhost' IDENTIFIED BY 'squirtle123';
GRANT ALL PRIVILEGES ON squirtlechat.* TO 'squirtle'@'localhost';
FLUSH PRIVILEGES;
"@

Get-Content $schema -Raw | mysql -u squirtle -psquirtle123 squirtlechat
Write-Host "DB ready: squirtle / squirtle123 @ squirtlechat"
