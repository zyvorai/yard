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

After deploy, useful console paths: **Assets** (bulk CSV/JSON), **Map**
(clustering), **Administration** (severity policies). See
[Console features](./console).

<RelatedArticles
  items={[
    {label: 'Console features', to: './console', description: 'Bulk IO, map clustering, runbooks'},
    {label: 'Quickstart', to: '../getting-started/quickstart', description: 'Run Yard locally in under a minute'},
    {label: 'Connectors', to: './connectors', description: 'Ingest and Device Agent'},
  ]}
/>
