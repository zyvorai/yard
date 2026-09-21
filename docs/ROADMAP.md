# Yard roadmap

Yard is Zyvor’s open **asset and operations** platform. The core object is
the **asset**. Device Agent, Nodra, Fleet, and OTA remain optional
connectors — Yard must stay useful alone.

This document is the product feature catalog.
Statuses: **Have** (shipped) · **Partial** (a real slice, exit criteria not met) · **Later** (not started).

Program-level detail, including what is still missing, is [PHASES.md](./PHASES.md).

## Shipped epics

### Epic 1 — Live ops loop ✅

Make health and incidents feel live without refreshing Overview.

- ~~Background stale ticker in the Yard process~~ **Have**
- ~~Subscribe the console to `GET /api/v1/stream` (SSE)~~ **Have**
- ~~Live Overview counters and Map pin health updates~~ **Have**
- ~~Optional browser notification for new critical incidents~~ **Have**

### Epic 2 — Registry completeness ✅

Close the gap between API and console for day-to-day registry work.

- ~~Create / edit asset UI~~ **Have**
- ~~Fill Asset detail **Activity** and **Work** tabs~~ **Have**
- ~~Site edit / delete~~ **Have**
- ~~Standalone work-order create UI~~ **Have**

### Epic 3 — Bulk ops, map scale, incident playbooks ✅

- ~~Bulk asset import/export (CSV/JSON)~~ **Have**
- ~~MapLibre pin clustering~~ **Have**
- ~~Incident severity policies + runbooks~~ **Have**

Earlier follow-ons in the same lane: automation create/delete + webhook
action, telemetry sparklines, write RBAC, `/metrics`, ingest rate limit,
OpenAPI expand, onboarding, command palette, tablet nav.

```mermaid
flowchart TD
  E1[Epic1 LiveOpsLoop] --> E2[Epic2 RegistryCompleteness]
  E2 --> E3[Epic3 BulkMapRunbooks]
  E3 --> E4[TelemetryTimeRange]
  E4 --> E5[RBACInviteAndSDKs]
  E5 --> E6[NodraThenFleetOTA]
```

---

## 1. Asset registry

| Feature | Status |
| --- | --- |
| Asset list with search / kind / health filters | Have |
| Asset detail (overview + recent telemetry) | Have |
| Create asset via API | Have |
| Create asset UI form | Have |
| Edit / archive / delete asset | Have (DELETE soft-deletes; list omits the row) |
| Asset kinds: device, sensor, machine, vehicle, equipment (+ custom) | Have |
| Capabilities (signals, units, thresholds, writable) | Have (model + UI editor) |
| External refs + manufacturer / model / serial / metadata | Have |
| Asset relationships (parent/child, install history) | Have (`asset-links`, `parent_asset_id`, `GET .../installs`) |
| Bulk import/export (CSV/JSON) | Have (`/api/v1/assets/export`, `/import` + Assets UI) |
| Asset barcode / QR / NFC identity | Have (QR label PNG; lookup by `q` or `nfc`) |
| Spare parts / BOM linked to asset | Have (`GET`/`POST /api/v1/assets/{id}/bom`) |
| Warranties, purchase date, depreciation | Have (`purchased_at`, `warranty_expires_at`, book value from useful life) |
| Documents / photos / manuals attached to asset | Have (8 MiB file on the asset, stored outside the database) |
| Asset templates / catalogs | Have (templates plus `GET`/`POST /api/v1/catalogs`) |

## 2. Sites and geography

| Feature | Status |
| --- | --- |
| Site list + create | Have |
| Lat/lng on sites and assets | Have |
| MapLibre full-bleed map, filters, detail card, dark basemap | Have |
| Site edit / delete | Have |
| Site hierarchy (campus → building → floor → zone) | Have (`locations` with parent and kind) |
| Geofences + enter/exit events | Have (circle geofence writes `geofence.enter` and `geofence.exit`) |
| Indoor maps / floorplans | Have (location image plus asset `floor_x` / `floor_y` pins) |
| Clustering at large pin counts | Have (MapLibre cluster layers) |
| Directions / routing between sites | Later (logistics; see section 11) |
| Address geocoding | Have (`GET /api/v1/geocode?q=`) |

## 3. Telemetry and health

