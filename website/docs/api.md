---
sidebar_position: 5
title: API
---

import RelatedArticles from '@site/src/components/RelatedArticles';

# API

Yard exposes a versioned HTTP API under `/api/v1`. The OpenAPI 3.0
description lives in the repository:

[openapi.yaml](https://github.com/zyvorai/yard/blob/main/openapi.yaml)

## Auth

```bash
curl -sS -X POST "$YARD_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@yard.local","password":"yard-admin"}'
```

Use `Authorization: Bearer <token>` on subsequent calls.

:::info Write access
Write mutations (assets, sites, incidents, work orders, automations,
connectors, secrets, locations, actions, severity policies, and asset
import) require role `admin` or `operator`. `GET /api/v1/audit` does
too. Viewers can read operational routes. Managing other users is
admin-only — see [Security](./security).
:::

The OpenAPI file covers the original route set. Newer routes on this
page (meta, logout, sessions, stream tickets, connector secrets,
locations) are documented here; the YAML catalog has not been regenerated
for them yet. Hand-authored Go and TypeScript clients covering the
core flows (auth, assets, telemetry, incidents, work orders,
automations) are in
[`sdk/go`](https://github.com/zyvorai/yard/tree/main/sdk/go) and
[`sdk/ts`](https://github.com/zyvorai/yard/tree/main/sdk/ts) — each
verified against a real running Yard server, not just typechecked.

## Surfaces

| Area | Paths |
| --- | --- |
| Health | `/healthz`, `/readyz`, `/metrics` |
| Auth | `/api/v1/auth/login`, `…/logout`, `…/me`, `…/sessions`, `…/accept-invite`, `…/request-reset`, `…/reset`, `…/oidc` |
| Meta | `/api/v1/meta` (mode; no auth) |
| Admin | `/api/v1/admin/users` (list/invite/role/deactivate), `/api/v1/api-keys` |
| Registry | `/api/v1/sites`, `/api/v1/assets`, `GET /api/v1/assets/{id}/label`, `GET /api/v1/assets/lookup`, `…/{id}/capabilities`, `/api/v1/locations`, `/api/v1/asset-templates`, `/api/v1/asset-links` |
| Bulk IO | `/api/v1/assets/export`, `/api/v1/assets/import` |
| Ops | `/api/v1/telemetry`, `/api/v1/assets/{id}/observations` (optional `capability`/`from`/`to`), `/api/v1/events`, `/api/v1/incidents`, `/api/v1/work-orders` |
| Policies | `/api/v1/severity-policies` |
| Platform | `/api/v1/connectors`, `PUT /api/v1/connectors/{id}/secret`, `POST /api/v1/connectors/{id}/test`, `POST /api/v1/connectors/{id}/sync`, `/api/v1/actions`, `GET /api/v1/jobs`, `POST /api/v1/jobs/{id}/retry`, `POST /api/v1/jobs/{id}/cancel`, `POST /api/v1/jobs/{id}/approve`, `/api/v1/automations`, `/api/v1/audit` |
| Live | `POST /api/v1/stream/ticket`, `GET /api/v1/stream?ticket=` |
| Ingest | `/api/v1/ingest/observations`, `…/inventory`, `…/events` |
| Misc | `/api/v1/onboarding`, `/api/v1/geocode` |

Ingest uses connector bearer tokens, not human sessions. See
[Connectors](./guides/connectors). Everything else accepts either a
session token (from login) or a human API key — see
[Security](./security).

## Live updates

```bash
TICKET=$(curl -sS -X POST -H "Authorization: Bearer $TOKEN" \
  "$YARD_URL/api/v1/stream/ticket" | jq -r .ticket)
curl -N "$YARD_URL/api/v1/stream?ticket=$TICKET"
```

The ticket is single-use and expires in about 60 seconds. Do not put the session token on the stream URL.

## Connector secret

```bash
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"secret":"..."}' \
  "$YARD_URL/api/v1/connectors/$CONNECTOR_ID/secret"
```

The response includes `has_secret` and `secret_hint`. It does not echo the secret. Connector list JSON never includes `auth_token`.

## Locations

```bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Cold room","kind":"zone"}' \
  "$YARD_URL/api/v1/locations"
```

`parent_id` is optional. This is a stored tree, not a floor-plan UI.

## Time-range telemetry

```bash
curl -sS -H "Authorization: Bearer $TOKEN" \
  "$YARD_URL/api/v1/assets/$ASSET_ID/observations?capability=temperature&from=2026-09-01T00:00:00Z&to=2026-09-02T00:00:00Z"
```

Omit `capability`/`from`/`to` for an unbounded, all-capability history
(the same call the asset detail panel makes); `/api/v1/telemetry`
remains the latest-value-only snapshot across every asset.

## Bulk asset export / import

Export (JSON or CSV attachment):

```bash
curl -sS -H "Authorization: Bearer $TOKEN" \
  "$YARD_URL/api/v1/assets/export?format=csv" -o assets.csv
```

Import upserts by `external_ref` when present:

```bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '[{"name":"Pump P-09","external_ref":"P-09","kind":"machine"}]' \
  "$YARD_URL/api/v1/assets/import?format=json"
```

CSV columns: `name`, `external_ref`, `kind`, `status`, `health`,
`manufacturer`, `model`, `serial`, `site_id`, `latitude`, `longitude`,
`stale_after_sec`, `metadata`.

## Severity policies

List or create policies that map a capability name, automation name, or
default fallback to `severity` + `runbook`. Highest `priority` wins when
automations open incidents.

```bash
curl -sS -H "Authorization: Bearer $TOKEN" \
  "$YARD_URL/api/v1/severity-policies"
```

See [Console features](./guides/console) for the Admin UI and Map
clustering behavior.

<RelatedArticles
  items={[
    {label: 'Console features', to: './guides/console', description: 'Admin UI for users, API keys, bulk IO, and severity policies'},
    {label: 'Security', to: './security', description: 'RBAC, invite/reset flows, API keys vs connector tokens'},
    {label: 'Connectors', to: './guides/connectors', description: 'Ingest auth and Device Agent gateway'},
    {label: 'Architecture', to: './core-concepts/architecture', description: 'Data model and boundaries'},
  ]}
/>
