# 后台静默启动前端 Vite（:5173），不弹出窗口
# 日志：logs/frontend.*.log

param(
    [switch]$Visible
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..")
$frontend = Join-Path $root "frontend"
$logs = Join-Path $root "logs"
New-Item -ItemType Directory -Force -Path $logs | Out-Null

Get-NetTCPConnection -LocalPort 5173 -ErrorAction SilentlyContinue |
    ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }
Start-Sleep -Seconds 1

$windowStyle = if ($Visible) { "Normal" } else { "Hidden" }
$outLog = Join-Path $logs "frontend.out.log"
$errLog = Join-Path $logs "frontend.err.log"
"" | Set-Content -Path $outLog -Encoding UTF8
"" | Set-Content -Path $errLog -Encoding UTF8

# npm.cmd 在 Windows 上更可靠
$npmCmd = Get-Command npm.cmd -ErrorAction SilentlyContinue
if ($npmCmd) { $npm = $npmCmd.Source } else { $npm = "npm.cmd" }

$p = Start-Process -FilePath $npm `
    -ArgumentList @("run", "dev", "--", "--host", "127.0.0.1", "--port", "5173") `
    -WorkingDirectory $frontend `
    -WindowStyle $windowStyle `
    -RedirectStandardOutput $outLog `
    -RedirectStandardError $errLog `
    -PassThru

$ready = $false
for ($i = 0; $i -lt 40; $i++) {
    try {
        $r = Invoke-WebRequest "http://127.0.0.1:5173" -UseBasicParsing -TimeoutSec 1
        if ($r.StatusCode -eq 200) { $ready = $true; break }
    } catch {
        Start-Sleep -Milliseconds 500
    }
}

if ($ready) {
    Write-Host "Frontend running in background: http://localhost:5173  pid=$($p.Id)"
    Write-Host "  log=logs/frontend.*.log"
} else {
    Write-Host "Frontend started but not ready yet. Check logs/frontend.err.log"
}
