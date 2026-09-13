<#
.SYNOPSIS
    Deploy a new ItemCostTracker image from a tar.gz archive.

.DESCRIPTION
    Loads the new image from itemcosttracker.tar.gz, verifies it, and only then
    recreates the running container. If the archive is missing or fails to load,
    the current container is left running untouched.

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

# Load the new image while the current container keeps serving. Native command
# failures don't throw under $ErrorActionPreference, so check $LASTEXITCODE.
# stderr is deliberately not redirected: in Windows PowerShell 5.1, `2>&1` on a
# native command turns its first stderr line into a terminating error.
Write-Host "Loading image..." -ForegroundColor Yellow
$loadOutput = docker load -i $ImageFile
$loadExit = $LASTEXITCODE
$loadOutput | ForEach-Object { Write-Host "  $_" }

if ($loadExit -ne 0) {
    Write-Host "ERROR: docker load failed (exit code $loadExit). The running container was not touched." -ForegroundColor Red
    exit 1
}

# Verify the load produced the expected tag. Otherwise `docker compose up` would
# quietly recreate the container from the previously loaded (old) image.
if (-not ($loadOutput -match "Loaded image: itemcosttracker")) {
    Write-Host "ERROR: docker load did not produce the 'itemcosttracker' image." -ForegroundColor Red
    Write-Host "       The running container was not touched." -ForegroundColor Red
    exit 1
}

# Replace the container. No `docker compose down` first: `up --force-recreate`
# stops and replaces it itself, so the app is only down for the swap.
#   --no-build      : never rebuild from server source; only run the loaded image
#   --force-recreate: recreate the container even if compose thinks it's unchanged
Write-Host "Recreating container..." -ForegroundColor Yellow
docker compose up -d --force-recreate --no-build
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: docker compose up failed (exit code $LASTEXITCODE)." -ForegroundColor Red
    Write-Host "       Check 'docker compose ps' and 'docker compose logs'." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "Deploy complete!" -ForegroundColor Green
