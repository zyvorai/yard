---
sidebar_position: 3
title: Console features
---

import RelatedArticles from '@site/src/components/RelatedArticles';

# Console features

Day-to-day surfaces beyond the [quickstart](../getting-started/quickstart)
loop.

## Assets — bulk CSV / JSON

On **Assets**:

- **Export CSV** / **Export JSON** — downloads the current filter set
  (`q`, `kind`) via `GET /api/v1/assets/export`
- **Import** — accepts `.csv` or `.json`; rows upsert by `external_ref`

Typical CSV header:

```text
name,external_ref,kind,status,health,manufacturer,model,serial,site_id,latitude,longitude,stale_after_sec,metadata
```

## Telemetry — signal history

Below the per-asset latest-value cards, **Signal history** picks one
asset and capability and plots a real chronological sparkline over a
1h/24h/7d range, backed by
`GET /api/v1/assets/{id}/observations?capability=...&from=...&to=...`
(both `from`/`to` are optional RFC3339 bounds).

## Automations — triggers and actions

Trigger kinds:

| Trigger | Fires when | Needs |
| --- | --- | --- |
| `threshold` | `capability` `operator` `threshold` (e.g. `temperature gt 75`) | An explicit operator + literal threshold |
| `capability_min` / `capability_max` | The observation falls outside the **capability's own** declared `Min`/`Max` | Nothing else — no duplicated number to keep in sync |
| `stale` | Missed heartbeat past `stale_after_sec` | — |

Actions: `open_incident`, `notify`, `webhook`, `slack`, `email`,
`pagerduty`. Each action-specific field (Slack webhook URL, PagerDuty
routing key, recipient email, webhook URL) appears in the rule form only
for the action you pick, and is stored as a small JSON blob in the
rule's `config`.

:::note Email/Slack/PagerDuty need real credentials
These three actions are implemented and unit-tested against a local
mock server, but not verified against a real Slack workspace,
PagerDuty account, or SMTP relay — there wasn't one available while
building this. Email additionally needs `YARD_SMTP_HOST` (and
optionally `_PORT`/`_USER`/`_PASS`/`_FROM`) set as a server-wide
environment variable, since a mail relay is operator infrastructure
rather than a per-rule credential. Validate delivery with your own
credentials before relying on any of the three in production.
:::

## Administration — Users and API keys

**Users** (admin role only): invite by email + role, change a user's
role inline, or deactivate/reactivate them — see
[Security → Users](../security#users-invite-deactivate-password-reset)
for what each of those does under the hood.

**Your API keys** (any role, self-service): create a long-lived,
per-person credential for scripts and automation — separate from
connector tokens, which are shared per-integration rather than
per-person. The raw token is shown once at creation; only a short hint
is kept afterward.

## Map — clustering

The Map page uses MapLibre **clustered GeoJSON** layers:

- Dense pins collapse into count clusters (blue → amber → orange by size)
- Click a cluster to zoom into its members
- Click an unclustered pin to open the detail card
- Site pins use blue; asset pins follow health (healthy / warning /
  critical / stale)

## Incidents — severity + runbooks

When a threshold or stale automation opens an incident, Yard resolves a
**severity policy** (highest priority match):

| Match kind | Example | Effect |
| --- | --- | --- |
| `capability` | `temperature` | severity + runbook for that signal |
| `automation` | `Missed heartbeat` | match by automation name |
| `default` | *(empty value)* | fallback for everything else |

Seeded defaults include a critical temperature runbook and a warning
heartbeat checklist. The Incidents detail panel shows the attached
**Runbook** text.

Edit policies under **Administration → Incident severity policies**
(`GET/POST /api/v1/severity-policies`, `PATCH/DELETE …/{id}`).

<RelatedArticles
  items={[
    {label: 'API', to: '../api', description: 'Curl examples for export/import, policies, and auth'},
    {label: 'Security', to: '../security', description: 'RBAC, invite/reset flows, API keys vs connector tokens'},
    {label: 'Architecture', to: '../core-concepts/architecture', description: 'Data model'},
    {label: 'Full feature catalog', to: 'https://github.com/zyvorai/yard/blob/main/docs/ROADMAP.md', description: 'docs/ROADMAP.md'},
  ]}
/>
