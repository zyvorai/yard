---
sidebar_position: 1
title: Quickstart
---

# Quickstart

Run Yard locally in under a minute. Demo login:

```
admin@yard.local
yard-admin
```

## Local server

```bash
go run ./cmd/yard
```

Open [http://127.0.0.1:8080](http://127.0.0.1:8080).

In another terminal, start the included simulator:

```bash
export YARD_SIMULATOR_TOKEN=$(cat data/simulator.token)
go run ./cmd/simulator
```

First-use path: seeded workspace → simulator (or Device Agent gateway) →
discover assets → inspect health → temperature / missed heartbeat opens an
incident (with severity policy + runbook) → assign a work order → record
resolution on the asset timeline.

## Console development

```bash
cd web && npm install && npm run dev
```

Vite proxies `/api` to `:8080`.

## Docker Compose

```bash
docker compose up --build
```

PostgreSQL is optional:

```bash
docker compose --profile postgres up --build
```

## Remote lab deploy

Same one-command pattern as Fabric/Nodra:

```bash
./scripts/ship sus@HOST              # quick redeploy
./scripts/ship sus@HOST --full       # first install + firewall
./scripts/ship sus@HOST --with-sim   # also start the simulator
```

Open `http://HOST:18080` (default lab port). Smoke from your laptop:

```bash
YARD_URL=http://HOST:18080 ./scripts/verify-remote.sh
```

See [Deploy](../guides/deploy) for details.

## What's next

- [Architecture](../core-concepts/architecture) — asset model and boundaries
- [Console features](../guides/console) — bulk IO, map clustering, runbooks
- [Connectors](../guides/connectors) — ingest and Device Agent
- [API](../api) — export/import and severity policies
- Full feature catalog: [docs/ROADMAP.md](https://github.com/zyvorai/yard/blob/master/docs/ROADMAP.md)
