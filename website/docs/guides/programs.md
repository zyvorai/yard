---
sidebar_position: 3
title: Programs
---

import RelatedArticles from '@site/src/components/RelatedArticles';

# Programs

Yard connects assets, telemetry, maintenance, incidents, edge software, and remote actions. This page is the current cut: what the binary does, and what is only planned. The repository copy with the same detail is [docs/PHASES.md](https://github.com/zyvorai/yard/blob/main/docs/PHASES.md).

## Shipped foundation

`YARD_MODE` defaults to `demo`. Demo seeds a sample workspace and the public login `admin@yard.local` / `yard-admin`. `production` refuses that password, skips sample assets, and requires `YARD_PUBLIC_URL`, `YARD_SECRET_KEY`, and on first boot `YARD_BOOTSTRAP_EMAIL` plus `YARD_BOOTSTRAP_PASSWORD`. See [Deploy](./deploy) and [Security](../security).

Authorization is three roles. Viewers can read operational data. `operator` and `admin` can change assets, sites, incidents, work orders, automations, connectors, secrets, locations, and remote actions. `GET /api/v1/audit` is limited to those write roles. User administration stays admin-only. A viewer opening Overview does not mark assets stale; the background ticker does.

Connector `auth_token` values are encrypted at rest and are not returned by the API. Set them with `PUT /api/v1/connectors/{id}/secret`. Outbound connector calls allow only `http` and `https`, refuse redirects, and block link-local and metadata addresses. Production also blocks private networks unless the host is on `YARD_EGRESS_ALLOWLIST`. Certificate verification is on unless demo mode or `YARD_ALLOW_INSECURE_TLS=1` explicitly allows `tls_insecure`.

Live updates use a one-time ticket:

1. `POST /api/v1/stream/ticket` with the session bearer token.
2. `GET /api/v1/stream?ticket=...` once. The console does this for you.

`/readyz` fails when the database is down or migrations are behind the binary. `/healthz` only means the process is up.

## Started, not finished

**Remote actions.** A connector action returns `202` and a job. Failures back off up to 60s and then sit in `dead` until an operator retries them. Dangerous actions (`lifecycle.request`, `update.delegate`, reboot, shutdown, wipe, firmware, power off) wait in `pending_approval` until a different operator approves. Connectors can be tested, synced now, or scheduled (`sync_interval_sec`). On Postgres, one replica holds the schedule lock, and live events cross processes with `LISTEN`/`NOTIFY`.

**Locations.** `GET` and `POST /api/v1/locations` store a named place with an optional parent. The console shows that tree. Asset templates and links have list and create routes. A work order `schedule_cron` such as `0 8 * * 1` opens one work order when that minute arrives. An asset QR label is `GET /api/v1/assets/{id}/label`. Manuals and photos are `POST /api/v1/assets/{id}/attachments` (8 MiB). Parts and labor are lines on a work order. **Field** (`/field`) keeps open work orders on the device and syncs a completion when the network returns.

## Planned after that

An observation can be a number, a bool, or text. Numeric readings older than 24 hours roll up to an hourly min, max, and average. PostgreSQL stores those rollups in monthly partitions. Prometheus remote write, OTLP JSON metrics, and an MQTT subscriber are ingest paths. Operators save dashboards of asset signals. Timescale, histograms, and image payloads are not included.

| Order | Program | Intent |
| ---: | --- | --- |
| 4 | Telemetry Data Platform | Retention, typed values, hourly rollups, remote write, OTLP JSON, MQTT, and saved dashboards are in. Timescale and histograms are not |
| 5 | Intelligent Incident Management | De-duplication, SLAs, on-call, one incident for a flood of symptoms |
| 6 | Safe Automation and Playbook Engine | Multi-step recovery with dry-run and approval |
| 7 | Asset Digital Twin and Operations Graph | Dependencies, blast radius, indoor maps |
| 8 | Integration Hub | Connector SDK and enterprise systems, still optional to the core |
| 9 | Yard Intelligence | Explainable health scores and evidence-backed summaries |
| 10 | Enterprise Identity and Governance | OIDC/SAML/SCIM, custom roles, audit export |
| 11 | High Availability and Large-Scale Deployment | Replicas, shared workers, documented sizing |
| 12 | Advanced Field Workforce | Skills, permits, offline forms, still about assets |
| 13 | Energy, Sustainability, and Cost Intelligence | Downtime and energy cost next to maintenance |
| 14 | Vertical Solution Packs | Templates and dashboards, not separate products |
| 15 | Developer and Community Ecosystem | Releases, signed images, upgrade notes |
| 16 | Commercial Platform | Enterprise features stay out of the Apache-2.0 core |

OIDC today is discovery metadata only. Do not treat `GET /api/v1/auth/oidc` as single sign-on.

These are deliberately not next: route optimization, payroll, procurement, and a model that closes incidents by itself.

<RelatedArticles
  items={[
    {label: 'Security', to: '../security', description: 'Modes, sessions, secrets, and egress'},
    {label: 'Deploy', to: './deploy', description: 'Demo versus production environment'},
    {label: 'API', to: '../api', description: 'Auth, stream tickets, secrets, locations'},
  ]}
/>
