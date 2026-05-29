#!/usr/bin/env bash
# Thin wrapper — prefer ./scripts/hass-dev.sh or go run ./src/manager/cmd/hassdev
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ARGS=()
while [ $# -gt 0 ]; do
  case "$1" in
    --seed-devices) ARGS+=(--seed-devices) ;;
    --configure-otbr) ARGS+=(--configure-otbr) ;;
    --reset)
      docker compose -f "$ROOT/docker-compose.integration.yml" stop homeassistant 2>/dev/null || true
      rm -rf "$ROOT/testdata/ha-config" "$ROOT/testdata/ha-credentials.env"
      mkdir -p "$ROOT/testdata/ha-config"
      ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
  shift
done
(cd "$ROOT/src/manager" && go run ./cmd/hassdev wait-ha)
docker compose -f "$ROOT/docker-compose.integration.yml" stop homeassistant 2>/dev/null || true
(cd "$ROOT/src/manager" && go run ./cmd/hassdev setup "${ARGS[@]}")
docker compose -f "$ROOT/docker-compose.integration.yml" start homeassistant 2>/dev/null || true
(cd "$ROOT/src/manager" && go run ./cmd/hassdev wait-ha)
