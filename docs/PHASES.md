# Yard programs

Yard is the open operations system for physical and edge assets: inventory, telemetry, maintenance, incidents, remote actions, and later AI-assisted diagnosis. It is not a generic fleet manager and not an ERP. The advantage is the link between assets and Zyvor’s edge stack.

Statuses used below:

- **Have** — shipped in this repository and covered by tests or the console.
- **Partial** — a real slice exists; the exit criteria are not met.
- **Planned** — documented only. `internal/platform` lists the program order so CI can see the sequence. There is no product behavior behind these yet.

Closing loop for the product:

> Detect → understand → approve → act → verify → maintain → learn

## What is in the tree today

| Program | Status | What exists | What is still missing |
| --- | --- | --- | --- |
| 1. Production Foundation | Have | Modes, RBAC, secrets, egress, SSE tickets, readiness | OIDC login, MFA, Vault, distributed rate limits |
| 2. Reliable Actions | Have | Durable jobs, backoff, retry and cancel, connector sync, leader lock, live-event relay, action approval, shared login limits | Playbook approvals stay in program 6 |
| 3. Maintenance Operations | Partial | Locations, templates, links, cron schedules, QR labels, asset files | Parts and labor, technician PWA |
| 4–16 | Planned | Names and order in `internal/platform` | All of the behavior described in those phases |

The public lab at `http://175.110.122.71:18081` runs **demo** mode. Demo credentials are valid there on purpose. A production install must set `YARD_MODE=production` (see [SECURITY.md](../SECURITY.md)).

## 1. Production Foundation (Have)

Goal: a fresh production process cannot be operated with the public demo password, and viewers cannot mutate incidents, connectors, or remote actions.

### Runtime modes

`YARD_MODE=demo` is the default. An empty database gets the Northwind sample workspace, sites, assets, connectors, and `admin@yard.local` / `yard-admin`. Ingest and simulator tokens are written under `YARD_DATA_DIR` (mode `0600`). The welcome log line does not include the password or the raw tokens.

`YARD_MODE=production` refuses to start unless `YARD_PUBLIC_URL` and `YARD_SECRET_KEY` are set. `YARD_SECRET_KEY` is 32 bytes, standard base64 or hex. On an empty database it also requires `YARD_BOOTSTRAP_EMAIL` and `YARD_BOOTSTRAP_PASSWORD`. That password cannot be `yard-admin`, and the email cannot be `admin@yard.local`. No sample sites or assets are inserted. If a database already contains `admin@yard.local` and that account still matches `yard-admin`, the process exits.

`GET /api/v1/meta` is unauthenticated and returns `mode`, whether SMTP is configured, and the public URL. The login page prefills the demo password only when `mode` is `demo`.

### Authorization

Write access is `admin` or `operator`. Viewers are rejected on:

- `POST` and `PATCH /api/v1/incidents` (resolve is `PATCH` only)
- `PATCH /api/v1/connectors` and `PUT /api/v1/connectors/{id}/secret`
- `POST /api/v1/actions` and `POST /api/v1/locations`
- Existing write routes for sites, assets, work orders, automations, and severity policies
- `GET /api/v1/audit`

Overview no longer marks assets stale as a side effect of a read. The background ticker still does that. Recent activity on Overview is empty for viewers. Self-service API keys stay available to every role; the key inherits the caller.

### Connector secrets and egress

`auth_token` and `token` are not stored in connector `config` once the API has seen them. They move into `connector_secrets` as AES-256-GCM ciphertext under the master key. List and update responses include `has_secret` and `secret_hint` only. Putting a token in a connector PATCH body returns 400. Action payloads cannot replace the stored token. In production they also cannot replace the connector URL.

TLS verification is on unless the connector config sets `tls_insecure: true` and either the mode is `demo` or `YARD_ALLOW_INSECURE_TLS=1`. Production rejects insecure TLS on connector update otherwise.

Outbound HTTP from Device Agent, Nodra, Fleet, OTA, and automation webhooks uses one policy: `http` or `https` only, no redirects, response bodies capped at 1 MiB, and DNS pinned to the addresses that were checked. Link-local and cloud metadata addresses are always blocked. Production also blocks loopback and private ranges unless the host is in `YARD_EGRESS_ALLOWLIST`. Demo allows private addresses so a Device Agent on `127.0.0.1:9188` still works.

