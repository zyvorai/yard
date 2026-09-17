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

Organization · User · APIKey · Site · Asset · Capability · Observation ·
Event · WorkOrder · Incident · SeverityPolicy · ActionRequest ·
Connector · Automation

Every observation stores **source**, **unit**, **observed_at**,
**received_at**, and **quality**. Offline data is marked stale rather than
healthy.

:::note SeverityPolicy
Rows map `capability`, `automation`, or `default` matches (ordered by
priority) to incident severity and runbook text when automations open
incidents.
:::

Remote actions require a session, expire, carry an idempotency key, and
record an outcome. Connectors advertise the actions they can execute.

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
connectors are wired for outbound sync when endpoint + `auth_token` are
set — see [Connectors](../guides/connectors).

## Live ops loop

- Background stale ticker marks missed heartbeats without waiting for an
  Overview refresh
- Console pages subscribe to `GET /api/v1/stream` (SSE)
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
- Schema changes are versioned, transactional migrations tracked in a
  `schema_migrations` table — not ad hoc `CREATE TABLE IF NOT EXISTS`
  — so upgrades are ordered and idempotent across both backends
- `scripts/backup.sh`/`restore.sh` snapshot either backend (SQLite
  `VACUUM INTO`; Postgres `pg_dump`/`pg_restore`) — see
  [Deploy → Backups](../guides/deploy#backups)

<RelatedArticles
  items={[
    {label: 'API', to: '../api', description: 'Endpoints for every object in the data model'},
    {label: 'Console features', to: '../guides/console', description: 'Bulk IO, map clustering, runbooks'},
    {label: 'Connectors', to: '../guides/connectors', description: 'How optional connectors plug in'},
  ]}
/>
