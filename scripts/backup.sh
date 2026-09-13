#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: Apache-2.0
#
# Back up Yard's database — SQLite (default) or Postgres, whichever
# YARD_DATABASE_URL points at. Writes one timestamped file per run; nothing
# is ever overwritten.
#
# SQLite path uses `sqlite3 ... "VACUUM INTO"` — a live, consistent snapshot
# that doesn't lock out the running server. Postgres path shells out to
# `pg_dump` in custom (-Fc) format, restorable with `pg_restore`.
set -euo pipefail

DSN="${YARD_DATABASE_URL:-file:data/yard.db?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)}"
OUT_DIR="backups"

usage() {
  cat <<EOF
Usage: ./scripts/backup.sh [flags]

Back up Yard's database (SQLite or Postgres, auto-detected from the DSN).

  --dsn DSN   Database URL (default: \$YARD_DATABASE_URL, else local dev SQLite)
  --out DIR   Output directory (default: backups/)
  -h, --help  This help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dsn) DSN="$2"; shift 2 ;;
    --out) OUT_DIR="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown flag: $1" >&2; usage; exit 2 ;;
  esac
done

mkdir -p "$OUT_DIR"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"

case "$DSN" in
  postgres://*|postgresql://*|pgx://*)
    command -v pg_dump >/dev/null || { echo "pg_dump not found on PATH" >&2; exit 2; }
    dest="${OUT_DIR}/yard-${stamp}.dump"
    pg_dump "${DSN/pgx:\/\//postgres://}" -Fc -f "$dest"
    echo "==> Postgres backup written to $dest"
    echo "    restore with: ./scripts/restore.sh $dest"
    ;;
  *)
    command -v sqlite3 >/dev/null || { echo "sqlite3 not found on PATH" >&2; exit 2; }
    src="${DSN#file:}"
    src="${src%%\?*}"
    [[ -f "$src" ]] || { echo "sqlite file not found: $src (from DSN $DSN)" >&2; exit 2; }
    dest="${OUT_DIR}/yard-${stamp}.db"
    sqlite3 "$src" "VACUUM INTO '${dest}'"
    echo "==> SQLite backup written to $dest"
    echo "    restore with: ./scripts/restore.sh $dest"
    ;;
esac