### Sessions, mail, and HTTP limits

- Login failures are limited per client address and email (about 10 per 15 minutes, then 429 with `Retry-After`). The limiter is in-process.
- `POST /api/v1/auth/logout` deletes the current session.
- `GET /api/v1/auth/sessions` lists the caller’s sessions. `DELETE` on that collection revokes all of them. `DELETE /api/v1/auth/sessions/{id}` revokes one. Password reset revokes that user’s sessions.
- Invites and password resets are emailed when `YARD_SMTP_HOST` is set, and the token is not written to the log. Demo mode without SMTP still logs the link. Production without SMTP returns 503 and does not log a token.
- Live updates: `POST /api/v1/stream/ticket` (Bearer session) returns a single-use ticket of about 60 seconds. `GET /api/v1/stream?ticket=` consumes it. `?token=` on the stream is rejected.
- CORS: unset demo mode sends `Access-Control-Allow-Origin: *`. Unset production sends no allow-origin header. `YARD_CORS_ORIGINS` is a comma-separated allowlist.
- Responses set `Content-Security-Policy`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, and `Referrer-Policy: no-referrer`. `Strict-Transport-Security` is set when `YARD_PUBLIC_URL` is `https`.
- The HTTP server sets read, idle, and header limits. Write timeout stays unset so SSE is not cut off. JSON bodies are capped at 1 MiB; asset import stays at 8 MiB.
- `/healthz` is process liveness. `/readyz` pings the database and checks `schema_migrations` against the binary’s latest version. Helm readiness uses `/readyz`. `/metrics` includes `yard_build_info` plus login-lockout, secret-rotation, and egress-denial counters.

OIDC remains discovery only (`GET /api/v1/auth/oidc`). There is no authorization-code callback.

## 2. Reliable Actions (Have)

`POST /api/v1/actions` with a connector inserts the action as `queued` and a `jobs` row of kind `remote_action`, then returns 202. A worker started from `cmd/yard` claims queued jobs, runs the connector, and writes the action result. Failures wait 2s, then 4s, 8s, 16s, 32s, and then 60s, and become `dead` once `max_attempts` (5) is reached. Operators retry a dead or failed job with `POST /api/v1/jobs/{id}/retry` (attempts reset) and cancel a still-queued or pending job with `POST /api/v1/jobs/{id}/cancel`. Viewers can list `GET /api/v1/jobs` but cannot change them. An action with no connector is recorded locally and completed in the request. Idempotency keys and the expiry sweeper are unchanged.

- Connector `sync_interval_sec` (minimum 30s, 0 disables). The leader replica enqueues a sync when the interval has elapsed and no job for that connector is already open. `POST /api/v1/connectors/{id}/test` records latency and the last error. `POST /api/v1/connectors/{id}/sync` queues a sync immediately.
- Schedule ticks (stale assets, action expiry, connector sync) take a Postgres advisory lock (`pg_try_advisory_lock`). SQLite always runs them, because that deployment is one process. Job claim stays row-level so more than one replica can execute queued work.
- Live events are inserted in `live_events` and, on Postgres, published with `NOTIFY yard_live`. Other replicas `LISTEN` and fan the event out locally. The writer replica publishes to its own subscribers immediately.
- `lifecycle.request`, `update.delegate`, `reboot`, `shutdown`, `wipe`, `firmware.update`, `power.off`, and `factory.reset` are stored as `pending_approval`. `POST /api/v1/jobs/{id}/approve` requires a different operator or admin.
- Login failures are counted in `login_attempts` (10 failures / 15 minutes per IP and email) so the limit is shared by every replica.

## 3. Maintenance Operations (Partial)

`locations` stores a parent pointer, name, and kind (`region`, `campus`, `building`, `floor`, `zone`, or any string). `GET` and `POST /api/v1/locations` are org-scoped; create requires a write role.

