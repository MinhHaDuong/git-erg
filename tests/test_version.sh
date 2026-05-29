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
    echo "  sha256:  aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
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

# Test: live erg version prints the full SHA-256 digest, named sha256:, and that
# digest is the REAL hash of the binary — recomputable with stock tools. Shape
# alone is not enough: the security requirement is recomputability, so compare
# the printed value byte-for-byte against sha256sum of the same binary.
# Run with discovery suppressed so only the running binary's line is matched.
SELF_OUT=$(ERG_VERSION_NO_DISCOVER=1 "$ERG_ABS" version 2>&1)
PRINTED=$(echo "$SELF_OUT" | sed -n 's/^[[:space:]]*sha256:[[:space:]]*\([0-9a-f]\{64\}\)$/\1/p')
ACTUAL=$(sha256sum "$ERG_ABS" | cut -d' ' -f1)
if [ -n "$PRINTED" ] && [ "$PRINTED" = "$ACTUAL" ]; then
    pass "version: printed sha256 equals sha256sum of the binary (recomputable)"
else
    fail "version: printed sha256 ('$PRINTED') != sha256sum ('$ACTUAL'): $SELF_OUT"
fi

echo "version: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
