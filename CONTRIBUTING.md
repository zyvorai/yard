# Contributing to Estate

Estate is original Apache-2.0 software. Do not copy source from AGPL-licensed
projects such as Fleetbase.

## Development

```bash
make test
make web
make run
```

In another terminal:

```bash
export ESTATE_SIMULATOR_TOKEN=$(cat data/simulator.token)
make sim
```

## Boundaries

- Device Agent owns hardware discovery.
- Nodra owns industrial protocol semantics.
- Zyvor Fleet owns remote lifecycle.
- Estate owns the asset registry, sites, incidents, work orders, and UI.

## Pull requests

Include tests for ingest, tenant isolation, or workflow changes.
Keep the console system-font based (no CDN fonts) and reserve orange for
primary actions.
