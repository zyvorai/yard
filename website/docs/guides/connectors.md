---
sidebar_position: 1
title: Connectors
---

# Connectors

Yard treats Zyvor products as optional connectors. The platform installs
and runs without Device Agent, Nodra, Fleet, or OTA.

## Authentication

Connectors send `Authorization: Bearer <token>`. Tokens are hashed at rest
(SHA-256) and scoped to one organization.

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
| Nodra | Display decoded telemetry once published here | Protocol interpretation, WAL, twins |
| Zyvor Fleet | Show lifecycle request progress | Desired state, site reconciliation |
| OTA | List campaigns in the Integrations tab | Execute the update |

Nodra, Fleet, and OTA stay catalog-only until their product APIs are wired.
Actions against them return `unsupported` rather than a fake success.

Full contract notes live in the repo at
[docs/CONNECTORS.md](https://github.com/zyvorai/yard/blob/master/docs/CONNECTORS.md).

For console bulk import, map clustering, and severity runbooks, see
[Console features](./console).
