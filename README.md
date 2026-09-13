# Yard

**Open asset and operations platform.** Apache-2.0.

Yard is a standalone registry for physical operations: devices, vehicles,
machines, sensors, sites, telemetry, incidents, and work orders. Zyvor
products plug in as optional connectors. You can install Yard without
installing anything else in the Zyvor suite.

This is an original implementation. It is not a fork of Fleetbase
(AGPL-3.0) and contains no Fleetbase source.

## Why it exists

Logistics platforms optimize dispatch. Infrastructure platforms optimize
runtimes. Field teams still stitch device health, site context, and
maintenance work across three consoles.

Yard’s core object is an **asset**. A device is one asset kind. Vehicles
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
| Yard | Asset registry, sites, workflows, incidents, shared UI |
| Device Agent | Hardware discovery, health, local diagnostics |
| Nodra connector | Decoded industrial telemetry and buffered events |
| Zyvor Fleet connector | Lifecycle requests and progress |
| OTA connector | Campaign display; execution stays elsewhere |
| HTTP / simulator | Zero-dependency evaluation path |

Device Agent documents this split: it reports physical capability, Nodra
interprets protocols, Fleet owns desired state. Yard preserves those
lines and adds the operations surface.

## Quick start

```bash
go run ./cmd/yard
```

Open [http://127.0.0.1:8080](http://127.0.0.1:8080)

```
admin@yard.local
yard-admin
```

In another terminal:

```bash
export YARD_SIMULATOR_TOKEN=$(cat data/simulator.token)
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

Compose starts Yard and the included simulator. PostgreSQL is available as
an optional profile (keeps SQLite as the default for `go run` and CI):

```bash
docker compose --profile postgres up --build
# Yard on :8081 with YARD_DATABASE_URL=postgres://...
```

### Remote lab deploy

Same one-command pattern as Fabric/Nodra — cross-compile locally, install a
systemd unit on the host, verify `/healthz`:

```bash
./scripts/ship sus@HOST              # quick redeploy
./scripts/ship sus@HOST --full       # first install + firewall
./scripts/ship sus@HOST --with-sim   # also start the simulator
./scripts/ship sus@HOST --dry-run
```

Open `http://HOST:18080` (default lab port; override with `--port`, or reuse
`.deploy-last`). Demo login remains `admin@yard.local` / `yard-admin`.
Smoke from your laptop:

```bash
YARD_URL=http://HOST:18080 ./scripts/verify-remote.sh
```

### Device Agent gateway

```bash
export YARD_INGEST_TOKEN=$(cat data/ingest.token)
export DEVICE_AGENT_URL=http://127.0.0.1:9188
go run ./cmd/agent-gateway
```

The gateway reads the agent locally and publishes normalized inventory and
observations. From the Integrations console you can also run
`inventory.refresh` and `diagnostics.read` against a configured Device Agent
endpoint. It does not require inbound access to every remote device.

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
