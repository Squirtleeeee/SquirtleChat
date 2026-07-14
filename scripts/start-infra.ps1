Set-Location $PSScriptRoot\..\deploy
docker compose up -d
Write-Host "Infra started. Run gateway-http, gateway-ws, frontend separately."