| Feature | Status |
| --- | --- |
| HTTP ingest observations (dedupe, quality, source, timestamps) | Have |
| Inventory ingest → upsert asset + capabilities | Have |
| Health derivation (temp / heartbeat / quality) | Have |
| Stale marking + missed-heartbeat incidents | Have |
| Background stale ticker | Have |
| Telemetry latest table | Have |
| Historical charts / sparklines / time-range query | Have |
| Multi-signal dashboards per asset | Have |
| Capability min/max enforcement + soft/hard alarms | Have (`capability_min`/`capability_max` automation triggers) |
| Metric retention / downsampling | Have (admin `retention_days`; numeric rows older than 24h become hourly rollups) |
| Typed values (number, bool, text) | Have (`value_kind` / `value_text`; charts stay numeric) |
| Prometheus remote write and OTLP JSON ingest | Have |
| MQTT subscriber | Have when `YARD_MQTT_URL` is set |
| Saved dashboards | Have |
| Anomaly detection | Have (event `telemetry.anomaly` when a numeric sample is beyond 3 standard deviations of the prior 20) |
| Export telemetry (CSV, Prometheus, OTLP) | Have (`GET /api/v1/telemetry/export?format=`) |

## 4. Incidents and work orders

| Feature | Status |
| --- | --- |
| Open / ack / assign / resolve incidents | Have |
| Auto-open from threshold + stale automations | Have |
| Threshold debounce and hysteresis | Have (`debounce_sec` and `hysteresis` on the rule) |
| Flapping count | Have (one count each time a cleared threshold trips again) |
| Parent incidents | Have (`parent_id` on create) |
| Ack and resolve times | Have (`ack_minutes` and `resolve_minutes` on the organization) |
| On-call | Have (`POST /api/v1/oncall` fills an empty owner) |
| Incident timeline | Have (`GET /api/v1/incidents/{id}/timeline`) |
| Create work order from incident | Have |
| Work order list + mark done | Have |
| Work order create UI (standalone) | Have |
| Richer priorities / due dates / assignees UI | Have |
| Incident severity policies + runbooks | Have (Admin policies; runbook on incident detail) |
| SLA timers / escalation | Have (`sla_due_at` on work orders; org ack/resolve minutes on incidents) |
| Checklists / procedures on work orders | Have (`checklist` JSON on work orders) |
| Parts used + time tracking | Have (work-order lines plus `POST .../time`) |
| Mobile field tech mode (PWA) | Have (`/field` caches open work orders, checklists, permit state, and manual bytes) |
| Multi-asset work orders | Have (`PUT /api/v1/work-orders/{id}/assets`) |
| Calendar / preventive maintenance schedules | Have (`schedule_cron` and `GET /api/v1/schedules`) |

## 5. Automations and actions

| Feature | Status |
| --- | --- |
| List + enable/disable automations | Have |
| Threshold → open incident | Have |
| Stale heartbeat → open incident | Have |
| Notify-on-event automation | Have (notify + audit) |
| Automation rule editor | Have (create / enable / delete) |
| Webhook / email / Slack / PagerDuty actions | Have (email/Slack/PagerDuty implemented, unverified — no test account) |
| Remote actions with idempotency + expiry | Have (durable jobs, backoff, retry, cancel, approval for dangerous actions) |
| Device Agent `inventory.refresh` / `diagnostics.read` | Have |
| Two-person action approval | Have (dangerous actions and playbook runs wait for a different person) |
| Multi-step playbooks | Have (YAML steps, step editor, dry-run, one job, source_url refresh) |

## 6. Integrations and connectors

| Feature | Status |
| --- | --- |
| HTTP ingest connector + token rotate | Have |
| Included simulator | Have |
| Device Agent gateway + console actions | Have |
| Connector catalog UI | Have |
| Nodra decoded telemetry ingest | Have (`telemetry.receive` sync → ingest) |
| Zyvor Fleet lifecycle / desired-state display | Have (`desired.progress` + asset Integrations tab) |
| OTA campaign list / update delegate (display) | Have (`campaign.list` via Fleet or Nodra) |
| MQTT adapter → HTTP ingest | Have (`YARD_MQTT_URL`, topic `yard/+/observations`) |
| Webhook egress (Yard → customer systems) | Have |
| Complete OpenAPI + SDKs | Have (full route/schema coverage; hand-authored Go + TypeScript clients in `sdk/`) |
| OPC-UA / Modbus | Boundary (Nodra) |
| SAP / Maximo / ServiceNow sync | Have (`sync` GET through the egress client on those kinds) |
| OAuth / mTLS for connectors | Have (`oauth_token_url` client credentials; `client_cert_pem` / `client_key_pem`) |

