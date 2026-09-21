# Security

Report vulnerabilities privately to security@zyvor.dev if you have a
relationship with Zyvor, or open a confidential security advisory on GitHub.

## Runtime modes

`YARD_MODE=demo` (default) seeds the Northwind sample workspace and the
public demo login `admin@yard.local` / `yard-admin`. Those credentials are
for local evaluation only.

`YARD_MODE=production` refuses the public demo password, does not insert
sample assets, and requires:

- `YARD_PUBLIC_URL`
- `YARD_SECRET_KEY` (32-byte AES key, base64 or hex)
- On first boot: `YARD_BOOTSTRAP_EMAIL` and `YARD_BOOTSTRAP_PASSWORD`

Connector tokens are stored as SHA-256 hashes. Outbound connector credentials
are encrypted at rest and never returned by the API. Treat plaintext tokens
written to `data/*.token` as secrets.

Write RBAC: roles `admin` and `operator` may mutate registry, policies,
automations, incidents, connectors, and remote actions; viewers are
read-only. Audit log access requires a write role.

SSE uses single-use tickets from `POST /api/v1/stream/ticket` instead of
putting the session token in the EventSource URL.

Longer notes: [website security doc](https://zyvorai.github.io/yard/docs/security).
Program-by-program status: [docs/PHASES.md](docs/PHASES.md).
