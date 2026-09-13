#!/usr/bin/env bash
# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: Apache-2.0
#
# Deploy Yard to a remote host as a systemd service.
#
#   ./scripts/deploy-remote.sh USER@HOST
#   ./scripts/deploy-remote.sh USER@HOST --with-sim
#   ./scripts/deploy-remote.sh USER@HOST --full
#   ./scripts/deploy-remote.sh USER@HOST --port 8080
#   ./scripts/deploy-remote.sh USER@HOST --uninstall
#   ./scripts/deploy-remote.sh USER@HOST --dry-run
#
# Env: DEPLOY_HOST, DEPLOY_USER, DEPLOY_PASS, YARD_PORT
# Exit: 0 OK · 1 fail · 2 misconfig
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_DIR"
exec </dev/null

REMOTE_BIN=/usr/local/bin/yard
REMOTE_SIM=/usr/local/bin/yard-simulator
REMOTE_GW=/usr/local/bin/yard-agent-gateway
REMOTE_DIR=/etc/yard
REMOTE_ENV=/etc/yard/yard.env
REMOTE_DATA=/var/lib/yard
REMOTE_UNIT=/etc/systemd/system/yard.service
REMOTE_SIM_UNIT=/etc/systemd/system/yard-simulator.service

WITH_SIM=0
FULL=0
UNINSTALL=0
DRY_RUN=0
SKIP_VERIFY=0
PORT_FROM_CLI=""
TARGET=""
PASS="${DEPLOY_PASS:-}"

usage() {
  sed -n '2,18p' "$0" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --with-sim) WITH_SIM=1; shift ;;
    --full) FULL=1; shift ;;
    --uninstall) UNINSTALL=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    --skip-verify) SKIP_VERIFY=1; shift ;;
    --port)
      [[ $# -ge 2 ]] || { echo "misconfig: --port needs a value" >&2; exit 2; }
      PORT_FROM_CLI="$2"; shift 2
      ;;
    --port=*) PORT_FROM_CLI="${1#*=}"; shift ;;
    -*)
      echo "misconfig: unknown flag $1" >&2
      usage >&2
      exit 2
      ;;
    *)
      if [[ -n "$TARGET" ]]; then
        # allow host user [pass] positional style
        if [[ -z "${_pos_user:-}" ]]; then
          _pos_user="$1"
        elif [[ -z "$PASS" ]]; then
          PASS="$1"
        else
          echo "misconfig: unexpected argument $1" >&2
          exit 2
        fi
        shift
        continue
      fi
      TARGET="$1"
      shift
      ;;
  esac
done

TARGET="${TARGET:-${DEPLOY_HOST:-}}"
if [[ -z "$TARGET" && -f "$REPO_DIR/.deploy-last" ]]; then
  # shellcheck disable=SC1091
  source "$REPO_DIR/.deploy-last"
  TARGET="${USER}@${HOST}"
  echo "Using .deploy-last → ${TARGET}"
fi
[[ -n "$TARGET" ]] || { echo "misconfig: missing USER@HOST" >&2; usage >&2; exit 2; }

if [[ "$TARGET" == *@* ]]; then
  SSH_USER="${TARGET%@*}"
  SSH_HOST="${TARGET#*@}"
else
  SSH_USER="${_pos_user:-${DEPLOY_USER:-root}}"
  SSH_HOST="$TARGET"
fi
# Prefer explicit second positional user if given as HOST USER
if [[ -n "${_pos_user:-}" && "$TARGET" != *@* ]]; then
  SSH_USER="$_pos_user"
fi
SSH_TARGET="${SSH_USER}@${SSH_HOST}"

LAST_PORT=""
if [[ -f "$REPO_DIR/.deploy-last" ]]; then
  LAST_PORT="$(awk -F= '/^PORT=/ {print $2; exit}' "$REPO_DIR/.deploy-last" || true)"
fi
if [[ -n "$PORT_FROM_CLI" ]]; then
  YARD_PORT="$PORT_FROM_CLI"
elif [[ -n "${YARD_PORT:-}" ]]; then
  :
elif [[ -n "$LAST_PORT" ]]; then
  YARD_PORT="$LAST_PORT"
  echo "Reusing port ${YARD_PORT} from .deploy-last"
else
  YARD_PORT=18080
fi
case "$YARD_PORT" in
  ''|*[!0-9]*) echo "misconfig: invalid port ${YARD_PORT}" >&2; exit 2 ;;
esac

info() { printf '==> %s\n' "$*"; }
die() { echo "FAIL: $*" >&2; exit 1; }

[[ -f "$REPO_DIR/go.mod" ]] || die "not in yard repo"
[[ -d "$REPO_DIR/cmd/yard" ]] || die "cmd/yard missing"

SUDO=""
[[ "$SSH_USER" != root ]] && SUDO="sudo"

