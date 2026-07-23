#!/usr/bin/env bash
set -euo pipefail

LOG_DIR="${LOG_DIR:-/app/logs/}"

mkdir -p "$LOG_DIR" || true

if ! touch "$LOG_DIR/.w" 2>/dev/null; then
  echo "[entrypoint] ERROR: '$LOG_DIR' not writable" >&2
  exit 70
fi

rm -f "$LOG_DIR/.w" || true

exec "$@"
