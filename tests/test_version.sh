#!/bin/sh
# Integration tests for: erg version discovers ./tickets/erg candidates
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0; FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg version candidate discovery ==="

# Test: erg version discovers a binary placed at ./tickets/erg in the CWD.
# We create a temp workspace, place a stub at tickets/erg, then run erg version
# from that workspace and assert the stub path appears in the output.
ERG_ABS=$(readlink -f "$ERG")
WORKSPACE=$(mktemp -d)

cleanup() { rm -rf "$WORKSPACE"; }
trap cleanup EXIT

mkdir -p "$WORKSPACE/tickets"
STUB="$WORKSPACE/tickets/erg"
cat > "$STUB" <<'STUBEOF'
#!/bin/sh
if [ "$1" = "version" ]; then
    echo "erg version"
    echo "  path:    /tmp/tickets/erg"
    echo "  hash:    aabbccddeeff"
    echo "  built:   2020-01-01T00:00:00Z"
    echo "  revision: olddeadbeef"
    echo "  arch:    linux/amd64"
fi
STUBEOF
chmod +x "$STUB"

OUT=$(cd "$WORKSPACE" && ERG_VERSION_NO_DISCOVER="" "$ERG_ABS" version 2>&1)

if echo "$OUT" | grep -qF "$WORKSPACE/tickets/erg"; then
    pass "version: discovers ./tickets/erg in CWD"
else
    fail "version: did not discover ./tickets/erg in CWD: $OUT"
fi

echo "version: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
