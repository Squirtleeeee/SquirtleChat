# 双 gateway 实例（分布式 WS 测试）
$env:GATEWAY_INSTANCE_ID = "gw-1"
$env:WS_PORT = "8081"
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$PSScriptRoot\..\backend'; `$env:GATEWAY_INSTANCE_ID='gw-1'; `$env:WS_PORT='8081'; go run ./services/gateway-ws"

$env:GATEWAY_INSTANCE_ID = "gw-2"
$env:WS_PORT = "8082"
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$PSScriptRoot\..\backend'; `$env:GATEWAY_INSTANCE_ID='gw-2'; `$env:WS_PORT='8082'; go run ./services/gateway-ws"

Write-Host "gw-1 :8081, gw-2 :8082 started. Point clients to different WS ports."
