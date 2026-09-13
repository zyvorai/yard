# Contributing to Yard

Yard is original Apache-2.0 software. Do not copy source from AGPL-licensed
projects such as Fleetbase.

## Development

```bash
make test
make web
make run
```

In another terminal:

```bash
export YARD_SIMULATOR_TOKEN=$(cat data/simulator.token)
make sim
```

## Boundaries

- Device Agent owns hardware discovery.
- Nodra owns industrial protocol semantics.
- Zyvor Fleet owns remote lifecycle.
- Yard owns the asset registry, sites, incidents, work orders, severity
  policies / runbooks, and UI (including bulk asset IO and map clustering).

## Pull requests

Include tests for ingest, tenant isolation, workflow, or policy changes.
Keep the console system-font based (self-hosted Inter as the non-Apple
fallback, no CDN fonts); use Apple-blue for primary actions and reserve
orange for the Zyvor brand mark only. Update `docs/ROADMAP.md` and the
Docusaurus site under `website/docs/` when you ship user-visible
features.

Schema changes go through the versioned migration runner
(`internal/store/store.go`'s `migrations` slice) — append a new
`{version, sql}` entry, never edit an existing one or fall back to
ad hoc `ALTER TABLE` at startup.
