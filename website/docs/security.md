---
sidebar_position: 4
title: Security
---

# Security

Report vulnerabilities privately to [security@zyvor.dev](mailto:security@zyvor.dev)
or open a confidential security advisory on GitHub.

## Demo credentials

Default credentials (`admin@yard.local` / `yard-admin`) are for **local
evaluation only**. Change them before any shared deployment.

## Sessions and RBAC

- Console login issues a bearer session (bcrypt password hash)
- Write operations (create/update/delete assets, sites, work orders,
  automations, actions, severity policies, asset import) require role
  `admin` or `operator`
- Viewers can read but not mutate

## Connector tokens

Connectors authenticate with `Authorization: Bearer <token>`. Tokens are
stored as SHA-256 hashes. Treat plaintext tokens written to
`data/*.token` (or `/var/lib/yard/*.token` on lab hosts) as secrets.

## Ingest hardening

- Ingest endpoints are rate limited (per token / remote address)
- Observations carry quality, source, and timestamps; duplicates via
  `dedupe_key` are ignored
- Prometheus-style counters are exposed at `/metrics`

## Audit

Mutating console and automation actions write to the org audit log
(Administration → audit), including asset import and severity policy
changes.
