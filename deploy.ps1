<#
.SYNOPSIS
    Deploy a new ItemCostTracker image from a tar.gz archive.

.DESCRIPTION
    Stops the running container, loads the new image from itemcosttracker.tar.gz,
    and starts the container again.

.PARAMETER ImageFile
    Path to the image archive. Defaults to .\itemcosttracker.tar.gz

.EXAMPLE
    .\deploy.ps1

.EXAMPLE
    .\deploy.ps1 -ImageFile "C:\Downloads\itemcosttracker.tar.gz"
#>

param(
    [string]$ImageFile = ""
)

$ErrorActionPreference = "Stop"
$scriptDir = $PSScriptRoot
if (-not $ImageFile) { $ImageFile = Join-Path $scriptDir "itemcosttracker.tar.gz" }
$ImageFile = [System.IO.Path]::GetFullPath($ImageFile)

Write-Host "ItemCostTracker Deploy" -ForegroundColor Cyan
Write-Host "  Image file: $ImageFile"
Write-Host ""

if (-not (Test-Path $ImageFile)) {
    Write-Host "ERROR: Image file not found: $ImageFile" -ForegroundColor Red
    exit 1
}

# Stop running container
Write-Host "Stopping container..." -ForegroundColor Yellow
docker compose down

# Load new image
Write-Host "Loading image..." -ForegroundColor Yellow
docker load -i $ImageFile

# Start container
Write-Host "Starting container..." -ForegroundColor Yellow
docker compose up -d

Write-Host ""
Write-Host "Deploy complete!" -ForegroundColor Green
