# Estate

**Open asset and operations platform.** Apache-2.0.

Estate is a standalone registry for physical operations: devices, vehicles,
machines, sensors, sites, telemetry, incidents, and work orders. Zyvor
products plug in as optional connectors. You can install Estate without
installing anything else in the Zyvor suite.

This is an original implementation. It is not a fork of Fleetbase
(AGPL-3.0) and contains no Fleetbase source.

## Why it exists

Logistics platforms optimize dispatch. Infrastructure platforms optimize
runtimes. Field teams still stitch device health, site context, and
maintenance work across three consoles.

Estate’s core object is an **asset**. A device is one asset kind. Vehicles
and route optimization can arrive later as an optional logistics
extension — they do not own the data model.

| Module | What users can do |
| --- | --- |
| Overview | Asset health, active work, incidents, recent activity |
| Assets | Register devices, vehicles, machines, sensors, equipment |
| Sites | Factories, warehouses, offices, customer locations |
| Map | Known locations and stale-or-live status |
| Telemetry | Measurements, freshness, quality, threshold context |
| Work orders | Inspections, repairs, installations, maintenance |
| Incidents | Acknowledge, assign, resolve |
| Automations | Notifications and approved actions from events |
| Integrations | Agents, MQTT-ready HTTP contracts, optional Zyvor connectors |
| Administration | Workspace, credentials, audit history |

## Boundaries

| Component | Responsibility |
| --- | --- |
| Estate | Asset registry, sites, workflows, incidents, shared UI |
| Device Agent | Hardware discovery, health, local diagnostics |
| Nodra connector | Decoded industrial telemetry and buffered events |
| Zyvor Fleet connector | Lifecycle requests and progress |
| OTA connector | Campaign display; execution stays elsewhere |
| HTTP / simulator | Zero-dependency evaluation path |

Device Agent documents this split: it reports physical capability, Nodra
interprets protocols, Fleet owns desired state. Estate preserves those
lines and adds the operations surface.

## Quick start

```bash
go run ./cmd/estate
```

Open [http://127.0.0.1:8080](http://127.0.0.1:8080)

```
admin@estate.local
estate-admin
```

In another terminal:

```bash
export ESTATE_SIMULATOR_TOKEN=$(cat data/simulator.token)
go run ./cmd/simulator
```

First-use path: workspace (seeded) → simulator or Device Agent gateway →
discover assets → inspect health → temperature / missed heartbeat opens an
incident → assign a work order → record resolution on the asset timeline.

To develop the console separately:

```bash
cd web && npm install && npm run dev
```

Vite proxies `/api` to `:8080`.

### Docker Compose

```bash
docker compose up --build
```

Compose starts Estate and the included simulator. PostgreSQL + PostGIS is
the documented production target; the first release ships a SQLite engine
so `go run` and CI work with no extra services. The schema is portable.

### Device Agent gateway

```bash
export ESTATE_INGEST_TOKEN=$(cat data/ingest.token)
export DEVICE_AGENT_URL=http://127.0.0.1:9188
go run ./cmd/agent-gateway
```

The gateway reads the agent locally and publishes normalized inventory and
observations. It does not require inbound access to every remote device.

## Data model

Organization · Site · Asset · Capability · Observation · Event ·
WorkOrder · Incident · ActionRequest · Connector · Automation

Every observation stores **source**, **unit**, **observed_at**,
**received_at**, and **quality**. Offline data is marked stale rather than
healthy.

Remote actions require a session, expire, carry an idempotency key, and
record an outcome. Connectors advertise the actions they can execute.

## Interface

Apple-inspired, original identity:

- White and soft-gray surfaces, dark type, generous spacing
- Orange (`#FF5A1F`) only for the Zyvor mark, primary actions, and selected states
- Compact labeled sidebar
- Asset detail panel that does not replace the list
- Dark, searchable diagnostics
- System fonts (no CDN), visible focus, reduced-motion support

## Tests

```bash
go test ./...
cd web && npm test
```

Release gates covered in tests: tenant isolation, connector authentication,
duplicate observations, stale telemetry, and the browser-equivalent
health → incident → work order → resolve workflow.

## License

Apache License 2.0. See `LICENSE` and `NOTICE`.
