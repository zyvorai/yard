# Interface and tests

The console interface principles and how the project is tested.

[Back to the README](../README.md)

## Interface

Apple-inspired, original identity:

- White and soft-gray surfaces, dark type, generous spacing
- Apple-blue (`#0071e3` / `#0a84ff` in dark mode) for primary actions and selected states; the Zyvor mark keeps its own brand orange
- Compact labeled sidebar; asset detail panel that does not replace the list
- Dark, searchable diagnostics
- System fonts (no CDN), visible keyboard focus, reduced-motion support
- Skip-to-content link; dialogs trap focus and close on Escape

## Tests

```bash
go test ./...
cd web && npm test
```

Or `make test`. Release gates cover tenant isolation, connector authentication, duplicate observations, stale telemetry, bulk import/export, severity policies, the health → incident → work order → resolve workflow, capability-range alarms, the full invite/role/deactivate/API-key lifecycle, and that the SSE stream survives the request-logging middleware chain (not just the handler in isolation). The Go and TypeScript SDKs (`sdk/`) are each verified against a real running server.
