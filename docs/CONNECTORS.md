# Connector contracts

Yard treats Zyvor products as optional connectors. The platform installs
and runs without Device Agent, Nodra, Fleet, or OTA.

## Authentication

Connectors send `Authorization: Bearer <token>`. Tokens are hashed at rest
(SHA-256). Tokens are scoped to one organization.

A connector token authenticates one machine integration against the
ingest/action routes below. A human API key (`/api/v1/api-keys`) is a
separate credential type: it authenticates one person against
everything their own console session can already do. See the docs site
[Security](https://zyvorai.github.io/yard/docs/security) page for both.

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
`GET /api/v1/sensors`) and pushes normalized records to Yard. Remote
devices do not need inbound ports.

The same sync path lives in `internal/connectors/deviceagent` and is
invoked from the console via `POST /api/v1/actions`:

| Action | Behavior |
| --- | --- |
| `inventory.refresh` | Pull agent inventory/sensors → Yard ingest |
| `diagnostics.read` | Return agent health/inventory JSON |

Configure the agent URL on the connector (`PATCH /api/v1/connectors` with
`endpoint`) or pass `{"agent_url":"..."}` in the action payload. For HTTPS
agents (lab self-signed TLS), Yard skips certificate verify by default;
set `config.tls_insecure` to `false` to require a trusted CA. Put the
agent bearer in `config.auth_token`. Dispatch needs `YARD_INGEST_TOKEN`
(or `YARD_INGEST_TOKEN_FILE` / `data/ingest.token`) because connector
tokens are stored hashed.

## Optional Zyvor connectors

| Connector | Yard responsibility | External responsibility |
| --- | --- | --- |
| Nodra | Pull devices/twins → inventory + numeric observations | Protocol interpretation, WAL, twins |
| Zyvor Fleet | Show sites, rollouts, OTA device lifecycle progress | Desired state, site reconciliation |
| OTA | List campaigns in the asset Integrations tab | Execute the update |

### Wiring (shipped)

1. **Integrations** page: set Endpoint + API bearer token (`config.auth_token`).
2. Run actions:
   - Nodra: `telemetry.receive` — lists `/api/v1/devices` + `/api/v1/twins`, upserts inventory, publishes numeric reported fields as observations.
   - Fleet: `lifecycle.request` / `desired.progress` — reads sites, rollouts, OTA devices.
   - OTA: `campaign.list` — Fleet `/api/v1/ota/devices` or Nodra campaigns/devices (`config.source` = `fleet`|`nodra`).
3. Asset detail **Integrations** tab: `GET /api/v1/assets/{id}/integrations`.
4. Nodra sync requires `YARD_INGEST_TOKEN` (or `data/ingest.token`) for Yard ingest POSTs.

Unknown connector kinds still return `unsupported`.

Each connector advertises supported actions. The UI offers executable actions for Device Agent, Nodra, Fleet, and OTA when an endpoint is set.

## MQTT adapter

A future adapter maps MQTT payloads onto the same HTTP ingest contract.
Topic and payload mapping belong in the adapter, not in the core schema.

## Related product surfaces

Bulk asset CSV/JSON import-export, MapLibre clustering, and incident
severity policies / runbooks are first-party Yard features (not connectors).
See the docs site [Console features](https://zyvorai.github.io/yard/docs/guides/console)
and [docs/ROADMAP.md](./ROADMAP.md).
