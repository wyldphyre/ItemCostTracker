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
$loadOutput = docker load -i $ImageFile
$loadOutput | ForEach-Object { Write-Host "  $_" }

# Verify the load actually produced the expected image tag. If it didn't,
# `docker compose up` would silently fall back to `build: .` and compile from
# the server's (possibly stale) source tree, deploying old code.
if (-not ($loadOutput -match "Loaded image: itemcosttracker")) {
    Write-Host "ERROR: docker load did not produce the 'itemcosttracker' image." -ForegroundColor Red
    Write-Host "       Aborting so we don't run a stale server-built image." -ForegroundColor Red
    exit 1
}

# Start container.
#   --no-build      : never rebuild from server source; only run the loaded image
#   --force-recreate: recreate the container even if compose thinks it's unchanged
Write-Host "Starting container..." -ForegroundColor Yellow
docker compose up -d --force-recreate --no-build

Write-Host ""
Write-Host "Deploy complete!" -ForegroundColor Green
