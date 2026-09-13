# Connector contracts

Estate treats Zyvor products as optional connectors. The platform installs
and runs without Device Agent, Nodra, Fleet, or OTA.

## Authentication

Connectors send `Authorization: Bearer <token>`. Tokens are hashed at rest
(SHA-256). Tokens are scoped to one organization.

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

`POST /api/v1/ingest/observations`

Single object or array:

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

Every observation stores source, unit, observation time, receipt time, and
quality. Duplicate `dedupe_key` values are accepted and ignored.

`POST /api/v1/ingest/events` records an operational event.

## Device Agent gateway

`cmd/agent-gateway` pulls the agent locally (`GET /api/v1/inventory`,
`GET /api/v1/sensors`) and pushes normalized records to Estate. Remote
devices do not need inbound ports.

## Optional Zyvor connectors

| Connector | Estate responsibility | External responsibility |
| --- | --- | --- |
| Nodra | Display decoded telemetry once published here | Protocol interpretation, WAL, twins |
| Zyvor Fleet | Show lifecycle request progress | Desired state, site reconciliation |
| OTA | List campaigns in the asset Integrations tab | Execute the update |

Each connector advertises supported actions. The UI only offers actions the
connector declares.

## MQTT adapter

A future adapter maps MQTT payloads onto the same HTTP ingest contract.
Topic and payload mapping belong in the adapter, not in the core schema.
