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
| Edit / archive / delete asset | Have |
| Asset kinds: device, sensor, machine, vehicle, equipment (+ custom) | Have |
| Capabilities (signals, units, thresholds, writable) | Have (model + UI editor) |
| External refs + manufacturer / model / serial / metadata | Have |
| Asset relationships (parent/child, install history) | Partial (`POST /api/v1/asset-links` and `parent_asset_id`; install history later) |
| Bulk import/export (CSV/JSON) | Have (`/api/v1/assets/export`, `/import` + Assets UI) |
| Asset barcode / QR / NFC identity | Partial (QR label PNG and lookup; NFC later) |
| Spare parts / BOM linked to asset | Partial (part and labor lines on a work order; a stocked BOM later) |
| Warranties, purchase date, depreciation | Later |
| Documents / photos / manuals attached to asset | Have (8 MiB file on the asset, stored outside the database) |
| Asset templates / catalogs | Partial (`GET`/`POST /api/v1/asset-templates` and the Locations page; catalogs later) |

## 2. Sites and geography

| Feature | Status |
| --- | --- |
| Site list + create | Have |
| Lat/lng on sites and assets | Have |
| MapLibre full-bleed map, filters, detail card, dark basemap | Have |
| Site edit / delete | Have |
| Site hierarchy (campus → building → floor → zone) | Partial (`GET`/`POST /api/v1/locations` and the Locations page) |
| Geofences + enter/exit events | Later |
| Indoor maps / floorplans | Later |
| Clustering at large pin counts | Have (MapLibre cluster layers) |
| Directions / routing between sites | Later (logistics) |
| Address geocoding | Later |

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
| Anomaly detection | Later |
| Export telemetry (CSV, Prometheus, OTLP) | Later |

## 4. Incidents and work orders

| Feature | Status |
| --- | --- |
| Open / ack / assign / resolve incidents | Have |
| Auto-open from threshold + stale automations | Have |
| Threshold debounce and hysteresis | Have (`debounce_sec` and `hysteresis` on the rule) |
| Create work order from incident | Have |
| Work order list + mark done | Have |
| Work order create UI (standalone) | Have |
| Richer priorities / due dates / assignees UI | Have |
| Incident severity policies + runbooks | Have (Admin policies; runbook on incident detail) |
| SLA timers / escalation | Have (`sla_due_at` on work orders; escalation policy later) |
| Checklists / procedures on work orders | Have (`checklist` JSON on work orders) |
| Parts used + time tracking | Later |
| Mobile field tech mode (PWA) | Have (`/field` caches open work orders and syncs completions; manuals and shifts later) |
| Multi-asset work orders | Later |
| Calendar / preventive maintenance schedules | Partial (five-field `schedule_cron` opens one work order per matching minute) |

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
| Two-person action approval | Later |
| Multi-step playbooks | Later |

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
| SAP / Maximo / ServiceNow sync | Later |
| OAuth / mTLS for connectors | Later |

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
| Customizable overview widgets | Later |
| Multi-workspace switcher | Later |

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
| Login rate limit, logout, session revoke | Have (limiter is in-process) |
| Encrypted connector secrets | Have |
| Outbound egress policy | Have |
| Readiness (`/readyz` checks the database) | Have |
| SSO (OIDC/SAML) | Partial (OIDC discovery via `GET /api/v1/auth/oidc`; browser callback not implemented) |
| Multi-tenant product UX | Later |
| Soft-delete + retention | Later |
| Secrets vault for connector credentials | Have (AES-GCM at rest, redacted API; not an external vault) |
| Compliance exports | Later |

## 9. UX / product surfaces

| Feature | Status |
| --- | --- |
| Apple-inspired light UI + Zyvor mark | Have |
| Dark theme toggle | Have |
| Map full-bleed glass UI | Have |
| Asset Activity / Work / Integrations tabs filled | Have (Activity / Work) |
| Empty states + first-run guided demo | Have |
| Accessibility audit (WCAG) | Have (focus-visible rings, skip-to-content link, dialog focus trap + Escape via shared `useDialogA11y` hook) / Later (formal third-party audit) |
| Responsive / tablet field layout | Have (bottom nav) |
| i18n | Later |
| Printable WO / incident reports | Later |
| Saved views / filters | Later |

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
| PostGIS for map queries | Later |
| Horizontal replicas + shared Postgres | Later |
| Helm / k8s deploy | Have (`deploy/helm/yard`) |
| Multi-region | Later |

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
the Later-lane items above.