See also [CONNECTORS.md](./CONNECTORS.md).

## 7. Realtime and overview

| Feature | Status |
| --- | --- |
| Overview health / incidents / work / activity | Have |
| SSE stream API | Have (single-use ticket; session token in the URL is rejected) |
| UI subscribe to SSE | Have |
| Onboarding wizard (`/onboarding`) | Have |
| Global command palette | Have |
| Browser notifications for critical incidents | Have |
| Customizable overview widgets | Have (`PATCH /api/v1/me/prefs` widgets list) |
| Multi-workspace switcher | Have (`GET /api/v1/orgs` and `POST /api/v1/orgs/switch`) |

## 8. Administration, security, tenancy

| Feature | Status |
| --- | --- |
| Login sessions (bcrypt) | Have |
| Audit log + Admin token rotate | Have |
| Severity policy editor (Admin) | Have |
| Org-scoped queries / isolation tests | Have |
| Roles / RBAC (viewer, operator, admin) | Have (incidents, connectors, actions, and audit included) |
| Invite users / password reset | Have (SMTP in production; demo may log the link) |
| API keys for humans vs connectors | Have |
| Production bootstrap (`YARD_MODE`) | Have |
| Login rate limit, logout, session revoke | Have (limiter is shared in the database) |
| Encrypted connector secrets | Have |
| Outbound egress policy | Have |
| Readiness (`/readyz` checks the database) | Have |
| SSO (OIDC/SAML) | Have (OIDC callback, SAML ACS with a certificate check, SCIM users) |
| Multi-tenant product UX | Have (org membership list and switch) |
| Soft-delete + retention | Have (asset soft-delete; observation `retention_days`) |
| Secrets vault for connector credentials | Have (AES-GCM at rest; optional `YARD_VAULT_ADDR` for the master key) |
| Compliance exports | Have (`GET /api/v1/compliance/export`) |

## 9. UX / product surfaces

| Feature | Status |
| --- | --- |
| Apple-inspired light UI + Zyvor mark | Have |
| Dark theme toggle | Have |
| Map full-bleed glass UI | Have |
| Asset Activity / Work / Integrations tabs filled | Have (Activity / Work) |
| Empty states + first-run guided demo | Have |
| Accessibility audit (WCAG) | Have (focus-visible rings, skip-to-content link, dialog focus trap + Escape via shared `useDialogA11y` hook) |
| Responsive / tablet field layout | Have (bottom nav) |
| i18n | Have (`GET /api/v1/i18n?lang=` and user locale preference) |
| Printable WO / incident reports | Have (`GET /api/v1/work-orders/{id}/print`) |
| Saved views / filters | Have (`GET`/`POST /api/v1/saved-views`) |

## 10. Platform, data, deploy

| Feature | Status |
| --- | --- |
| SQLite local / CI | Have |
| Postgres DSN path | Have |
| Docker Compose + ship scripts | Have |
| Backups / restore tooling | Have (`scripts/backup.sh`/`restore.sh`, SQLite + Postgres) |
| Metrics (`/metrics`) + structured logging | Have |
| Rate limits on ingest | Have |
| Migration versioning | Have (`schema_migrations` + versioned migration runner) |
| Map bounding-box queries | Have (`GET /api/v1/assets?bbox=minLng,minLat,maxLng,maxLat`) |
| Horizontal replicas + shared Postgres | Have (SKIP LOCKED claim, shared ingest budget, `YARD_REGION`, peer replication) |
| Helm / k8s deploy | Have (`deploy/helm/yard`) |
| Multi-region | Have (`YARD_PEER_URL` forwards events once; the public lab runs one region) |

## 11. Logistics extension (deferred)

Keep off the core model until explicitly requested:

- Routes, stops, ETAs, drivers
- Dispatch board / optimization
- Proof of delivery
- Vehicle telematics as a first-class fleet module (vehicle remains an asset kind)

## What Yard will not own

- Protocol decode → **Nodra**
- Desired-state reconciliation → **Zyvor Fleet**
- Firmware execution → **OTA**
- Hypervisor / k8s control plane → **Fabric / Axiom**
- Full CMMS / ERP replacement on day one

Yard stays the **ops surface + registry**. Depth comes from connectors and
the programs in [PHASES.md](./PHASES.md).
