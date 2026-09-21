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

- Console login issues a bearer session (bcrypt password hash, 12 hours)
- Failed logins are limited per client address and email
- `POST /api/v1/auth/logout` deletes that session. Administration lists the caller's sessions and can revoke one or all of them. Password reset deletes every session for that user
- Write operations (assets, sites, incidents, work orders, automations, connectors, connector secrets, locations, remote actions, severity policies, asset import) require role `admin` or `operator`
- `GET /api/v1/audit` requires the same write role. Viewers do not see recent audit rows on Overview
- Viewers can read operational pages
- An empty or unrecognized role is read-only
- Managing other users (invite, role change, deactivate) is **admin-only**
- Reading Overview does not mark assets stale. A background ticker does

## Runtime mode

`YARD_MODE=demo` (default) seeds sample data and `admin@yard.local` / `yard-admin`.

`YARD_MODE=production` refuses that password, does not insert sample assets, and requires `YARD_PUBLIC_URL`, `YARD_SECRET_KEY` (32 bytes, base64 or hex), and on an empty database `YARD_BOOTSTRAP_EMAIL` and `YARD_BOOTSTRAP_PASSWORD`. The login form asks `GET /api/v1/meta` and only prefills the demo password in demo mode.

## Users: invite, deactivate, password reset

Administration → **Users** (admin role required):

- **Invite** creates the account with an unusable password and a 72-hour single-use token. The person sets a password at `/accept-invite?token=...`
- When `YARD_SMTP_HOST` is set, Yard emails the link and does not log the token. Demo mode without SMTP logs the link. Production without SMTP returns 503 and does not log a token
- **Deactivate** makes the next authenticated request fail
- **Forgot password** always responds `200` when mail can be sent, including for unknown emails. Production without SMTP returns 503 for every request so the handler does not mint a token

## Connector secrets, tokens, and API keys

| | Ingest token | Outbound secret | API key |
| --- | --- | --- | --- |
| Belongs to | One connector | One connector | One human user |
| Storage | SHA-256 hash | AES-256-GCM ciphertext | SHA-256 hash |
| Set via | Rotate token (shown once) | `PUT /api/v1/connectors/{id}/secret` | Administration → API keys (shown once) |
| Returned later | Hint only | `has_secret` and `secret_hint` only | Hint only |

Do not put `auth_token` in connector `config`. The API rejects that field. Action payloads cannot override the stored secret.

Treat files under `data/*.token` as secrets. They are the bootstrap ingest and simulator tokens.

## Live updates

The console requests `POST /api/v1/stream/ticket` and opens `GET /api/v1/stream?ticket=`. The ticket is single-use and lasts about 60 seconds. A long-lived session token in the stream query string is rejected.

## Egress and TLS

Connector and webhook calls allow `http` and `https` only, do not follow redirects, and cap response bodies. Link-local and metadata addresses are blocked. Production also blocks loopback and private ranges unless the host is listed in `YARD_EGRESS_ALLOWLIST`. Demo allows private addresses so a local Device Agent works.

TLS verification is on by default. `tls_insecure: true` is accepted in demo mode or when `YARD_ALLOW_INSECURE_TLS=1`.

## Ingest hardening

- Ingest endpoints are rate limited (per token / remote address)
- Observations carry quality, source, and timestamps; duplicates via
  `dedupe_key` are ignored
- Prometheus-style counters are exposed at `/metrics`
- Every request is logged as one structured line (method, path, status,
  duration); set `YARD_LOG_FORMAT=json` for machine-parseable output


## Production hardening

The controls above are the production baseline: mode, encrypted secrets, login limits, session revoke, SSE tickets, CORS (`YARD_CORS_ORIGINS`; production with an empty list is same-origin only), security headers, egress policy, and `/readyz`.

CORS in demo mode with no allowlist is `*`. `Strict-Transport-Security` is sent when `YARD_PUBLIC_URL` uses `https`.

## OIDC login

`GET /api/v1/auth/oidc` returns issuer/client metadata when `YARD_OIDC_ISSUER`
and `YARD_OIDC_CLIENT_ID` are set (Helm chart `oidc.*` values map to these
env vars). `GET /api/v1/auth/oidc/start` begins the authorization code flow;
`/api/v1/auth/oidc/callback` exchanges the code and issues a Yard session.

See [Programs](./guides/programs) for the full program board.

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
