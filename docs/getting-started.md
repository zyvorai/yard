# Getting started

Prerequisites, first run, Docker Compose, remote lab deploy, backups and the Device Agent gateway.

[Back to the README](../README.md)

## Prerequisites

Go 1.27+ (`go.mod`) and Node 20+ (the docs site build uses Node 22). SQLite ships with the Go standard toolchain via `modernc.org/sqlite` — no CGO, no system SQLite package required. Postgres is optional (`docker compose --profile postgres`). No other services are required to run Yard standalone.

```bash
make help
make ci                         # gofmt, vet, tests
make status                     # GET /healthz on a running server
make deploy-remote H=<host> U=sus
make ship HOST=user@host        # older alias
```

## Quick start

```bash
go run ./cmd/yard
```

Open [http://127.0.0.1:8080](http://127.0.0.1:8080). The default mode is `demo`:

```
admin@yard.local
yard-admin
```

Production refuses that login. Set `YARD_MODE=production`, `YARD_PUBLIC_URL`, `YARD_SECRET_KEY`, and on an empty database `YARD_BOOTSTRAP_EMAIL` and `YARD_BOOTSTRAP_PASSWORD`. Details: [SECURITY.md](../SECURITY.md) and [docs/PHASES.md](PHASES.md).

In another terminal:

```bash
export YARD_SIMULATOR_TOKEN=$(cat data/simulator.token)
go run ./cmd/simulator
```

First-use path: seeded workspace → simulator (or Device Agent gateway) → discover assets → inspect health → temperature / missed heartbeat opens an incident with severity policy + runbook → assign a work order → record resolution on the asset timeline.

Console development (Vite proxies `/api` to `:8080`):

```bash
cd web && npm install && npm run dev
```

### Docker Compose

```bash
docker compose up --build
```

PostgreSQL is optional (SQLite remains the default for `go run` and CI):

```bash
docker compose --profile postgres up --build
# Yard on :8081 with YARD_DATABASE_URL=postgres://...
```

### Remote lab deploy

Same one-command pattern as Fabric/Nodra — cross-compile locally, install a systemd unit, verify `/healthz`:

```bash
./scripts/ship sus@HOST              # quick redeploy
./scripts/ship sus@HOST --full       # first install + firewall
./scripts/ship sus@HOST --with-sim   # also start the simulator
./scripts/ship sus@HOST --dry-run
```

Open `http://HOST:18080` (default lab port; override with `--port`). Demo login remains `admin@yard.local` / `yard-admin`.

```bash
YARD_URL=http://HOST:18080 ./scripts/verify-remote.sh
```

### Backups

Works against either backend, auto-detected from `YARD_DATABASE_URL` (SQLite via `sqlite3 ... VACUUM INTO`, a live consistent snapshot; Postgres via `pg_dump`/`pg_restore`):

```bash
./scripts/backup.sh                    # snapshot to backups/yard-<timestamp>.db|.dump
./scripts/restore.sh backups/yard-....db   # restores; saves the current file as *.before-restore first
```

### Device Agent gateway

```bash
export YARD_INGEST_TOKEN=$(cat data/ingest.token)
export DEVICE_AGENT_URL=http://127.0.0.1:9188
go run ./cmd/agent-gateway
```

The gateway reads the agent locally and publishes normalized inventory and observations. From Integrations you can also run `inventory.refresh` and `diagnostics.read`. It does not require inbound access to every remote device.
