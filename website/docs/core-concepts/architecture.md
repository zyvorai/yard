---
sidebar_position: 1
title: Architecture
---

import RelatedArticles from '@site/src/components/RelatedArticles';

# Architecture

Yard is a standalone **asset and operations** platform. The core object is
an **asset**. Devices, vehicles, machines, sensors, and equipment are asset
kinds. Zyvor products plug in as optional connectors — Yard installs and
runs alone.

## Data model

Organization · User · Session · APIKey · Site · Location · Asset · Capability · Observation ·
Event · WorkOrder · Incident · SeverityPolicy · ActionRequest · Job ·
Connector · Automation

Every observation stores **source**, **unit**, **observed_at**,
**received_at**, **quality**, optional **quality_reason**, **uncertainty**,
**calibration_state**, and **sequence_num**. Offline data is marked stale rather than
healthy.

Remote actions with a connector are inserted as `queued` and run by an
in-process worker (`internal/queue`). Idempotency keys and expiry remain.
Outbound connector credentials are encrypted; the API returns a hint only.

`GET`/`POST /api/v1/locations` stores a parented place. Asset templates,
catalogs, and links are available in the API and console where operators
need them.

## Boundaries

| Component | Responsibility |
| --- | --- |
| **Yard** | Asset registry, sites, workflows, incidents, shared UI |
| **Device Agent** | Hardware discovery, health, local diagnostics |
| **Nodra connector** | Pull decoded telemetry / twins into Yard ingest |
| **Zyvor Fleet connector** | Lifecycle / rollout / OTA-device progress display |
| **OTA connector** | Campaign list display; execution stays elsewhere |
| **HTTP / simulator** | Zero-dependency evaluation path |

Device Agent reports physical capability. Nodra interprets protocols.
Fleet owns desired state. Yard preserves those lines and adds the
operations surface: health, incidents, and work orders. Optional
connectors are wired for outbound sync when an endpoint and a stored
secret are set — see [Connectors](../guides/connectors).

## Live ops loop

- Background stale ticker marks missed heartbeats without waiting for an
  Overview refresh. Opening Overview does not do that work
- Console pages obtain a single-use ticket and subscribe to
  `GET /api/v1/stream?ticket=`
- Automations trigger on a literal `threshold`, a capability's own
  declared `Min`/`Max` range (`capability_min`/`capability_max`, so the
  range lives in one place), or a `stale` heartbeat — and can
  `open_incident`, `notify` (audit), or call a `webhook`/`slack`/
  `email`/`pagerduty` action, all behind one shared dispatch path so a
  new action type is one addition, not two
- Opened incidents inherit severity + runbook from severity policies

## Registry and map

- Assets support create/edit/delete, capability editing, and **bulk
  CSV/JSON import-export**
- MapLibre map uses **clustered** pin layers for dense sites/assets

## Storage

- **SQLite** by default (`go run`, CI, Compose)
- **Postgres** via `YARD_DATABASE_URL` (Compose `--profile postgres`)
- Schema changes are versioned in `schema_migrations`. `/readyz` stays
  unavailable until the database is up and that version matches the binary
- `scripts/backup.sh`/`restore.sh` snapshot either backend (SQLite
  `VACUUM INTO`; Postgres `pg_dump`/`pg_restore`) — see
  [Deploy → Backups](../guides/deploy#backups)

<RelatedArticles
  items={[
    {label: 'API', to: '../api', description: 'Endpoints for every object in the data model'},
    {label: 'Console features', to: '../guides/console', description: 'Bulk IO, map clustering, runbooks'},
    {label: 'Programs', to: '../guides/programs', description: 'What is shipped, partial, and planned'},
  ]}
/>
