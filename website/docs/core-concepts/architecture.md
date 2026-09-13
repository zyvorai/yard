---
sidebar_position: 1
title: Architecture
---

# Architecture

Yard is a standalone **asset and operations** platform. The core object is
an **asset**. Devices, vehicles, machines, sensors, and equipment are asset
kinds. Zyvor products plug in as optional connectors — Yard installs and
runs alone.

## Data model

Organization · Site · Asset · Capability · Observation · Event ·
WorkOrder · Incident · ActionRequest · Connector · Automation

Every observation stores **source**, **unit**, **observed_at**,
**received_at**, and **quality**. Offline data is marked stale rather than
healthy.

Remote actions require a session, expire, carry an idempotency key, and
record an outcome. Connectors advertise the actions they can execute.

## Boundaries

| Component | Responsibility |
| --- | --- |
| **Yard** | Asset registry, sites, workflows, incidents, shared UI |
| **Device Agent** | Hardware discovery, health, local diagnostics |
| **Nodra connector** | Decoded industrial telemetry and buffered events |
| **Zyvor Fleet connector** | Lifecycle requests and progress (catalog today) |
| **OTA connector** | Campaign display; execution stays elsewhere |
| **HTTP / simulator** | Zero-dependency evaluation path |

Device Agent reports physical capability. Nodra interprets protocols.
Fleet owns desired state when wired. Yard preserves those lines and adds
the operations surface: health, incidents, and work orders.

## Live ops loop

- Background stale ticker marks missed heartbeats without waiting for an
  Overview refresh
- Console pages subscribe to `GET /api/v1/stream` (SSE)
- Automations can open incidents, notify (audit), or call a webhook

## Storage

- **SQLite** by default (`go run`, CI, Compose)
- **Postgres** via `YARD_DATABASE_URL` (Compose `--profile postgres`)