Schema also adds `asset_templates`, `asset_links` (relation names such as `installed-on` or `depends-on`), and nullable `parent_asset_id`, `location_id`, and `template_id` on assets. `GET` and `POST /api/v1/asset-templates` and `/api/v1/asset-links` cover those tables. `PATCH /api/v1/assets/{id}` can set the three placement fields. The Locations page lists the place tree and templates.

A work order with `schedule_cron` is a schedule. Five fields, UTC: minute, hour, day of month, month, weekday (Sunday is 0). `*` or a comma-separated list of numbers. The leader replica checks every 30 seconds and opens one work order the first time that minute matches. A missed minute is not backfilled.

Not built yet: parts and labor, and an offline technician app. A QR label is `GET /api/v1/assets/{id}/label`. A file on an asset is `POST /api/v1/assets/{id}/attachments` (multipart field `file`, 8 MiB). Bytes live under `YARD_DATA_DIR/attachments` at mode `0600`. List and download stay on that asset; delete requires a write role. The Assets page shows the label and the files.

## 4. Telemetry Data Platform (Planned)

Typed values beyond the current numeric `observations.value`, retention and downsampling, partitioned or Timescale storage, MQTT / remote-write / OTLP ingest, and a dashboard builder. Observations today are float values with quality, source, and timestamps. Historical charts exist per asset. There is no retention policy and no wallboard.

## 5. Intelligent Incident Management (Planned)

Hysteresis, debounce, flapping, parent incidents, acknowledgement and resolution SLAs, on-call rotations, and a command-center timeline. Today an automation opens one incident and a severity policy attaches a runbook. `sla_due_at` is a timestamp on the work order, not an escalation engine.

## 6. Safe Automation and Playbook Engine (Planned)

Visual multi-step playbooks, dry-run, two-person approval, and Git-backed YAML. Today automations are a single trigger and a single action (incident, notify, webhook, Slack, email, or PagerDuty).

## 7. Asset Digital Twin and Operations Graph (Planned)

Reported versus desired state, a relationship graph with blast radius, indoor maps, and geofences. The location and link tables in program 3 are the storage start, not this experience.

## 8. Integration Hub (Planned)

A connector SDK and marketplace, plus enterprise systems (ServiceNow, Maximo, SAP). Community connectors today are HTTP ingest, the simulator, Device Agent, Nodra, Fleet, and OTA, all compiled into the Yard binary.

## 9. Yard Intelligence (Planned)

Explainable health scores, statistical anomaly detection, incident summaries that cite evidence, and natural-language search. Mutating answers must preview and require authorization. No model is called from Yard today.

## 10. Enterprise Identity and Governance (Planned)

Full OIDC, SAML, SCIM, WebAuthn, custom roles, immutable audit export, and customer-managed keys. Local users, three roles, encrypted connector secrets, and OIDC discovery are what exist now.

## 11. High Availability and Large-Scale Deployment (Planned)

Stateless replicas, a shared job bus, Postgres HA, and published sizing profiles. The process is a modular monolith. The SSE hub, ingest limiter, stale ticker, and metrics are still local to one process. Helm runs a single replica by default.

## 12. Advanced Field Workforce (Planned)

Skills, shifts, permits, inspection forms, and offline manuals, still tied to asset maintenance rather than logistics dispatch.

## 13. Energy, Sustainability, and Cost Intelligence (Planned)

Energy, downtime cost, and repair-versus-replace views. No cost fields exist on work orders yet.

## 14. Vertical Solution Packs (Planned)

Manufacturing, data-center and edge, hospital facilities, cold-chain, and utilities packs as templates and dashboards inside one Yard, not forks.

## 15. Developer and Community Ecosystem (Planned)

Versioned releases, signed multi-arch images, SBOM, a public roadmap board, and upgrade notes. CI already builds Go, Postgres, backups, the web console, the docs site, and Helm. There is no release automation in this repository yet.

## 16. Commercial Platform (Planned)

Apache-2.0 core stays the operational product in this repo. Enterprise clustering, SSO, long-term telemetry, AI, and compliance exports are a separate license when they exist. This repository does not implement billing.

## Explicitly out of scope until those programs need them

Route optimization, payroll, procurement accounting, a black-box failure model, a large set of shallow connectors, and splitting the Go process into microservices.
