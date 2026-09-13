#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: Apache-2.0
#
# Smoke a deployed Yard instance (login + assets; optional incident gate).
#   YARD_URL=http://HOST:8080 ./scripts/verify-remote.sh
#   YARD_URL=... YARD_SIMULATOR_TOKEN=... ./scripts/verify-remote.sh
set -euo pipefail

BASE="${YARD_URL:-http://127.0.0.1:8080}"
BASE="${BASE%/}"

echo "==> health ${BASE}/healthz"
curl -fsS --connect-timeout 8 "${BASE}/healthz" >/dev/null

echo "==> login"
TOKEN=$(curl -sf -X POST "${BASE}/api/v1/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"admin@yard.local","password":"yard-admin"}' \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["token"])')
test -n "$TOKEN"

echo "==> overview"
curl -sf -H "Authorization: Bearer $TOKEN" "${BASE}/api/v1/overview" >/dev/null

echo "==> assets"
curl -sf -H "Authorization: Bearer $TOKEN" "${BASE}/api/v1/assets" >/dev/null

echo "==> connectors"
curl -sf -H "Authorization: Bearer $TOKEN" "${BASE}/api/v1/connectors" >/dev/null

SIM="${YARD_SIMULATOR_TOKEN:-}"
if [[ -z "$SIM" && -f data/simulator.token ]]; then
  SIM=$(tr -d '\n' < data/simulator.token)
fi
if [[ -n "$SIM" ]]; then
  echo "==> first-release gate (trip temperature)"
  YARD_URL="$BASE" YARD_SIMULATOR_TOKEN="$SIM" \
    "$(cd "$(dirname "$0")" && pwd)/first-release.sh"
else
  echo "==> skip incident gate (no YARD_SIMULATOR_TOKEN)"
fi

echo "PASS: verify-remote ${BASE}"
