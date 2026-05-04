#!/bin/sh
# Integration tests for: erg next-id
set -e

ERG="${ERG_BIN:-tickets/tools/go/erg}"
FIXTURES="tests/fixtures"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

mkdir -p "$FIXTURES"
trap 'rm -rf "$FIXTURES"/nextid-*' EXIT

echo "=== erg next-id ==="

# --- Missing dir -> 0001 ---
out=$($ERG next-id "$FIXTURES/nextid-nonexistent" 2>/dev/null)
if [ "$out" = "0001" ]; then
    pass "missing dir returns 0001"
else
    fail "missing dir returns 0001 (got: $out)"
fi

# --- Empty dir -> 0001 ---
mkdir -p "$FIXTURES/nextid-empty"
out=$($ERG next-id "$FIXTURES/nextid-empty" 2>/dev/null)
if [ "$out" = "0001" ]; then
    pass "empty dir returns 0001"
else
    fail "empty dir returns 0001 (got: $out)"
fi

# --- Single ticket 0042-foo.erg -> 0043 ---
mkdir -p "$FIXTURES/nextid-single"
: > "$FIXTURES/nextid-single/0042-foo.erg"
out=$($ERG next-id "$FIXTURES/nextid-single" 2>/dev/null)
if [ "$out" = "0043" ]; then
    pass "single ticket 0042 returns 0043"
else
    fail "single ticket 0042 returns 0043 (got: $out)"
fi

# --- Gap in sequence -> 0006 ---
mkdir -p "$FIXTURES/nextid-gap"
: > "$FIXTURES/nextid-gap/0001-a.erg"
: > "$FIXTURES/nextid-gap/0005-b.erg"
out=$($ERG next-id "$FIXTURES/nextid-gap" 2>/dev/null)
if [ "$out" = "0006" ]; then
    pass "gap in sequence returns 0006"
else
    fail "gap in sequence returns 0006 (got: $out)"
fi

# --- Archive subdirectory NOT counted ---
mkdir -p "$FIXTURES/nextid-archive/archive"
: > "$FIXTURES/nextid-archive/0001-y.erg"
: > "$FIXTURES/nextid-archive/archive/0500-x.erg"
out=$($ERG next-id "$FIXTURES/nextid-archive" 2>/dev/null)
if [ "$out" = "0002" ]; then
    pass "archive subdir not counted"
else
    fail "archive subdir not counted (got: $out)"
fi

# --- Non-.erg files ignored ---
mkdir -p "$FIXTURES/nextid-nonerg"
: > "$FIXTURES/nextid-nonerg/0099-foo.txt"
: > "$FIXTURES/nextid-nonerg/notes.erg"
out=$($ERG next-id "$FIXTURES/nextid-nonerg" 2>/dev/null)
if [ "$out" = "0001" ]; then
    pass "non-.erg files ignored"
else
    fail "non-.erg files ignored (got: $out)"
fi

# --- Custom dir argument used ---
mkdir -p "$FIXTURES/nextid-custom"
: > "$FIXTURES/nextid-custom/0010-custom.erg"
out=$($ERG next-id "$FIXTURES/nextid-custom" 2>/dev/null)
if [ "$out" = "0011" ]; then
    pass "custom dir argument used"
else
    fail "custom dir argument used (got: $out)"
fi

echo "next-id: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
