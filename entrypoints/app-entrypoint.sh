#!/usr/bin/env bash
set -euo pipefail

LOG_DIR="${LOG_DIR:-/app/logs/}"
SECRET_DIR="${JWT_SECRET_DIR:-/app/secret}"

mkdir -p "$LOG_DIR" "$SECRET_DIR" || true

if ! touch "$LOG_DIR/.w" 2>/dev/null; then
  echo "[entrypoint] ERROR: '$LOG_DIR' not writable" >&2
  exit 70
fi

rm -f "$LOG_DIR/.w" || true
if [ -w "$SECRET_DIR" ] && { [ ! -f "$SECRET_DIR/jwt_private.pem" ] || [ ! -f "$SECRET_DIR/jwt_public.pem" ]; }; then
  openssl genpkey -algorithm RSA -out "$SECRET_DIR/jwt_private.pem" -pkeyopt rsa_keygen_bits:2048
  openssl rsa -in "$SECRET_DIR/jwt_private.pem" -pubout -out "$SECRET_DIR/jwt_public.pem"
  chmod 600 "$SECRET_DIR/jwt_private.pem" || true
  chmod 644 "$SECRET_DIR/jwt_public.pem"  || true
elif [ ! -w "$SECRET_DIR" ]; then
  echo "[entrypoint] NOTICE: '$SECRET_DIR' is read-only; skipping key generatio"
fi

exec "$@"
