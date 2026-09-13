# Security

Report vulnerabilities privately to security@zyvor.dev if you have a
relationship with Zyvor, or open a confidential security advisory on GitHub.

Default demo credentials (`admin@yard.local` / `yard-admin`) are for
local evaluation only. Change them before any shared deployment.

Connector tokens are stored as SHA-256 hashes. Treat the plaintext tokens
written to `data/*.token` as secrets.

Write RBAC: roles `admin` and `operator` may mutate registry, policies,
automations, and asset import; viewers are read-only. Mutating actions are
audited (including severity-policy and bulk-import changes).

Longer notes: [website security doc](https://zyvorai.github.io/yard/docs/security).
