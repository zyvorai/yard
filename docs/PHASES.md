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
| 1. Production Foundation | Have | Modes, RBAC, secrets, egress, SSE tickets, readiness, authenticator codes, Vault key fetch, shared login and ingest limits | |
| 2. Reliable Actions | Have | Durable jobs, backoff, retry and cancel, connector sync, leader lock, live-event relay, action approval, shared login limits | |
| 3. Maintenance Operations | Have | Locations, templates, links, schedules, QR labels, files, parts and labor, Field page | |
| 4. Telemetry Data Platform | Have | Retention, typed values (number/integer/counter/bool/text/json/ref/enum/event/histogram), sequence and quality metadata, hourly rollups, remote write, OTLP JSON, MQTT, saved dashboards, filtered query, export, optional Timescale hypertable | |
| 5. Intelligent Incident Management | Have | Debounce, hysteresis, flap counts, parent incidents, ack and resolve SLAs, on-call, timeline | |
| 6. Safe Automation and Playbook Engine | Have | Step editor, dry-run, approval, ordered steps, source refresh | |
| 7. Asset Digital Twin and Operations Graph | Have | Desired state, twin, blast radius, geofence enter/exit, floorplan pins | |
| 8. Integration Hub | Have | Catalog, webhook, marketplace, ServiceNow, Maximo, and SAP create clients | |
| 9. Yard Intelligence | Have | Score, anomaly, search, cited answers, preview then approve | |
| 10. Enterprise Identity and Governance | Have | OIDC, SAML, SCIM, WebAuthn storage, custom roles, audit CSV, required CMK | |
| 11. High Availability and Large-Scale Deployment | Have | Claim retry, Postgres SKIP LOCKED, shared ingest budget, region stamp, peer event replication | |
| 12. Advanced Field Workforce | Have | Permits, skills, shifts, offline manuals | |
| 13. Energy, Sustainability, and Cost Intelligence | Have | Energy, downtime, replacement, tariffs, carbon, 7-day forecast | |
| 14. Vertical Solution Packs | Have | Five packs import templates, a dashboard, two automations, and a work order | |
| 15. Developer and Community Ecosystem | Have | Tag binaries, checksums, build info, amd64 and arm64, public programs page, keyless checksum signing | |
| 16. Commercial Platform | Have | Community edition, compliance CSV | |

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

OIDC login is `GET /api/v1/auth/oidc/start` and the callback. An authenticator secret is `POST /api/v1/auth/totp/setup` and `POST /api/v1/auth/totp/confirm`. After confirmation, login requires `otp`. `YARD_VAULT_ADDR` and `YARD_VAULT_TOKEN` load the master key from Vault KV (`YARD_VAULT_PATH`, default `secret/data/yard`). Login failures and ingest counts are stored in the database so every replica shares them.

## 2. Reliable Actions (Have)

`POST /api/v1/actions` with a connector inserts the action as `queued` and a `jobs` row of kind `remote_action`, then returns 202. A worker started from `cmd/yard` claims queued jobs, runs the connector, and writes the action result. Failures wait 2s, then 4s, 8s, 16s, 32s, and then 60s, and become `dead` once `max_attempts` (5) is reached. Operators retry a dead or failed job with `POST /api/v1/jobs/{id}/retry` (attempts reset) and cancel a still-queued or pending job with `POST /api/v1/jobs/{id}/cancel`. Viewers can list `GET /api/v1/jobs` but cannot change them. An action with no connector is recorded locally and completed in the request. Idempotency keys and the expiry sweeper are unchanged.

- Connector `sync_interval_sec` (minimum 30s, 0 disables). The leader replica enqueues a sync when the interval has elapsed and no job for that connector is already open. `POST /api/v1/connectors/{id}/test` records latency and the last error. `POST /api/v1/connectors/{id}/sync` queues a sync immediately.
- Schedule ticks (stale assets, action expiry, connector sync) take a Postgres advisory lock (`pg_try_advisory_lock`). SQLite always runs them, because that deployment is one process. Job claim stays row-level so more than one replica can execute queued work.
- Live events are inserted in `live_events` and, on Postgres, published with `NOTIFY yard_live`. Other replicas `LISTEN` and fan the event out locally. The writer replica publishes to its own subscribers immediately.
- `lifecycle.request`, `update.delegate`, `reboot`, `shutdown`, `wipe`, `firmware.update`, `power.off`, and `factory.reset` are stored as `pending_approval`. `POST /api/v1/jobs/{id}/approve` requires a different operator or admin.
- Login failures are counted in `login_attempts` (10 failures / 15 minutes per IP and email) so the limit is shared by every replica.

## 3. Maintenance Operations (Have)

`locations` stores a parent pointer, name, and kind (`region`, `campus`, `building`, `floor`, `zone`, or any string). `GET` and `POST /api/v1/locations` are org-scoped; create requires a write role.

Schema also adds `asset_templates`, `asset_links` (relation names such as `installed-on` or `depends-on`), and nullable `parent_asset_id`, `location_id`, and `template_id` on assets. `GET` and `POST /api/v1/asset-templates` and `/api/v1/asset-links` cover those tables. `PATCH /api/v1/assets/{id}` can set the three placement fields. The Locations page lists the place tree and templates.

