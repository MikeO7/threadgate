#!/usr/bin/env bash
# Smoke-test — prefer: ./scripts/hass-dev.sh test
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/src/manager"
exec go run ./cmd/hassdev test
