# 后台静默启动后端（HTTP :8080 + WS :8081），不弹出 PowerShell 窗口
# 日志：logs/gateway-http.log、logs/gateway-ws.log
# 智能体：复制 deploy/llm.env.example → deploy/llm.env 并填入 LLM_API_KEY

param(
    [switch]$Visible  # 需要看控制台时：.\scripts\start-backend.ps1 -Visible
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..")
$backend = Join-Path $root "backend"
$llmEnv = Join-Path $root "deploy\llm.env"
$logs = Join-Path $root "logs"
$kafka = "localhost:29092"

New-Item -ItemType Directory -Force -Path $logs | Out-Null

# 加载 LLM 环境到当前进程（子进程会继承）
if (Test-Path $llmEnv) {
    Write-Host "Loading LLM config from deploy/llm.env"
    Get-Content $llmEnv | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq '' -or $line.StartsWith('#')) { return }
        $i = $line.IndexOf('=')
        if ($i -lt 1) { return }
        $k = $line.Substring(0, $i).Trim()
        $v = $line.Substring($i + 1).Trim()
        if ($v.StartsWith('"') -and $v.EndsWith('"')) { $v = $v.Substring(1, $v.Length - 2) }
        [Environment]::SetEnvironmentVariable($k, $v, 'Process')
    }
} else {
    Write-Host "Tip: copy deploy/llm.env.example to deploy/llm.env for AI assistant"
}

$env:KAFKA_BROKER = $kafka

foreach ($port in 8080, 8081) {
    Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue |
        ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }
}
Start-Sleep -Seconds 1

$windowStyle = if ($Visible) { "Normal" } else { "Hidden" }

function Start-BackgroundGo {
    param([string]$ServiceRel, [string]$LogName)
    $outLog = Join-Path $logs "$LogName.out.log"
    $errLog = Join-Path $logs "$LogName.err.log"
    # 清空旧日志
    "" | Set-Content -Path $outLog -Encoding UTF8
    "" | Set-Content -Path $errLog -Encoding UTF8
    $p = Start-Process -FilePath "go" `
        -ArgumentList @("run", $ServiceRel) `
        -WorkingDirectory $backend `
        -WindowStyle $windowStyle `
        -RedirectStandardOutput $outLog `
        -RedirectStandardError $errLog `
        -PassThru
    return $p
}

$httpProc = Start-BackgroundGo -ServiceRel "./services/gateway-http" -LogName "gateway-http"
Start-Sleep -Seconds 2
$wsProc = Start-BackgroundGo -ServiceRel "./services/gateway-ws" -LogName "gateway-ws"

# 等待 HTTP 就绪
$ready = $false
for ($i = 0; $i -lt 30; $i++) {
    try {
        $null = Invoke-RestMethod "http://localhost:8080/health" -TimeoutSec 1
        $ready = $true
        break
    } catch {
        Start-Sleep -Milliseconds 500
    }
}

if ($ready) {
    Write-Host "Backend running in background (no console windows)."
    Write-Host "  HTTP  :8080  pid=$($httpProc.Id)  log=logs/gateway-http.*.log"
    Write-Host "  WS    :8081  pid=$($wsProc.Id)  log=logs/gateway-ws.*.log"
    if ($env:LLM_API_KEY) { Write-Host "  LLM: configured" } else { Write-Host "  LLM: not configured" }
    Write-Host "Stop: .\scripts\stop-dev.ps1"
} else {
    Write-Host "Backend started but health check not ready yet. Check logs/gateway-http.err.log"
}
