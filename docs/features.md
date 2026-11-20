# Features

What Yard does, live operations, the registry and map, incidents and runbooks, and access control.

[Back to the README](../README.md)

## What Yard does

| Module | What users can do |
| --- | --- |
| Overview | Asset health, active work, incidents, recent activity |
| Assets | Register devices, vehicles, machines, sensors, equipment; CSV/JSON import-export |
| Sites | Factories, warehouses, offices, customer locations |
| Map | Clustered locations and stale-or-live status |
| Telemetry | Measurements, freshness, quality, threshold context |
| Work orders | Inspections, repairs, installations, maintenance |
| Incidents | Acknowledge, assign, resolve; severity policies attach runbooks |
| Automations | Threshold, capability-range, or stale triggers → incident, notify, webhook, Slack, email, or PagerDuty |
| Integrations | HTTP ingest, simulator, Device Agent; optional Zyvor connectors |
| Administration | Workspace, users & roles, API keys, severity policies, connector credentials, audit history |

## Live ops and automations

- Background **stale ticker** marks missed heartbeats without waiting for a page refresh
- Console pages subscribe to live updates with a short-lived SSE ticket (`POST /api/v1/stream/ticket`, then `GET /api/v1/stream?ticket=`). The session token is not placed in the stream URL
- Optional browser notifications for new **critical** incidents
- Automations: a literal **threshold**, a capability's own declared **Min/Max range** (no duplicated number to keep in sync), or a **stale** heartbeat, each → open incident, notify, webhook, Slack, email, or PagerDuty
- Create / enable / delete rules in the Automations console
- Remote actions carry idempotency keys and expiry. A connector action is queued on a `jobs` row and executed by an in-process worker, with exponential backoff up to 60s and operator retry or cancel; an action with no connector is recorded locally

## Registry, map, and bulk IO

- Asset create / edit / delete with capabilities, custom kinds, and site placement
- Site create / edit / delete with lat/lng
- MapLibre full-bleed map with **clustered** GeoJSON pins (click cluster to zoom)
- Bulk **CSV/JSON** export and import (`GET /api/v1/assets/export`, `POST /api/v1/assets/import`) — upsert by `external_ref`
- Onboarding wizard, command palette, and tablet-friendly bottom nav

See [Console features](https://zyvorai.github.io/yard/docs/guides/console) and [API](https://zyvorai.github.io/yard/docs/api).

## Incidents, severity policies, and runbooks

When a threshold or stale automation opens an incident, Yard resolves a **severity policy** (highest priority match):

| Match kind | Example | Effect |
| --- | --- | --- |
| `capability` | `temperature` | severity + runbook for that signal |
| `automation` | `Missed heartbeat` | match by automation name |
| `default` | *(empty value)* | fallback for everything else |

Seeded defaults include a critical temperature runbook and a warning heartbeat checklist. Edit policies under **Administration**; the Incidents detail panel shows the attached runbook. API: `/api/v1/severity-policies`.

## Users, roles, and API keys

- Three roles: **viewer** (read-only), **operator** and **admin** (both can mutate incidents, connectors, remote actions, and the registry). Audit reads require a write role. An empty or unrecognized role fails closed to read-only
- **Admin → Users**: invite by email (a 72h single-use token). Production sends it with `YARD_SMTP_*` and returns 503 if mail is not configured. Demo without SMTP logs the link. Deactivate rejects the next request; password reset deletes existing sessions
- **Admin → Sessions**: list and revoke the signed-in user's sessions, or sign out everywhere
- Self-service **password reset** never reveals whether an email has an account
- **Admin → Your API keys**: any role can mint a long-lived, per-person `yard_key_...` credential. Connector ingest tokens are separate and hashed. Outbound connector `auth_token` values are encrypted and set with `PUT /api/v1/connectors/{id}/secret`, not returned in connector JSON
- Login failures are rate-limited per address and email. `GET /api/v1/meta` tells the console whether to show the demo password
