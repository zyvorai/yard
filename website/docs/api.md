---
sidebar_position: 5
title: API
---

# API

Yard exposes a versioned HTTP API under `/api/v1`. The OpenAPI 3.0
description lives in the repository:

[openapi.yaml](https://github.com/zyvorai/yard/blob/master/openapi.yaml)

## Auth

```bash
curl -sS -X POST "$YARD_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@yard.local","password":"yard-admin"}'
```

Use `Authorization: Bearer <token>` on subsequent calls.

## Surfaces

| Area | Paths |
| --- | --- |
| Health | `/healthz`, `/readyz`, `/metrics` |
| Registry | `/api/v1/sites`, `/api/v1/assets` |
| Ops | `/api/v1/telemetry`, `/api/v1/events`, `/api/v1/incidents`, `/api/v1/work-orders` |
| Platform | `/api/v1/connectors`, `/api/v1/actions`, `/api/v1/automations`, `/api/v1/audit` |
| Live | `/api/v1/stream` (SSE) |
| Ingest | `/api/v1/ingest/observations`, `…/inventory`, `…/events` |

Ingest uses connector bearer tokens, not human sessions. See
[Connectors](./guides/connectors).
