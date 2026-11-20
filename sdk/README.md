# Yard SDKs

Thin, hand-authored clients for the Yard API. The API surface is small
(~35 routes — see [openapi.yaml](../openapi.yaml) at the repo root for the
full reference), so these wrap the core flows rather than every route:
auth, assets, sites, telemetry, incidents, work orders, automations. Adding
a call you need is a few lines, following the pattern of the ones already
there.

Authenticate either client with a session token (from login) or a
long-lived human API key — create one from the console under
**Administration → Your API keys**, or `POST /api/v1/api-keys`.

## Go — [`go/`](go/)

```go
import yardclient "github.com/zyvorai/yard/sdk/go"

anon := yardclient.New("http://localhost:8080")
client, session, err := anon.Login(ctx, "admin@yard.local", "yard-admin")

// or, with an existing API key:
client := yardclient.New("http://localhost:8080").WithToken("yard_key_...")

assets, err := client.ListAssets(ctx, "", "", "")
```

## TypeScript — [`ts/`](ts/)

```ts
import { YardClient } from "@zyvorai/yard-client";

const anon = new YardClient("http://localhost:8080");
const { client, session } = await anon.login("admin@yard.local", "yard-admin");

// or, with an existing API key:
const client = new YardClient("http://localhost:8080", "yard_key_...");

const assets = await client.listAssets();
```

Build it with `cd sdk/ts && npm install && npm run build` (emits `dist/`).
