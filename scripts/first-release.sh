#!/usr/bin/env bash
set -euo pipefail
# Exercises the first-release gate against a running Yard instance.
BASE=${YARD_URL:-http://127.0.0.1:8080}
echo "login"
TOKEN=$(curl -sf -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"admin@yard.local","password":"yard-admin"}' | python3 -c 'import sys,json; print(json.load(sys.stdin)["token"])')
echo "assets"
curl -sf -H "Authorization: Bearer $TOKEN" "$BASE/api/v1/assets" >/dev/null
SIM=${YARD_SIMULATOR_TOKEN:-}
if [[ -z "$SIM" && -f data/simulator.token ]]; then SIM=$(cat data/simulator.token); fi
echo "trip temperature"
curl -sf -X POST "$BASE/api/v1/ingest/observations" -H "Authorization: Bearer $SIM" -H 'Content-Type: application/json' \
  -d '[{"asset_external_ref":"SIM-TEMP-A","capability":"temperature","value":88.1,"unit":"°C","source":"gate","dedupe_key":"gate-'"$(date +%s)"'"}]' >/dev/null
echo "incidents"
curl -sf -H "Authorization: Bearer $TOKEN" "$BASE/api/v1/incidents?status=open" | python3 -c 'import sys,json; d=json.load(sys.stdin); assert d, "no incident"; print(d[0]["id"])'
echo "ok"
