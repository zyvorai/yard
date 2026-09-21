---
sidebar_position: 3
title: Programs
---

import RelatedArticles from '@site/src/components/RelatedArticles';

# Programs

Yard connects assets, telemetry, maintenance, incidents, edge software, and remote actions. This page is the current cut of what the binary does. The repository copy with the same detail is [docs/PHASES.md](https://github.com/zyvorai/yard/blob/main/docs/PHASES.md). The feature catalog is [docs/ROADMAP.md](https://github.com/zyvorai/yard/blob/main/docs/ROADMAP.md).

## Shipped foundation

`YARD_MODE` defaults to `demo`. Demo seeds a sample workspace and the public login `admin@yard.local` / `yard-admin`. `production` refuses that password, skips sample assets, and requires `YARD_PUBLIC_URL`, `YARD_SECRET_KEY`, and on first boot `YARD_BOOTSTRAP_EMAIL` plus `YARD_BOOTSTRAP_PASSWORD`. See [Deploy](./deploy) and [Security](../security).

Authorization is three roles. Viewers can read operational data. `operator` and `admin` can change assets, sites, incidents, work orders, automations, connectors, secrets, locations, and remote actions. `GET /api/v1/audit` is limited to those write roles. User administration stays admin-only. A viewer opening Overview does not mark assets stale; the background ticker does.

Connector `auth_token` values are encrypted at rest and are not returned by the API. Set them with `PUT /api/v1/connectors/{id}/secret`. Outbound connector calls allow only `http` and `https`, refuse redirects, and block link-local and metadata addresses. Production also blocks private networks unless the host is on `YARD_EGRESS_ALLOWLIST`. Certificate verification is on unless demo mode or `YARD_ALLOW_INSECURE_TLS=1` explicitly allows `tls_insecure`.

Live updates use a one-time ticket:

1. `POST /api/v1/stream/ticket` with the session bearer token.
2. `GET /api/v1/stream?ticket=...` once. The console does this for you.

`/readyz` fails when the database is down or migrations are behind the binary. `/healthz` only means the process is up.

## What is in the binary

**Remote actions.** A connector action returns `202` and a job. Failures back off up to 60s and then sit in `dead` until an operator retries them. Dangerous actions wait in `pending_approval` until a different operator approves. Connectors can be tested, synced now, or scheduled (`sync_interval_sec`). On Postgres, one replica holds the schedule lock, and live events cross processes with `LISTEN`/`NOTIFY`.

**Locations and field.** `GET` and `POST /api/v1/locations` store a named place with an optional parent. Asset templates, catalogs, links, QR labels, NFC lookup, BOM lines, and install history are in the API. A work order `schedule_cron` opens one work order when that minute arrives. **Field** (`/field`) keeps open work orders and manuals on the device, shows checklist and permit state while online, and syncs a completion when the network returns.

**Telemetry.** An observation can be a number, a bool, text, or a histogram. Numeric readings older than 24 hours roll up hourly. Prometheus remote write, OTLP JSON metrics, MQTT, filtered query, and CSV / Prometheus / OTLP export are ingest and read paths. PostgreSQL keeps monthly rollup partitions. Optional TimescaleDB is `YARD_TIMESCALE=1` against a TimescaleDB image; `/api/v1/meta` reports `timescale`.

| Order | Program | Intent |
| ---: | --- | --- |
| 4 | Telemetry Data Platform | Retention, typed values, histograms, hourly rollups, remote write, OTLP JSON, MQTT, saved dashboards, filtered query, and export are in |
| 5 | Intelligent Incident Management | Debounce, hysteresis, flap counts, parent incidents, ack and resolve times, on-call, and a timeline are in |
| 6 | Safe Automation and Playbook Engine | Step editor, dry-run, approval, and a source refresh are in |
| 7 | Asset Digital Twin and Operations Graph | Desired state, twin, blast radius, geofences, and floorplan pins are in |
| 8 | Integration Hub | Catalog, webhook, marketplace, and ServiceNow, Maximo, and SAP create clients plus sync are in |
| 9 | Yard Intelligence | Score, anomaly, cited answers, and approved writes are in |
| 10 | Enterprise Identity and Governance | OIDC, SAML, SCIM, WebAuthn storage, authenticator codes, custom roles, audit CSV, Vault key fetch, and a required customer key are in |
| 11 | High Availability and Large-Scale Deployment | SKIP LOCKED claims, a shared ingest budget, and peer event replication are in. The public lab runs one region |
| 12 | Advanced Field Workforce | Permits, skills, shifts, and offline manuals are in |
| 13 | Energy, Sustainability, and Cost Intelligence | Tariffs, carbon, and a 7-day forecast are on `/cost` |
| 14 | Vertical Solution Packs | Five packs import templates, a dashboard, automations, and a work order |
| 15 | Developer and Community Ecosystem | Tag builds cover amd64 and arm64 and sign the checksum file with cosign keyless |
| 16 | Commercial Platform | `/api/v1/meta` reports `edition: community`. Admins can export a compliance CSV. This repo does not bill |

OIDC login is `GET /api/v1/auth/oidc/start` and the callback at `/api/v1/auth/oidc/callback`. Production matches an existing user. Demo creates one only when `YARD_OIDC_PROVISION=1`. SAML is `POST /api/v1/auth/saml/acs`.

Still out of this product: route optimization, payroll, procurement, and a model that closes incidents by itself. Directions between sites stay in the logistics extension.

<RelatedArticles
  items={[
    {label: 'Security', to: '../security', description: 'Modes, sessions, secrets, and egress'},
    {label: 'Deploy', to: './deploy', description: 'Demo versus production environment'},
    {label: 'API', to: '../api', description: 'Auth, stream tickets, secrets, locations'},
  ]}
/>
