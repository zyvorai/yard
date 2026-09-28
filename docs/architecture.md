# Architecture

Component boundaries, the runtime layout, the repository map and the data model.

[Back to the README](../README.md)

## Boundaries

| Component | Responsibility |
| --- | --- |
| **Yard** | Asset registry, sites, workflows, incidents, shared UI |
| **Device Agent** | Hardware discovery, health, local diagnostics |
| **Nodra connector** | Pull decoded telemetry / twins into ingest |
| **Zyvor Fleet connector** | Lifecycle / rollout / OTA-device progress |
| **OTA connector** | Campaign list display; execution stays elsewhere |
| **HTTP / simulator** | Zero-dependency evaluation path |

Device Agent reports physical capability. Nodra interprets protocols. Fleet owns desired state. Optional connectors sync when an endpoint and a stored secret are set ([docs/CONNECTORS.md](CONNECTORS.md)). Yard preserves those lines and adds the operations surface. Full contracts: [docs/CONNECTORS.md](CONNECTORS.md).

## Architecture

```text
                         Browser / curl
                               |
                               v
                    +----------------------+
                    |      cmd/yard        |
                    | API + embedded UI    |
                    | jobs + action worker |
                    +----------+-----------+
                               |
                    SQLite / Postgres
                               |
         +---------------------+---------------------+
         |                     |                     |
         v                     v                     v
   HTTP ingest           Included              Device Agent
   (observations /       simulator             gateway
    inventory / events)                        (optional)
         |
         +---- optional wired: Nodra · Fleet · OTA ----+
```

## Repository

```text
cmd/yard/              API server + embedded console
cmd/simulator/         included telemetry simulator
cmd/agent-gateway/     Device Agent → Yard ingest bridge
internal/api/          HTTP handlers, RBAC, sessions, secrets
internal/store/        SQLite / Postgres persistence and migrations
internal/jobs/         automations, stale ticker, action sweeper
internal/queue/        durable remote-action worker
internal/egress/       outbound URL policy
internal/secrets/      AES-GCM connector secrets
internal/config/       YARD_MODE and process settings
internal/seed/         demo or production bootstrap
internal/sse/          in-process live event hub
internal/connectors/   Device Agent + Nodra/Fleet/OTA sync dispatch
internal/platform/     ordered list of later programs (no runtime)
web/                   React/Vite console
website/               Docusaurus docs (GitHub Pages)
docs/ROADMAP.md        feature catalog (Have / Partial / Later)
docs/PHASES.md         programs 1–16 with shipped versus remaining scope
docs/CONNECTORS.md     ingest and connector contracts
docs/ux/               live lab screenshots
docs/social/           share / OG card
openapi.yaml           OpenAPI 3.0
sdk/go, sdk/ts         thin Go and TypeScript API clients
scripts/ship           remote lab deploy (Fabric-style)
scripts/backup.sh      SQLite/Postgres backup
scripts/restore.sh     restore a scripts/backup.sh snapshot
```

## Data model

Organization · User · Session · APIKey · Site · Location · Asset · Capability · Observation ·
Event · WorkOrder · Incident · SeverityPolicy · ActionRequest · Job ·
Connector · ConnectorSecret · Automation

Every observation stores **source**, **unit**, **observed_at**, **received_at**, **quality**, optional **quality_reason** / **uncertainty** / **calibration_state** / **sequence_num**, and a **value_kind** of number, bool, text, json, ref, enum, event, or histogram. Numeric rows older than 24 hours become hourly rollups. Set `YARD_MQTT_URL` to subscribe, or post Prometheus remote write and OTLP JSON to the ingest routes. Saved dashboards are panels of one asset and one capability. Severity policies map capability or automation matches to incident severity and runbook text. Connector actions are queued, expire, carry an idempotency key, and record an outcome when the worker finishes. Locations, templates, catalogs, and links are in the API and console.
