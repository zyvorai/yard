---
sidebar_position: 2
title: Deploy
---

import RelatedArticles from '@site/src/components/RelatedArticles';

# Deploy

Yard ships with Fabric-style scripts: cross-compile locally, install a
systemd unit on the host, verify `/healthz`.

## One command

```bash
./scripts/ship sus@HOST              # quick redeploy
./scripts/ship sus@HOST --full       # first install + firewall
./scripts/ship sus@HOST --with-sim   # also start yard-simulator
./scripts/ship sus@HOST --dry-run
./scripts/ship sus@HOST --port 18080
```

:::tip Default port
Default lab listen port is **18080** (8080 is often occupied). Override
with `--port` or reuse `.deploy-last`.
:::

Demo login after bootstrap:

```
admin@yard.local / yard-admin
```

## What gets installed

| Path | Role |
| --- | --- |
| `/usr/local/bin/yard` | API + embedded console |
| `/etc/yard/yard.env` | Listen address, DSN, token files |
| `/var/lib/yard/` | SQLite DB + ingest/simulator tokens |
| `yard.service` | systemd unit |
| `yard-simulator.service` | Optional included simulator |

On hosts that previously ran the Estate rename, ship disables
`estate.service` so Yard can bind the listen port.

## Verify

```bash
YARD_URL=http://HOST:18080 ./scripts/verify-remote.sh
```

Checks health, login, overview, assets, connectors, and (when a simulator
token is available) the temperature → incident gate (severity policy +
runbook attached).

## Compose

```bash
docker compose up --build
docker compose --profile postgres up --build
```

## Helm (Kubernetes)

A starter chart lives at `deploy/helm/yard`:

```bash
helm upgrade --install yard ./deploy/helm/yard \
  --set listen=":8080" \
  --set oidc.enabled=false
```

| Value | Purpose |
| --- | --- |
| `listen` / `databaseUrl` | Process bind address and SQLite/Postgres DSN |
| `persistence.*` | PVC for SQLite when not using an external DB |
| `oidc.enabled` + `oidc.issuer` / `clientId` / `clientSecret` | Sets `YARD_OIDC_*` for `GET /api/v1/auth/oidc` discovery (browser callback is a follow-up) |

For remote hosts, prefer `make deploy-remote H=<host> U=sus`
(`./scripts/deploy-remote.sh USER@HOST`, or `./scripts/ship`). Set `YARD_URL` to a loopback URL inside the process
(`http://127.0.0.1:<port>`) when connectors sync into the same Yard —
using the public IP can hairpin-NAT and stall ingest.

## Backups

Auto-detects SQLite vs. Postgres from `YARD_DATABASE_URL`:

```bash
./scripts/backup.sh                        # snapshot to backups/yard-<timestamp>.db|.dump
./scripts/restore.sh backups/yard-....db   # saves the current file as *.before-restore first
```

SQLite uses `sqlite3 ... VACUUM INTO` — a live, consistent snapshot that
doesn't lock out the running server. Postgres uses `pg_dump -Fc` /
`pg_restore --clean --if-exists`. Every backup run writes a new
timestamped file; nothing is ever overwritten.

## Logging

Every request is logged as one structured line (method, path, status,
duration). Set `YARD_LOG_FORMAT=json` for machine-parseable output in
production; the default is plain text, easier to read in a terminal.

After deploy, useful console paths: **Assets** (bulk CSV/JSON), **Map**
(clustering), **Administration** (users, API keys, severity policies).
See [Console features](./console).

<RelatedArticles
  items={[
    {label: 'Console features', to: './console', description: 'Bulk IO, map clustering, runbooks'},
    {label: 'Quickstart', to: '../getting-started/quickstart', description: 'Run Yard locally in under a minute'},
    {label: 'Connectors', to: './connectors', description: 'Ingest and Device Agent'},
  ]}
/>
