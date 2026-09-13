---
sidebar_position: 3
title: Console features
---

# Console features

Day-to-day surfaces beyond the [quickstart](../getting-started/quickstart)
loop.

## Assets — bulk CSV / JSON

On **Assets**:

- **Export CSV** / **Export JSON** — downloads the current filter set
  (`q`, `kind`) via `GET /api/v1/assets/export`
- **Import** — accepts `.csv` or `.json`; rows upsert by `external_ref`

Typical CSV header:

```text
name,external_ref,kind,status,health,manufacturer,model,serial,site_id,latitude,longitude,stale_after_sec,metadata
```

## Map — clustering

The Map page uses MapLibre **clustered GeoJSON** layers:

- Dense pins collapse into count clusters (blue → amber → orange by size)
- Click a cluster to zoom into its members
- Click an unclustered pin to open the detail card
- Site pins use blue; asset pins follow health (healthy / warning /
  critical / stale)

## Incidents — severity + runbooks

When a threshold or stale automation opens an incident, Yard resolves a
**severity policy** (highest priority match):

| Match kind | Example | Effect |
| --- | --- | --- |
| `capability` | `temperature` | severity + runbook for that signal |
| `automation` | `Missed heartbeat` | match by automation name |
| `default` | *(empty value)* | fallback for everything else |

Seeded defaults include a critical temperature runbook and a warning
heartbeat checklist. The Incidents detail panel shows the attached
**Runbook** text.

Edit policies under **Administration → Incident severity policies**
(`GET/POST /api/v1/severity-policies`, `PATCH/DELETE …/{id}`).

## Related

- [API](../api) — curl examples for export/import and policies
- [Architecture](../core-concepts/architecture) — data model
- Full catalog: [ROADMAP.md](https://github.com/zyvorai/yard/blob/master/docs/ROADMAP.md)
