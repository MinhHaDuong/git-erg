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

# Test: the `verify:` hint (shown only for the in-repo bootstrap copy, a path
# ending in /tickets/erg) emits a ready-to-paste, correctly-quoted sha256sum
# command that ACTUALLY recomputes the displayed digest — run from an unrelated
# cwd, so a cwd-relative or unquoted command would fail. This guards both the
# single-quote escaping and the leading-separator path gate (ultra review).
BOOT="$WORKSPACE/bootstrap/tickets"
mkdir -p "$BOOT"
cp "$ERG_ABS" "$BOOT/erg"
VH_OUT=$(cd / && ERG_VERSION_NO_DISCOVER=1 "$BOOT/erg" version 2>&1)
VH_CMD=$(echo "$VH_OUT" | sed -n 's/^[[:space:]]*verify:[[:space:]]*//p')
VH_SHA=$(echo "$VH_OUT" | sed -n 's/^[[:space:]]*sha256:[[:space:]]*\([0-9a-f]\{64\}\)$/\1/p')
VH_GOT=$(cd / && eval "$VH_CMD" 2>/dev/null | cut -d' ' -f1)
if [ -n "$VH_CMD" ] && [ -n "$VH_SHA" ] && [ "$VH_GOT" = "$VH_SHA" ]; then
    pass "version: verify hint emits a working, correctly-quoted sha256sum command"
else
    fail "version: verify hint did not recompute the digest: cmd='$VH_CMD' got='$VH_GOT' want='$VH_SHA'"
fi

# Negative control for the path gate: a binary at .../my-tickets/erg must NOT
# show the verify hint — only a real ".../tickets/erg" (leading-separator) does.
# A bare-suffix check would wrongly fire here.
FAKE="$WORKSPACE/false/my-tickets"
mkdir -p "$FAKE"
cp "$ERG_ABS" "$FAKE/erg"
FAKE_OUT=$(cd / && ERG_VERSION_NO_DISCOVER=1 "$FAKE/erg" version 2>&1)
if echo "$FAKE_OUT" | grep -q '^[[:space:]]*verify:'; then
    fail "version: verify hint wrongly shown for a .../my-tickets/erg path: $FAKE_OUT"
else
    pass "version: verify hint correctly suppressed for non-bootstrap path"
fi

# unknown flag rejection (ticket 0185)
out=$($ERG version --bogus 2>&1) || rc=$?
if [ "${rc:-0}" -ne 0 ] && echo "$out" | grep -q "unknown flag"; then
    pass "unknown flag rejected with usage message"
else
    fail "unknown flag not rejected (rc=${rc:-0}, got: $out)"
fi

echo "version: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
