# Sizing

Two profiles. Both run the same binary.

## Small

One replica. SQLite on a local volume (`replicaCount: 1` and a `file:` database URL). This is the Helm default. It is enough for a single site and a single operator console.

## Shared

PostgreSQL and two replicas. Set `databaseUrl` to a `postgres://` or `postgresql://` URL. Helm refuses `replicaCount` greater than 1 with any other URL, because two processes cannot share a SQLite file. Each replica claims queued jobs. On Postgres that claim uses `FOR UPDATE SKIP LOCKED`. If another replica already took the row, SQLite tries the next queued id. Ingest counts in one table so the processes share a budget. `YARD_REGION` is stamped on new events.

`YARD_REGION` is stamped on new events. Set `YARD_PEER_URL` and `YARD_PEER_TOKEN` to post those events to another Yard. The receiver keeps the source region and does not forward them again. The public lab runs one region.
