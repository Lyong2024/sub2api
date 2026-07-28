# Sub2API DIY Mode

Single-binary deployment: **embedded Web UI + SQLite (WAL) + in-process Redis**.  
Only **port 8080** is required. No external PostgreSQL or Redis.

> Chinese guide: [README_DIY_CN.md](README_DIY_CN.md)

## Features

| Item | DIY |
|------|-----|
| Database | SQLite3 WAL (`data/sub2api.db`) |
| Cache / locks | Embedded miniredis |
| Frontend | Built into the binary (`-tags embed`) |
| Port | `8080` (API + Web UI) |
| Config | `.env` and/or `config.yaml` |
| Auto install | `AUTO_SETUP=true` (default in DIY) |

## Download binaries

GitHub Actions builds multi-platform packages on **every push to the `diy` branch** and on tags `diy-v*`.

1. Open the repository **Actions** → workflow **DIY Release**
2. Download the artifact for your OS/arch, e.g.  
   `sub2api-diy-linux-amd64` → extract `sub2api`
3. Or use a Release created from a `diy-v*` tag

Supported targets: `linux/amd64`, `linux/arm64`, `windows/amd64`, `darwin/amd64`, `darwin/arm64`.

## Quick start (manual)

```bash
# 1) Extract binary
chmod +x sub2api

# 2) Optional config
cp config.example.yaml config.yaml   # or use .env.example → .env
# Edit JWT_SECRET / ADMIN_* as needed

# 3) Run
export DEPLOY_MODE=diy
export AUTO_SETUP=true
./sub2api
```

Open `http://127.0.0.1:8080`.  
On first run the process creates SQLite schema, admin user, `config.yaml`, and `.installed`.

### Environment variables

| Variable | Meaning | Default |
|----------|---------|---------|
| `DEPLOY_MODE` | `diy` / `standard` | `standard` |
| `DATABASE_DRIVER` | `sqlite` / `postgres` | `postgres` |
| `DATABASE_PATH` | SQLite file path | `./data/sub2api.db` or `$DATA_DIR/sub2api.db` |
| `REDIS_EMBEDDED` | In-process Redis | `true` in DIY |
| `SERVER_PORT` | Listen port | `8080` |
| `AUTO_SETUP` | Auto install if no config | `true` in DIY |
| `ADMIN_EMAIL` | Bootstrap admin | `admin@sub2api.local` |
| `ADMIN_PASSWORD` | Bootstrap password | auto-generated if empty |
| `JWT_SECRET` | ≥32 bytes | auto-generated if empty |
| `DATA_DIR` | Config + data directory | `.` |
| `CONFIG_FILE` | Absolute config path | search paths |

Example `.env`:

```bash
DEPLOY_MODE=diy
AUTO_SETUP=true
SERVER_PORT=8080
DATABASE_PATH=./data/sub2api.db
ADMIN_EMAIL=admin@example.com
ADMIN_PASSWORD=change-me-now
JWT_SECRET=change-this-to-a-secure-random-string-32b
TZ=Asia/Shanghai
```

## Linux: systemd (boot on start)

Package ships `sub2api.service` and `install-systemd.sh`.

```bash
# As root
sudo ./install-systemd.sh ./sub2api

# Or manually:
sudo useradd --system --home /var/lib/sub2api --shell /usr/sbin/nologin sub2api || true
sudo mkdir -p /opt/sub2api /var/lib/sub2api /etc/sub2api
sudo install -m 0755 ./sub2api /opt/sub2api/sub2api
sudo cp deploy/diy/sub2api.service /etc/systemd/system/sub2api.service
sudo cp deploy/diy/.env.example /etc/sub2api/sub2api.env
sudo nano /etc/sub2api/sub2api.env   # set ADMIN_PASSWORD / JWT_SECRET
sudo chown -R sub2api:sub2api /var/lib/sub2api /opt/sub2api
sudo systemctl daemon-reload
sudo systemctl enable --now sub2api
sudo systemctl status sub2api
```

Useful commands:

```bash
sudo journalctl -u sub2api -f
sudo systemctl restart sub2api
sudo systemctl stop sub2api
```

Data lives under `/var/lib/sub2api` when using the unit file (`DATA_DIR`).

### Reverse proxy (optional)

Caddy / Nginx can terminate TLS and proxy to `127.0.0.1:8080`.  
DIY itself only needs the single process on 8080.

## Windows (quick)

```powershell
$env:DEPLOY_MODE = "diy"
$env:AUTO_SETUP = "true"
.\sub2api.exe
```

For auto-start, use **Task Scheduler** (run at logon/startup) or NSSM to register a Windows service pointing at `sub2api.exe` with the same environment variables.

## Build from source

```bash
# Frontend
cd frontend && pnpm install && pnpm run build && cd ..

# Backend (embed UI)
cd backend
CGO_ENABLED=0 go build -tags embed -ldflags="-s -w" -o ../dist/sub2api ./cmd/server

# Or
make diy-build
make diy-run
```

## Data & backup

| Path | Description |
|------|-------------|
| `sub2api.db` | Main SQLite DB |
| `sub2api.db-wal` / `-shm` | WAL sidecars (copy together) |
| `config.yaml` | Runtime config (secrets) |
| `.installed` | Install lock |

Stop the service before cold backups:

```bash
sudo systemctl stop sub2api
sudo tar czf sub2api-backup.tgz -C /var/lib/sub2api .
sudo systemctl start sub2api
```

## Limits (honest)

- **Single process** — not multi-instance HA
- **SQLite write serialisation** — fine for small/medium load
- **Ops / heavy analytics** — disabled by default (`ops.enabled=false`)
- **Outbox SKIP LOCKED** paths are skipped; full rebuild + in-process cache cover DIY
- For large multi-node production, use standard mode (PostgreSQL + Redis)

## Troubleshooting

| Symptom | Check |
|---------|--------|
| Port in use | `SERVER_PORT` or free 8080 |
| Re-setup forced | Do not delete `.installed` + `config.yaml` casually |
| Login fails | Reset admin via fresh `DATA_DIR` or DB backup restore |
| Permission denied (systemd) | `chown -R sub2api:sub2api /var/lib/sub2api` |

## License

Same as the main Sub2API project.
