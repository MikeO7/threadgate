#!/usr/bin/env bash
# Spin up / configure / test / tear down Home Assistant + ThreadGate for local dev.
#
#   make integration             # preferred: up + smoke tests
#   make integration-fresh       # reset → up → test
#
#   ./scripts/hass-dev.sh up              # start stack; manual pairing (dashboard approve)
#   ./scripts/hass-dev.sh up-auto         # same, but auto-approve pairing (CI / feature tests)
#   ./scripts/hass-dev.sh up --auto-approve
#   ./scripts/hass-dev.sh test            # smoke tests
#   ./scripts/hass-dev.sh cycle           # reset → up (manual) → test
#   ./scripts/hass-dev.sh cycle-auto      # reset → up-auto → test
#   PAIR_AUTO_APPROVE=1 ./scripts/hass-dev.sh up   # also enables auto-approve
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.integration.yml}"
COMPOSE=(docker compose -f "$COMPOSE_FILE")
FIXTURE="${HA_FIXTURE_FILE:-$ROOT/testdata/ha-fixture.tar.gz}"
HASSDEV=(go run ./src/manager/cmd/hassdev)

log() { printf '\n==> %s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

pair_auto_approve() {
  case "${PAIR_AUTO_APPROVE:-}" in
    1|true|yes|on) return 0 ;;
  esac
  return 1
}

usage() {
  cat <<'EOF'
Usage: hass-dev.sh <command>

Commands:
  up [--auto-approve]   Start stack + configure HA (pairing: manual by default)
  up-auto               Same as up --auto-approve
  test                  Run smoke tests
  verify                Deep check OTBR loaded, dataset, pairing
  repair-otbr           Reload or recreate failed HA OTBR entry
  down                  Stop containers (keep testdata)
  reset                 Wipe local HA/ThreadGate test state
  cycle                 reset → up (manual pair) → test
  cycle-auto            reset → up-auto → test
  fixture-build         Save golden HA snapshot after a good up
  logs [ha|tg|all]      Follow container logs

Pairing:
  Manual (default): open http://127.0.0.1:8081 and click Approve Connection
  Auto:             up-auto, cycle-auto, or PAIR_AUTO_APPROVE=1 ./scripts/hass-dev.sh up
  Test popup only:  ./scripts/hass-dev.sh pair-initiate (then open dashboard)
EOF
}

hassdev() {
  (
    cd "$ROOT/src/manager"
    export HA_CONFIG_DIR="$ROOT/testdata/ha-config"
    export CREDS_FILE="$ROOT/testdata/ha-credentials.env"
    export HA_FIXTURE_FILE="$FIXTURE"
    go run ./cmd/hassdev "$@"
  )
}

cmd_down() {
  log "Stopping integration stack"
  "${COMPOSE[@]}" down --remove-orphans 2>/dev/null || true
  docker rm -f threadgate-test 2>/dev/null || true
}

cmd_reset() {
  cmd_down
  log "Removing local test state"
  rm -rf "$ROOT/testdata/ha-config" "$ROOT/testdata/ha-credentials.env" "$ROOT/src/manager/testdata"
  rm -f "$ROOT/data/hass_config.json"
  mkdir -p "$ROOT/testdata/ha-config" "$ROOT/data"
}

cmd_up() {
  mkdir -p testdata/ha-config data

  if [ -f "$FIXTURE" ]; then
    log "Starting ThreadGate (--no-deps; HA starts after fixture apply)"
    "${COMPOSE[@]}" up -d --build --no-deps threadgate
    hassdev wait-tg

    log "Applying golden Home Assistant fixture (HA must stay stopped)"
    "${COMPOSE[@]}" stop homeassistant 2>/dev/null || true
    hassdev fixture-apply

    log "Starting Home Assistant"
    "${COMPOSE[@]}" up -d homeassistant
    hassdev wait-ha
  else
    log "Starting containers (first-time setup)"
    "${COMPOSE[@]}" up -d --build
    hassdev wait-tg
    hassdev wait-ha
    hassdev setup --configure-otbr
    "${COMPOSE[@]}" stop homeassistant
    hassdev seed-devices
    log "Starting Home Assistant after device seed"
    "${COMPOSE[@]}" up -d homeassistant
    hassdev wait-ha
    log "Saving golden fixture (speeds up future ./scripts/hass-dev.sh up)"
    hassdev fixture-build
  fi

  log "Setting Home Assistant country/location (default US)"
  hassdev ensure-core

  log "Pairing ThreadGate with Home Assistant"
  if pair_auto_approve; then
    log "Pairing mode: auto-approve (API)"
    hassdev pair --auto-approve
  else
    log "Pairing mode: manual — open http://127.0.0.1:8081 and approve the banner"
    if [ -f "$ROOT/data/hass_config.json" ]; then
      log "Clearing old pairing so the dashboard prompt appears (or use: hassdev pair-initiate)"
      hassdev pair --force
    else
      hassdev pair
    fi
  fi

  log "Ensuring Home Assistant OTBR integration points at ThreadGate"
  hassdev ensure-otbr

  log "Stack ready"
  printf '  Home Assistant:  http://127.0.0.1:8123\n'
  printf '  ThreadGate:        http://127.0.0.1:8081\n'
  printf '  Credentials:       %s/testdata/ha-credentials.env\n' "$ROOT"
}

cmd_fixture_build() {
  [ -d "$ROOT/testdata/ha-config/.storage" ] || die "no ha-config — run hass-dev.sh up first"
  hassdev fixture-build
  log "Golden fixture saved to $FIXTURE"
}

cmd_test() {
  hassdev test
}

cmd_logs() {
  local target="${1:-all}"
  case "$target" in
    ha|homeassistant) "${COMPOSE[@]}" logs -f homeassistant ;;
    threadgate|tg) "${COMPOSE[@]}" logs -f threadgate ;;
    all|"") "${COMPOSE[@]}" logs -f ;;
    *) die "unknown logs target: $target" ;;
  esac
}

cmd_cycle() {
  cmd_reset
  cmd_up
  cmd_test
}

cmd_up_auto() {
  PAIR_AUTO_APPROVE=1 cmd_up
}

cmd_cycle_auto() {
  cmd_reset
  PAIR_AUTO_APPROVE=1 cmd_up
  cmd_test
}

main() {
  local cmd="${1:-}"
  shift || true
  case "$cmd" in
    up)
      for arg in "$@"; do
        case "$arg" in
          --auto-approve|-a) PAIR_AUTO_APPROVE=1 ;;
          *) die "unknown option for up: $arg" ;;
        esac
      done
      cmd_up
      ;;
    up-auto) PAIR_AUTO_APPROVE=1; cmd_up ;;
    test) cmd_test ;;
    verify) hassdev verify ;;
    repair-otbr) hassdev repair-otbr ;;
    down) cmd_down ;;
    reset) cmd_reset ;;
    cycle) cmd_cycle ;;
    cycle-auto) cmd_cycle_auto ;;
    pair-initiate) hassdev pair-initiate ;;
    fixture-build) cmd_fixture_build ;;
    logs) cmd_logs "${1:-all}" ;;
    -h|--help|help) usage ;;
    "") usage; exit 2 ;;
    *) die "unknown command: $cmd (try --help)" ;;
  esac
}

main "$@"
