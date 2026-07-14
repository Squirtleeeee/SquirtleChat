# 停止后台运行的 SquirtleChat（释放 8080 / 8081 / 5173）

$ErrorActionPreference = "SilentlyContinue"
$ports = 8080, 8081, 5173
foreach ($port in $ports) {
    Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue |
        ForEach-Object {
            $procId = $_.OwningProcess
            if ($procId) {
                Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
                Write-Host "Stopped pid $procId on port $port"
            }
        }
}
Write-Host "Done. Ports 8080 / 8081 / 5173 released."
