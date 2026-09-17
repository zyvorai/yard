#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: Apache-2.0
# CI: SQLite backup/restore round-trip for Yard (software-class).
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
cd "$ROOT"
TMP=$(mktemp -d)
trap 'kill ${PID:-0} 2>/dev/null || true; rm -rf "$TMP"' EXIT

mkdir -p bin
# Static assets are committed under cmd/yard/static — build binaries without npm.
go build -o bin/yard ./cmd/yard

DATA="$TMP/data"
mkdir -p "$DATA"
export YARD_DATABASE_URL="file:${DATA}/yard.db?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
export YARD_DATA_DIR="$DATA"
PORT=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')
export YARD_LISTEN="127.0.0.1:${PORT}"

./bin/yard >"$TMP/yard.log" 2>&1 &
PID=$!
for i in $(seq 1 50); do
  curl -fsS "http://127.0.0.1:${PORT}/healthz" >/dev/null 2>&1 && break
  sleep 0.2
done
curl -fsS "http://127.0.0.1:${PORT}/healthz" >/dev/null
kill "$PID"
wait "$PID" 2>/dev/null || true
PID=0

test -f "$DATA/yard.db"
./scripts/backup.sh --dsn "$YARD_DATABASE_URL" --out "$TMP/backups"
BACKUP=$(ls -1 "$TMP/backups"/yard-*.db | head -1)
test -n "$BACKUP"
# VACUUM INTO rewrites pages — compare restore to the backup blob, not the live file.
hash_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}
BACKUP_HASH=$(hash_file "$BACKUP")

RESTORE_DB="$TMP/restore/yard.db"
mkdir -p "$(dirname "$RESTORE_DB")"
./scripts/restore.sh "$BACKUP" --dsn "file:${RESTORE_DB}"
AFTER=$(hash_file "$RESTORE_DB")
test "$BACKUP_HASH" = "$AFTER"

export YARD_DATABASE_URL="file:${RESTORE_DB}?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
export YARD_DATA_DIR="$TMP/restore"
./bin/yard >"$TMP/yard2.log" 2>&1 &
PID=$!
for i in $(seq 1 50); do
  curl -fsS "http://127.0.0.1:${PORT}/healthz" >/dev/null 2>&1 && break
  sleep 0.2
done
curl -fsS "http://127.0.0.1:${PORT}/healthz" >/dev/null
echo "PASS: yard backup-restore"
