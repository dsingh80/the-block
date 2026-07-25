#!/usr/bin/env bash
# Removes the generated local-HTTPS cert/key. Never touches the machine-wide
# mkcert CA trust store unless --uninstall-ca is passed explicitly -- that's a
# much bigger, easy-to-miss change (mkcert's CA is shared across every
# local-HTTPS project on this machine, not just this one), so it's not the
# default behavior of "clean up after this project"
# (guidelines/06-backend-architecture.md, "Deployment & HTTPS").
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERT_DIR="$SCRIPT_DIR/certs"

rm -f "$CERT_DIR/localhost.pem" "$CERT_DIR/localhost-key.pem"
echo "Removed generated cert/key from $CERT_DIR."

if [[ "${1:-}" == "--uninstall-ca" ]]; then
  if ! command -v mkcert >/dev/null 2>&1; then
    echo "mkcert is not installed (or not on PATH) -- nothing to uninstall." >&2
    exit 1
  fi
  echo "Removing mkcert's local CA from the machine-wide trust store..."
  mkcert -uninstall
fi
