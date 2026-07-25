#!/usr/bin/env bash
# Generates the local-HTTPS cert/key Caddy loads for https://localhost
# (guidelines/06-backend-architecture.md, "Deployment & HTTPS"). Run once
# before `docker compose up`; safe to re-run any time -- mkcert -install is a
# no-op once the local CA is already trusted, and this always regenerates the
# cert/key pair from scratch.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERT_DIR="$SCRIPT_DIR/certs"

if ! command -v mkcert >/dev/null 2>&1; then
  echo "mkcert is not installed (or not on PATH)." >&2
  echo "Install it, e.g.: winget install --id=FiloSottile.mkcert -e" >&2
  exit 1
fi

# Installs mkcert's local CA into the Windows + browser trust stores -- a
# host-level operation Docker Compose has no way to perform itself.
mkcert -install

mkdir -p "$CERT_DIR"
mkcert -cert-file "$CERT_DIR/localhost.pem" -key-file "$CERT_DIR/localhost-key.pem" localhost 127.0.0.1 ::1

echo "Certs written to $CERT_DIR -- ready for \`docker compose up\`."
