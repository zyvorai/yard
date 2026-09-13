#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: Apache-2.0
#
# Restore a backup produced by scripts/backup.sh. Detects SQLite (.db) vs
# Postgres (.dump) by file extension. Stop the Yard process first — this
# script does not do it for you.
#
# Safety: for SQLite, the CURRENT database file is saved alongside it
# (suffixed .before-restore) before being overwritten, never deleted
# outright. For Postgres, pg_restore --clean drops and recreates objects
# inside the target database — it does not touch other databases on the
# server.
set -euo pipefail

DSN="${YARD_DATABASE_URL:-file:data/yard.db?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)}"
BACKUP=""

usage() {
  cat <<EOF
Usage: ./scripts/restore.sh BACKUP_FILE [flags]

Restore a database backup produced by scripts/backup.sh.

  --dsn DSN   Database URL to restore into (default: \$YARD_DATABASE_URL, else local dev SQLite)
  -h, --help  This help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dsn) DSN="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    -*) echo "unknown flag: $1" >&2; usage; exit 2 ;;
    *) BACKUP="$1"; shift ;;
  esac
done

if [[ -z "$BACKUP" ]]; then
  usage
  exit 2
fi
[[ -f "$BACKUP" ]] || { echo "backup file not found: $BACKUP" >&2; exit 2; }

case "$BACKUP" in
  *.dump)
    command -v pg_restore >/dev/null || { echo "pg_restore not found on PATH" >&2; exit 2; }
    case "$DSN" in
      postgres://*|postgresql://*|pgx://*) ;;
      *) echo "a .dump backup needs a postgres:// --dsn to restore into, got: $DSN" >&2; exit 2 ;;
    esac
    echo "==> restoring $BACKUP into ${DSN%%\?*} (--clean: drops/recreates existing objects)"
    pg_restore --clean --if-exists -d "${DSN/pgx:\/\//postgres://}" "$BACKUP"
    echo "==> done"
    ;;
  *.db)
    command -v sqlite3 >/dev/null || { echo "sqlite3 not found on PATH" >&2; exit 2; }
    case "$DSN" in
      postgres://*|postgresql://*|pgx://*) echo "a .db backup needs a sqlite --dsn to restore into, got: $DSN" >&2; exit 2 ;;
    esac
    dest="${DSN#file:}"
    dest="${dest%%\?*}"
    if [[ -f "$dest" ]]; then
      saved="${dest}.before-restore"
      cp "$dest" "$saved"
      echo "==> current database saved to $saved"
    fi
    mkdir -p "$(dirname "$dest")"
    cp "$BACKUP" "$dest"
    echo "==> restored $BACKUP to $dest"
    ;;
  *)
    echo "unrecognized backup extension (expected .db or .dump): $BACKUP" >&2
    exit 2
    ;;
esac
