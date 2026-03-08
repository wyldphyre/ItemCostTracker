# ItemCostTracker

A web app for tracking the cost-over-time of items you own — electronics, furniture, shoes, etc. For each item it calculates cost per day, month, and year based on purchase price, resale value, additional costs, and how long you've owned it. Useful for understanding which purchases delivered good value.

Replaces a personal Excel spreadsheet. Key improvement: multiple labelled additional cost entries per item (e.g. accessories, repairs, trade-in credits) rather than a single summed field.

## Features

- Add, edit, and delete items
- Tracks purchase price, resale value, and multiple additional costs (each with a description and amount — can be negative for credits)
- Calculates cost per day, month, and year using Excel-compatible YEARFRAC (US 30/360)
- Active items (no final date set) update their cost metrics daily using today as the end date
- Projected years of use with projected cost/year
- Highlights items that have reached or exceeded their projected lifetime
- Export all data as JSON (re-importable) or CSV
- Import JSON — merge with existing data or replace all
- Filter to show active items only

## Stack

- **Backend:** Go, standard library only (`net/http`, `html/template`, `embed`)
- **Frontend:** [HTMX](https://htmx.org) 2.0 (CDN) + Go HTML templates, plain CSS
- **Storage:** JSON file with atomic writes
- **Binary:** Single static binary with all templates and CSS embedded via `//go:embed`

## Requirements

- Go 1.22 or later

## Build and run

```bash
go build -o itemcosttracker .
./itemcosttracker
```

Open [http://localhost:8080](http://localhost:8080).

### Environment variables

| Variable   | Default  | Description                        |
|------------|----------|------------------------------------|
| `ADDR`     | `:8080`  | TCP address to listen on           |
| `DATA_DIR` | `./data` | Directory where `items.json` lives |

```bash
ADDR=:9000 DATA_DIR=/var/data ./itemcosttracker
```

### Run tests

```bash
go test ./...
```

## Docker

The image is built from `scratch` — just the binary, no OS layer. Go only runs inside the build container so the destination machine only needs Docker, not Go.

### Build and run locally

```bash
docker compose up --build -d
```

The app is available on port 5010. Data is stored in `./data/items.json` (a bind mount) and persists across restarts and rebuilds.

```bash
docker compose down   # stop
docker compose logs   # view logs
```

### Deploy to another machine

**Step 1 — Build and export the image on this machine:**

```bash
docker build -t itemcosttracker .
docker save itemcosttracker | gzip > itemcosttracker.tar.gz
```

**Step 2 — Copy to the destination machine:**

```bash
scp itemcosttracker.tar.gz compose.yaml user@destination:~/itemcosttracker/
scp data/items.json user@destination:~/itemcosttracker/data/items.json
```

**Step 3 — On the destination machine:**

```bash
cd ~/itemcosttracker
docker load -i itemcosttracker.tar.gz
docker compose up -d
```

On Windows, `deploy.ps1` automates steps 2–3 (down, load, up):

```powershell
.\deploy.ps1
# or if the archive is elsewhere:
.\deploy.ps1 -ImageFile "C:\Downloads\itemcosttracker.tar.gz"
```

Data is stored in `./data/items.json` relative to `compose.yaml` and persists across restarts.

#### Windows + Docker Desktop (WSL2) — external access

If the destination is a Windows machine running Docker Desktop with the WSL2 backend, containers bind to the WSL2 VM's internal IP rather than the Windows host's network interface. An inbound firewall rule alone is not enough — you also need a port proxy. Run this in an **elevated PowerShell or Command Prompt**:

```cmd
netsh interface portproxy add v4tov4 listenport=5010 listenaddress=0.0.0.0 connectport=5010 connectaddress=<WSL2-IP>
```

Find the WSL2 IP with:

```powershell
wsl hostname -I
```

The WSL2 IP can change on reboot. To update the proxy after a reboot:

```cmd
netsh interface portproxy reset
netsh interface portproxy add v4tov4 listenport=5010 listenaddress=0.0.0.0 connectport=5010 connectaddress=<new-WSL2-IP>
```

## Importing from an Excel spreadsheet

A command-line tool converts the original "Item Cost per Time Period" `.xlsx` format to the JSON import format:

```bash
go run ./cmd/importxlsx <spreadsheet.xlsx> [output.json]
```

If `output.json` is omitted, JSON is written to stdout. The output can be imported via the web UI (**Import** button) or placed directly at `data/items.json`.

Expected spreadsheet columns: A=Item, B=Purchase Date, C=Purchase Price, D=Final Activity Date, E=Resale Value, F=Additional Cost, N=Projected Years.

## Project structure

```
cmd/importxlsx/      xlsx → JSON converter (stdlib only, no xlsx library)
data/                items.json (created at runtime, not embedded)
internal/
  handler/           HTTP handlers, template functions, import/export
  model/             Item struct, Calculated struct, Compute(), YearFrac()
  store/             JSON persistence with RWMutex and atomic writes
static/              CSS (embedded in binary)
templates/           HTML templates (embedded in binary)
backup.ps1           Windows backup script (daily/weekly/monthly rotation)
build-image.sh       Builds Docker image and exports itemcosttracker.tar.gz
deploy.ps1           Windows deploy script (load image + restart container)
```

## Data file

Items are stored in `data/items.json` as a JSON array. The file is created automatically on first write. You can back it up, edit it directly, or restore it via the Import feature.

## Backup (Windows)

`backup.ps1` creates compressed backups of `items.json` with daily/weekly/monthly rotation.

```powershell
# Basic usage — backs up to .\backups\ by default
.\backup.ps1

# Specify a backup destination (e.g. a NAS share)
.\backup.ps1 -BackupDir "Z:\Backups\ItemCostTracker"
.\backup.ps1 -BackupDir "\\nas\backups\itemcosttracker"

# Custom retention
.\backup.ps1 -BackupDir "Z:\Backups\ItemCostTracker" -DailyKeep 14 -WeeklyKeep 8 -MonthlyKeep 24
```

Backups are placed in `daily\`, `weekly\`, and `monthly\` subdirectories under the backup destination. Weekly backups are created on Sundays, monthly on the 1st of each month. Old backups beyond the retention limits are pruned automatically.

To restore, extract the zip and copy `items.json` back to the `data\` directory (or use the Import feature in the web UI).
