Set-Location $PSScriptRoot\..\deploy
docker compose up -d
Start-Sleep -Seconds 3
& "$PSScriptRoot\start-backend.ps1"
& "$PSScriptRoot\start-frontend.ps1"
Write-Host "All services started in background. Open http://localhost:5173"
Write-Host "Stop: .\scripts\stop-dev.ps1"
