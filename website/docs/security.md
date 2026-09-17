---
sidebar_position: 4
title: Security
---

import RelatedArticles from '@site/src/components/RelatedArticles';

# Security

Report vulnerabilities privately to [security@zyvor.dev](mailto:security@zyvor.dev)
or open a confidential security advisory on GitHub.

:::caution Demo credentials
Default credentials (`admin@yard.local` / `yard-admin`) are for **local
evaluation only**. Change them before any shared deployment.
:::

## Sessions and RBAC

- Console login issues a bearer session (bcrypt password hash)
- Write operations (create/update/delete assets, sites, work orders,
  automations, actions, severity policies, asset import) require role
  `admin` or `operator`
- Viewers can read but not mutate
- An empty or unrecognized role is treated as the most restrictive
  (read-only) rather than defaulting to admin — a misconfigured role
  fails closed, not open
- Managing other users' accounts (invite, role change, deactivate) is
  **admin-only**, stricter than the general write gate above, which
  also allows `operator`

## Users: invite, deactivate, password reset

Administration → **Users** (admin role required):

- **Invite** creates the account immediately with an unusable
  placeholder password and a 72-hour, single-use invite token; the
  invited person sets their own password via `/accept-invite?token=...`
  and is signed in on success
- **Deactivate** takes effect immediately — it invalidates the user's
  existing session token(s), not just their next login attempt
- **Forgot password** (`/api/v1/auth/request-reset`) always responds
  `200` regardless of whether the email exists, so the endpoint can't
  be used to enumerate registered accounts; the reset link itself is
  logged server-side rather than emailed (no SMTP account is wired up
  by default — see [Automations](./guides/console#automations--triggers-and-actions)
  for the same `YARD_SMTP_*` config used by the email automation action)

## Connector tokens and API keys

Two distinct bearer-token credential types, both `Authorization: Bearer
<token>`, both stored as SHA-256 hashes with only a short hint kept for
display, both shown once at creation/rotation and never retrievable
again:

| | Connector token | API key |
| --- | --- | --- |
| Belongs to | One connector (a machine integration) | One human user |
| Created via | Administration → Connectors → Rotate token | Administration → Your API keys |
| Scope | That connector's ingest/action routes | Everything that user's own session can do |
| Revoked when | Token rotated | Key deleted, or the owning user is deactivated |

Treat plaintext tokens written to `data/*.token` (or
`/var/lib/yard/*.token` on lab hosts) as secrets — those are the
bootstrap ingest/simulator tokens, distinct from both of the above.

## Ingest hardening

- Ingest endpoints are rate limited (per token / remote address)
- Observations carry quality, source, and timestamps; duplicates via
  `dedupe_key` are ignored
- Prometheus-style counters are exposed at `/metrics`
- Every request is logged as one structured line (method, path, status,
  duration); set `YARD_LOG_FORMAT=json` for machine-parseable output


## OIDC discovery (partial)

`GET /api/v1/auth/oidc` returns issuer/client metadata when `YARD_OIDC_ISSUER`
and `YARD_OIDC_CLIENT_ID` are set (Helm chart `oidc.*` values map to these
env vars). The browser authorization-code callback is not wired yet — treat
this as discovery-only until the callback lands.

## Audit

Mutating console and automation actions write to the org audit log
(Administration → audit), including asset import, severity policy
changes, user invites/role changes, and API key create/revoke.

<RelatedArticles
  items={[
    {label: 'Console features', to: './guides/console', description: 'Users, API keys, and connector credentials in Administration'},
    {label: 'API', to: './api', description: 'Auth, invite, and reset endpoints'},
    {label: 'Deploy', to: './guides/deploy', description: 'Backups and structured logging in production'},
  ]}
/>