SSH_OPTS=(-o StrictHostKeyChecking=accept-new -o ConnectTimeout=15 -o BatchMode=yes)
if [[ -n "$PASS" ]]; then
  command -v sshpass >/dev/null || die "sshpass required for password auth"
  SSH_OPTS=(-o StrictHostKeyChecking=accept-new -o ConnectTimeout=15)
fi

_ssh() {
  if [[ -n "$PASS" ]]; then
    SSHPASS="$PASS" sshpass -e ssh "${SSH_OPTS[@]}" "$SSH_TARGET" "$@"
  else
    ssh "${SSH_OPTS[@]}" "$SSH_TARGET" "$@"
  fi
}

_scp() {
  if [[ -n "$PASS" ]]; then
    SSHPASS="$PASS" sshpass -e scp "${SSH_OPTS[@]}" "$@"
  else
    scp "${SSH_OPTS[@]}" "$@"
  fi
}

if [[ "$DRY_RUN" -eq 1 ]]; then
  echo "[dry-run] target=${SSH_TARGET} port=${YARD_PORT} with-sim=${WITH_SIM} full=${FULL}"
  echo "[dry-run] would: detect arch → cross-compile → install systemd → verify /healthz"
  exit 0
fi

if [[ "$UNINSTALL" -eq 1 ]]; then
  info "Uninstalling Yard from ${SSH_TARGET}"
  _ssh "
    $SUDO systemctl disable --now yard-simulator.service 2>/dev/null || true
    $SUDO systemctl disable --now yard.service 2>/dev/null || true
    $SUDO rm -f $REMOTE_BIN $REMOTE_SIM $REMOTE_GW $REMOTE_UNIT $REMOTE_SIM_UNIT
    $SUDO rm -rf $REMOTE_DIR
    $SUDO systemctl daemon-reload 2>/dev/null || true
  "
  info "Removed binaries/units (data at ${REMOTE_DATA} left in place)"
  exit 0
fi

info "Detecting remote architecture"
REMOTE_ARCH_RAW="$(_ssh "uname -m" | tr -d '\r')"
case "$REMOTE_ARCH_RAW" in
  x86_64) GOARCH=amd64 ;;
  aarch64|arm64) GOARCH=arm64 ;;
  *) die "unsupported remote arch: ${REMOTE_ARCH_RAW}" ;;
esac
info "Remote linux/${GOARCH}"

info "Building web console + cross-compiling for linux/${GOARCH}"
BUILD_DIR="$(mktemp -d)"
trap 'rm -rf "$BUILD_DIR"' EXIT
(
  cd "$REPO_DIR"
  if [[ -d web ]]; then
    (cd web && npm ci --silent 2>/dev/null || npm install --silent)
    (cd web && npm run build --silent)
    rm -rf cmd/yard/static
    cp -R web/dist cmd/yard/static
  fi
  CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" \
    go build -trimpath -ldflags="-s -w" -o "$BUILD_DIR/yard" ./cmd/yard
  CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" \
    go build -trimpath -ldflags="-s -w" -o "$BUILD_DIR/yard-simulator" ./cmd/simulator
  CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" \
    go build -trimpath -ldflags="-s -w" -o "$BUILD_DIR/yard-agent-gateway" ./cmd/agent-gateway
)

info "Installing on ${SSH_TARGET}"
EXISTING_LISTEN="$(_ssh "grep -E '^YARD_LISTEN=' $REMOTE_ENV 2>/dev/null | cut -d= -f2- || true" | tr -d '\r' || true)"
if [[ -n "$EXISTING_LISTEN" && "$FULL" -eq 0 && -z "$PORT_FROM_CLI" && -z "${YARD_PORT_FORCE:-}" ]]; then
  # keep prior listen if redeploy without --port
  :
fi

cat > "$BUILD_DIR/yard.env" <<ENVEOF
YARD_LISTEN=0.0.0.0:${YARD_PORT}
YARD_DATABASE_URL=file:${REMOTE_DATA}/yard.db?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)
YARD_DATA_DIR=${REMOTE_DATA}
YARD_URL=http://${SSH_HOST}:${YARD_PORT}
YARD_SIMULATOR_TOKEN_FILE=${REMOTE_DATA}/simulator.token
YARD_INGEST_TOKEN_FILE=${REMOTE_DATA}/ingest.token
YARD_SIM_TRIP=auto
DEVICE_AGENT_URL=http://127.0.0.1:9188
ENVEOF

_scp "$BUILD_DIR/yard" "${SSH_TARGET}:/tmp/yard.new"
_scp "$BUILD_DIR/yard-simulator" "${SSH_TARGET}:/tmp/yard-simulator.new"
_scp "$BUILD_DIR/yard-agent-gateway" "${SSH_TARGET}:/tmp/yard-agent-gateway.new"
_scp "$REPO_DIR/systemd/yard.service" "${SSH_TARGET}:/tmp/yard.service.new"
_scp "$REPO_DIR/systemd/yard-simulator.service" "${SSH_TARGET}:/tmp/yard-simulator.service.new"
_scp "$BUILD_DIR/yard.env" "${SSH_TARGET}:/tmp/yard.env.new"

