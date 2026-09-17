---
sidebar_position: 1
title: Connectors
---

import RelatedArticles from '@site/src/components/RelatedArticles';

# Connectors

:::note Optional by design
Yard treats Zyvor products as optional connectors. The platform installs
and runs without Device Agent, Nodra, Fleet, or OTA.
:::

## Authentication

Connectors send `Authorization: Bearer <token>`. Tokens are hashed at rest
(SHA-256) and scoped to one organization.

:::note Connector token vs. API key
A connector token authenticates one machine integration against the
`/api/v1/ingest/*` and `/api/v1/actions` routes on this page. A human
[API key](../security#connector-tokens-and-api-keys) authenticates one
person against everything their own session can already do — the two
are separate credential types, not interchangeable.
:::

## HTTP ingestion

`POST /api/v1/ingest/inventory`

```json
{
  "external_ref": "ZY-GW-0001",
  "name": "Edge gateway",
  "kind": "device",
  "manufacturer": "Zyvor",
  "model": "Device Agent",
  "serial": "DA-9188-0001",
  "site_name": "Riverside Plant",
  "capabilities": ["cpu_temp", "heartbeat"],
  "metadata": "{}"
}
```

`POST /api/v1/ingest/observations` — single object or array:

```json
{
  "asset_external_ref": "ZY-GW-0001",
  "capability": "cpu_temp",
  "value": 41.2,
  "unit": "°C",
  "quality": "good",
  "source": "device-agent",
  "observed_at": "2026-09-13T01:00:00Z",
  "dedupe_key": "ZY-GW-0001:cpu_temp:20260913010000"
}
```

Duplicate `dedupe_key` values are accepted and ignored. Ingest is rate
limited.

`POST /api/v1/ingest/events` records an operational event.

### Live position updates

There is no separate "location" endpoint — a moving asset (a vehicle, a
mobile gateway) reports live position through the same `inventory` call
above. POST an updated `latitude`/`longitude` for the asset's
`external_ref` every 10-30 seconds and Yard treats it as the asset's
current position; the Map page picks it up automatically over the
existing `/api/v1/stream` SSE connection, no polling required. Omitting
`latitude`/`longitude` on a later inventory update (for example, one that
only refreshes capabilities) leaves the asset's last known position
untouched rather than clearing it, so a connector can send partial
updates freely.

## Device Agent gateway

```bash
export YARD_INGEST_TOKEN=$(cat data/ingest.token)
export DEVICE_AGENT_URL=http://127.0.0.1:9188
go run ./cmd/agent-gateway
```

The gateway pulls the agent locally and pushes normalized inventory and
observations. From the Integrations console you can run
`inventory.refresh` and `diagnostics.read`.

## Optional Zyvor connectors

| Connector | Yard responsibility | External responsibility |
| --- | --- | --- |
| Nodra | Pull devices/twins and publish inventory + numeric observations | Protocol interpretation, WAL, twins |
| Zyvor Fleet | Show sites, rollouts, and OTA device lifecycle progress | Desired state, site reconciliation |
| OTA | List campaigns (via Fleet or Nodra) in the Integrations tab | Execute the update |

### Wiring (shipped)

1. On **Integrations**, set each connector **Endpoint** and put the API bearer in
   `config.auth_token` (JSON on the connector).
2. Run actions from the console (or `POST /api/v1/actions`):

| Connector | Action | Behavior |
| --- | --- | --- |
| Device Agent | `inventory.refresh` / `sync` | Pull agent inventory/sensors → Yard ingest |
| Device Agent | `diagnostics.read` | Return agent health/inventory JSON |
| Nodra | `telemetry.receive` / `sync` | List devices + twins → ingest |
| Fleet | `lifecycle.request` / `desired.progress` / `sync` | Read sites, rollouts, OTA devices |
| OTA | `campaign.list` / `update.delegate` / `sync` | Fleet `/api/v1/ota/devices` or Nodra campaigns (`config.source` = `fleet`\|`nodra`) |

3. Asset detail **Integrations** tab: `GET /api/v1/assets/{id}/integrations`.
4. Outbound sync needs `YARD_INGEST_TOKEN` (or `YARD_INGEST_TOKEN_FILE` /
   `data/ingest.token`) because connector tokens are stored hashed.
5. For **HTTPS Device Agent** with a lab self-signed cert, Yard skips TLS
   verify by default on `https://` endpoints (set `config.tls_insecure: false`
   to require a trusted CA). Pass the agent bearer as `config.auth_token`.

Unknown connector kinds still return `unsupported`.

Full contract notes live in the repo at
[docs/CONNECTORS.md](https://github.com/zyvorai/yard/blob/main/docs/CONNECTORS.md).

For console bulk import, map clustering, and severity runbooks, see
[Console features](./console).

<RelatedArticles
  items={[
    {label: 'Console features', to: './console', description: 'Integrations tab and Device Agent gateway'},
    {label: 'API', to: '../api', description: 'Ingest and auth reference'},
    {label: 'Architecture', to: '../core-concepts/architecture', description: 'Data model and boundaries'},
  ]}
/>