A work order with `schedule_cron` is a schedule. Five fields, UTC: minute, hour, day of month, month, weekday (Sunday is 0). `*` or a comma-separated list of numbers. The leader replica checks every 30 seconds and opens one work order the first time that minute matches. A missed minute is not backfilled.

Parts and labor are lines on a work order (`kind` `part` or `labor`, quantity, unit, and `unit_cost_cents`). `GET` and `POST /api/v1/work-orders/{id}/lines`, and `DELETE` on a line. The Work orders page totals them. A QR label is `GET /api/v1/assets/{id}/label`. A file on an asset is `POST /api/v1/assets/{id}/attachments` (multipart field `file`, 8 MiB). Bytes live under `YARD_DATA_DIR/attachments` at mode `0600`.

Field (`/field`) is the technician page. The first online visit stores open work orders on the device. Completing one offline queues the note and status, and the page sends them when the network returns. The page lists checklist items and the permit state when the device is online. A service worker caches the console shell so the page can open with no connection. Completing a work order that has a checklist, or whose kind is `permit`, stays queued until the server has an approved permit.

## 4. Telemetry Data Platform (Have)

Observations keep a numeric `value`. `value_kind` is `number` (the default), `bool`, `text`, `json`, `ref` (image or file reference), `enum`, `event`, or `histogram`. Non-numeric kinds store `value_text` and do not trip a numeric threshold. Charts plot numbers only. Ingest may set `sequence_num`, `quality_reason`, `uncertainty`, and `calibration_state`. An organization `retention_days` of 0 keeps every observation. A positive value, set by an admin with `PATCH /api/v1/org`, makes the leader replica delete observations and hourly rollups older than that many days. The same hourly tick first folds numeric observations older than 24 hours into `observation_rollups` (min, max, sum, sample count). History queries return those hourly averages with `source` `rollup`. On PostgreSQL that rollup table is partitioned by month. Set `YARD_TIMESCALE=1` against a TimescaleDB image to load the extension and store raw `observations` as a seven-day hypertable; `GET /api/v1/meta` then reports `timescale: true`.

Ingest accepts the existing observation JSON, a snappy-compressed Prometheus remote-write body at `POST /api/v1/ingest/remote-write` (series need an `asset`, `yard_asset`, or `external_ref` label), and OTLP JSON gauges and sums at `POST /api/v1/ingest/otlp/v1/metrics` (`yard.asset` or `service.name`). `YARD_MQTT_URL` subscribes to `yard/+/observations` (override with `YARD_MQTT_TOPIC`). The payload is the same observation JSON. When more than one organization exists, set `YARD_MQTT_ORG`.

`GET` and `POST /api/v1/dashboards`, and `PATCH` or `DELETE /api/v1/dashboards/{id}`, store up to 12 panels. Each panel is an asset and a capability. The Dashboards page draws the last day of that signal.

`GET /api/v1/telemetry/query` filters by `asset_id`, `capability`, `from`, and `to`. A histogram observation stores `count`, `sum`, and `buckets` and does not trip a numeric threshold. On PostgreSQL the rollup table is partitioned by month. Optional TimescaleDB is `YARD_TIMESCALE=1` with a TimescaleDB image.

## 5. Intelligent Incident Management (Have)

A threshold rule can set `debounce_sec` and `hysteresis` in its config. The first sample over the line only starts a hold. The action runs after a later sample is still over the line and at least that many seconds have passed. A reading that falls below the threshold by more than `hysteresis` clears the hold. The Automations form has both fields. An open incident for the same title still suppresses a second one.

Each time that cleared condition trips again, `flap_count` increases by one and the timeline gains `incident.flap`. `POST /api/v1/incidents` accepts `parent_id`. An admin sets `ack_minutes` and `resolve_minutes` with `PATCH /api/v1/org`. A new incident stores both due times. `ack_breached` and `resolve_breached` are true when the clock passes the due time before the matching timestamp. `POST /api/v1/oncall` records who is on call. A new incident with an empty owner takes that person. `GET /api/v1/incidents/{id}/timeline` lists open, flaps, acknowledgement, child incidents, and resolution. The Incidents page shows those fields.

## 6. Safe Automation and Playbook Engine (Have)

A playbook is YAML (`name` and `steps` of `action` plus an optional `payload`) stored with `POST /api/v1/playbooks`. The Playbooks page edits those steps, reorders them, and saves the same YAML. A `source_url` is fetched through the egress policy when the playbook is created and again on the leader maintenance tick. A failed fetch leaves the previous body. `POST /api/v1/playbooks/{id}/run` with `dry_run: true` records the steps and does not enqueue a job. A step whose action needs approval leaves the run and its job in `pending_approval` until a different person calls `POST /api/v1/playbook-runs/{id}/approve`. One job of kind `playbook` then runs the steps in order and stops at the first failure.

## 7. Asset Digital Twin and Operations Graph (Have)