_ssh "
  set -euo pipefail
  $SUDO mkdir -p $REMOTE_DIR $REMOTE_DATA
  $SUDO install -m 755 /tmp/yard.new $REMOTE_BIN
  $SUDO install -m 755 /tmp/yard-simulator.new $REMOTE_SIM
  $SUDO install -m 755 /tmp/yard-agent-gateway.new $REMOTE_GW
  $SUDO install -m 640 /tmp/yard.env.new $REMOTE_ENV
  $SUDO install -m 644 /tmp/yard.service.new $REMOTE_UNIT
  $SUDO install -m 644 /tmp/yard-simulator.service.new $REMOTE_SIM_UNIT
  rm -f /tmp/yard.new /tmp/yard-simulator.new /tmp/yard-agent-gateway.new /tmp/yard.env.new /tmp/yard.service.new /tmp/yard-simulator.service.new
"

info "Starting yard.service"
_ssh "
  set -euo pipefail
  # Estate (pre-rename) may still own the listen port on lab hosts.
  $SUDO systemctl disable --now estate.service estate-simulator.service 2>/dev/null || true
  $SUDO systemctl daemon-reload
  $SUDO systemctl enable --now yard.service
  $SUDO systemctl restart yard.service
  if [[ $WITH_SIM -eq 1 ]]; then
    # wait for token file from bootstrap
    for i in 1 2 3 4 5 6 7 8 9 10; do
      [[ -f $REMOTE_DATA/simulator.token ]] && break
      sleep 1
    done
    $SUDO systemctl enable --now yard-simulator.service
    $SUDO systemctl restart yard-simulator.service
  fi
  if [[ $FULL -eq 1 ]]; then
    if command -v firewall-cmd &>/dev/null; then
      $SUDO firewall-cmd --permanent --add-port=${YARD_PORT}/tcp 2>/dev/null || true
      $SUDO firewall-cmd --reload 2>/dev/null || true
    elif command -v ufw &>/dev/null; then
      $SUDO ufw allow ${YARD_PORT}/tcp 2>/dev/null || true
    fi
  fi
  sleep 1
  if $SUDO systemctl is-active --quiet yard.service && curl -fsS "http://127.0.0.1:${YARD_PORT}/healthz" | grep -qx ok; then
    echo 'yard.service: running'
  else
    echo 'yard.service: FAILED'
    $SUDO journalctl -u yard.service --no-pager -n 40
    exit 1
  fi
"

BASE_URL="http://${SSH_HOST}:${YARD_PORT}"
if [[ "$SKIP_VERIFY" -eq 0 ]]; then
  info "Verifying"
  BODY="$(_ssh "curl -fsS http://127.0.0.1:${YARD_PORT}/healthz" | tr -d '\r' || true)"
  if [[ "$BODY" != "ok" ]]; then
    die "healthz expected plain 'ok' from Yard, got: ${BODY:-<empty>} (is port ${YARD_PORT} already taken?)"
  fi
  info "On-host health OK"
  if curl -fsS --connect-timeout 5 "${BASE_URL}/healthz" 2>/dev/null | grep -qx 'ok'; then
    info "External health OK ${BASE_URL}/healthz"
  else
    info "On-host health OK (external ${BASE_URL} may be firewalled; use --full once)"
  fi
  SIM_TOK="$(_ssh "sudo cat ${REMOTE_DATA}/simulator.token 2>/dev/null" | tr -d '\r\n' || true)"
  if [[ -n "$SIM_TOK" ]]; then
    if YARD_URL="$BASE_URL" YARD_SIMULATOR_TOKEN="$SIM_TOK" "$REPO_DIR/scripts/verify-remote.sh"; then
      info "Remote verify OK"
    else
      die "verify-remote failed against ${BASE_URL}"
    fi
  elif [[ -x "$REPO_DIR/scripts/verify-remote.sh" ]]; then
    YARD_URL="$BASE_URL" "$REPO_DIR/scripts/verify-remote.sh" || die "verify-remote failed"
  fi
fi

cat > "$REPO_DIR/.deploy-last" <<EOF
# Auto-generated by yard deploy-remote (no passwords)
HOST=${SSH_HOST}
USER=${SSH_USER}
PORT=${YARD_PORT}
MODE=$([ "$FULL" -eq 1 ] && echo full || echo quick)
WITH_SIM=${WITH_SIM}
UPDATED=$(date -u +%Y-%m-%dT%H:%M:%SZ)
EOF

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║  Yard shipped → ${SSH_TARGET}"
echo "║  UI           ${BASE_URL}"
echo "║  Health       ${BASE_URL}/healthz"
echo "║  Login        admin@yard.local / yard-admin"
echo "║  Redeploy     ./scripts/ship ${SSH_TARGET}"
if [[ "$WITH_SIM" -eq 1 ]]; then
  echo "║  Simulator    yard-simulator.service enabled"
fi
echo "╚══════════════════════════════════════════════════════════╝"
info "PASS: yard deployed"
