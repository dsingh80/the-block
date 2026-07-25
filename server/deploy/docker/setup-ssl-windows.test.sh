#!/usr/bin/env bash
# Black-box test for setup-ssl-windows.sh / cleanup-ssl-windows.sh: runs them
# as real subprocesses and asserts on their observable effects (files
# created/removed, exit codes, output) -- the same black-box-via-subprocess
# approach scripts/generate_vehicles.test.mjs uses for the JS generator
# (guidelines/06-backend-architecture.md). Requires mkcert on PATH. Not part
# of `go test ./...` -- these are shell scripts, not Go code. Run manually:
#   bash deploy/docker/setup-ssl-windows.test.sh
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERT_DIR="$SCRIPT_DIR/certs"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); }
fail() { FAIL=$((FAIL + 1)); echo "FAIL: $1"; }

assert_exists() { [[ -s "$1" ]] && pass || fail "$1 does not exist (or is empty)"; }
assert_absent() { [[ -e "$1" ]] && fail "$1 still exists" || pass; }
assert_contains() { [[ "$1" == *"$2"* ]] && pass || fail "expected output to contain '$2': $1"; }
assert_not_contains() { [[ "$1" == *"$2"* ]] && fail "expected output NOT to contain '$2': $1" || pass; }

if ! command -v mkcert >/dev/null 2>&1; then
  echo "mkcert is not on PATH -- these scripts can't be exercised for real. Skipping." >&2
  exit 0
fi

# --- setup produces valid cert/key files Caddy can load ---
"$SCRIPT_DIR/setup-ssl-windows.sh"

assert_exists "$CERT_DIR/localhost.pem"
assert_exists "$CERT_DIR/localhost-key.pem"

SAN=$(openssl x509 -in "$CERT_DIR/localhost.pem" -noout -text 2>/dev/null | grep -A1 "Subject Alternative Name")
assert_contains "$SAN" "localhost"

# --- cleanup with no flags removes the generated files, leaves the CA alone ---
CLEANUP_OUTPUT=$("$SCRIPT_DIR/cleanup-ssl-windows.sh" 2>&1)

assert_absent "$CERT_DIR/localhost.pem"
assert_absent "$CERT_DIR/localhost-key.pem"
assert_not_contains "$CLEANUP_OUTPUT" "uninstall"

# --- --uninstall-ca is gated behind the flag, and does attempt a real removal ---
# Exercised with mkcert hidden from a minimal PATH (coreutils only) so this
# never actually touches the machine's trust store -- proves the flag reaches
# the uninstall branch without performing the real (machine-wide,
# hard-to-undo-in-a-test) removal.
"$SCRIPT_DIR/setup-ssl-windows.sh" >/dev/null # recreate files for a clean starting state
UNINSTALL_OUTPUT=$(PATH="/usr/bin:/bin" "$SCRIPT_DIR/cleanup-ssl-windows.sh" --uninstall-ca 2>&1)
assert_contains "$UNINSTALL_OUTPUT" "nothing to uninstall"

# The --uninstall-ca run above still removed the cert/key files (that part of
# cleanup runs regardless of the flag) -- restore them so this test leaves the
# repo in the same state it found it in.
"$SCRIPT_DIR/setup-ssl-windows.sh" >/dev/null
"$SCRIPT_DIR/cleanup-ssl-windows.sh" >/dev/null

echo ""
echo "$PASS passed, $FAIL failed"
[[ "$FAIL" -eq 0 ]]