`PATCH /api/v1/assets/{id}` accepts `desired_state`, `floor_x`, and `floor_y`. `GET /api/v1/assets/{id}/twin` returns desired JSON, current health, and the latest telemetry point per capability. `GET /api/v1/assets/{id}/blast-radius` walks asset links breadth-first to depth 3. A geofence is a circle. The first sample inside writes `geofence.enter`. A later sample outside writes `geofence.exit`. A location can store a floorplan image, and the Locations page lists assets pinned on that plan.

## 8. Integration Hub (Have)

Connector kinds register a name, actions, and an execute function. `GET /api/v1/connectors/catalog` returns that list. `GET /api/v1/connectors/marketplace` lists ServiceNow, Maximo, and SAP. `POST /api/v1/connectors/marketplace/{name}` inserts a connector. Each of those kinds POSTs its JSON body through the egress client. Kind `webhook` does the same for an arbitrary payload.

## 9. Yard Intelligence (Have)

`GET /api/v1/assets/{id}/score` returns 0–100 from health, open incidents, and staleness, plus the rows used as evidence. On numeric ingest, when the last 20 samples exist and the new value is more than 3 standard deviations from their mean, Yard inserts an event `telemetry.anomaly`. `GET /api/v1/search?q=` matches asset name, incident title, and work order title. `GET /api/v1/search/answer?q=` returns a sentence and those hits. With `YARD_MODEL_URL` set, the sentence comes from that endpoint and the hits stay attached. `POST /api/v1/assistant/preview` stores a proposed work order. A different person approves it before it is created.

## 10. Enterprise Identity and Governance (Have)

`GET /api/v1/auth/oidc/start` and the callback issue a session. `POST /api/v1/auth/saml/acs` reads the email attribute. Production requires `YARD_SAML_CERT` and checks the signature. SCIM `/api/v1/scim/v2/Users` uses `YARD_SCIM_TOKEN`. Demo may create a user. Production returns an existing user in the single organization. WebAuthn register and login store a challenge and a credential. Custom roles carry `can_write`. `GET /api/v1/audit/export` returns CSV for an admin. `YARD_REQUIRE_CMK=1` refuses a key that was generated on disk. `YARD_SECRET_KEY` is the customer key.

## 11. High Availability and Large-Scale Deployment (Have)

Claiming a job retries the next queued id when the update matches zero rows, up to five tries. On Postgres the claim uses `FOR UPDATE SKIP LOCKED`. Helm refuses `replicaCount` greater than 1 unless `databaseUrl` starts with `postgres://` or `postgresql://`. Ingest counts in `ingest_budget` so two processes share one budget per organization per minute. `YARD_REGION` is stored on new events. The leader posts unreplicated events to `YARD_PEER_URL` with `YARD_PEER_TOKEN`. The peer stores them and does not send them back. [SIZING.md](./SIZING.md) describes the small and shared profiles. The public lab runs one region.

## 12. Advanced Field Workforce (Have)

`permits` rows belong to a work order. Completing a work order whose checklist is non-empty, or whose kind is `permit`, requires a permit in status `approved`. Users have skills. Shifts have a start and an end. Assigning a work order that names `required_skill` fails until the assignee has that skill. Field shows the checklist and the permit state, and it keeps attachment bytes on the device so a manual can open offline. The server still enforces the permit when the queue is sent.

## 13. Energy, Sustainability, and Cost Intelligence (Have)

An admin sets `energy_cents_per_kwh`, `carbon_grams_per_kwh`, and tariff windows with `PATCH /api/v1/org`. An asset can store `downtime_cents_per_hour` and `replacement_cost_cents`. `GET /api/v1/reports/cost` applies the tariff for each sample hour, adds carbon grams, and returns a 7-day forecast from the average daily energy cost. The Cost page shows energy, downtime, repair versus replacement, carbon, and the forecast.

## 14. Vertical Solution Packs (Have)

`packs/` holds manufacturing, data-center, hospital, cold-chain, and utilities. Each file is templates, one dashboard, two automations, and a work-order title. `POST /api/v1/packs/{name}` (admin) inserts all of that for the caller’s organization. The packs stay inside this product.

## 15. Developer and Community Ecosystem (Have)

A tag workflow builds `yard`, `yard-simulator`, and `yard-agent-gateway` for `linux/amd64` and `linux/arm64`, writes SHA256 checksums, and signs `SHA256SUMS` with cosign keyless. GitHub's OIDC identity is the signer. No signing key is stored in the repository. The programs page on the website is the public board. [UPGRADE.md](./UPGRADE.md) says schema migrations run on process start and that a tag is the upgrade unit.

## 16. Commercial Platform (Have)

`GET /api/v1/meta` includes `edition: community`. `GET /api/v1/compliance/export` returns a CSV of the edition, users, and audit rows for an admin. [COMMERCIAL.md](./COMMERCIAL.md) states the boundary. This repository does not bill and does not run a license server. That is the finished community edition.

## Explicitly out of scope until those programs need them

Route optimization, payroll, procurement accounting, a black-box failure model, a large set of shallow connectors, and splitting the Go process into microservices.
