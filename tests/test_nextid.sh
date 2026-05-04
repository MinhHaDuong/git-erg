#!/bin/sh
# Integration tests for: erg next-id
set -e

ERG="${ERG_BIN:-tickets/tools/go/erg}"
FIXTURES="tests/fixtures"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

NEXTID_DIR="$FIXTURES/nextid"
CUSTOM_DIR=""
trap 'rm -rf "$NEXTID_DIR"; [ -n "$CUSTOM_DIR" ] && rm -rf "$CUSTOM_DIR"' EXIT

echo "=== erg next-id ==="

# --- Missing dir prints 0001 ---
result=$($ERG next-id "$NEXTID_DIR/does_not_exist")
if [ "$result" = "0001" ]; then
    pass "missing dir prints 0001"
else
    fail "missing dir prints 0001 (got: $result)"
fi

# --- Empty dir prints 0001 ---
mkdir -p "$NEXTID_DIR/empty"
result=$($ERG next-id "$NEXTID_DIR/empty")
if [ "$result" = "0001" ]; then
    pass "empty dir prints 0001"
else
    fail "empty dir prints 0001 (got: $result)"
fi

# --- Single ticket 0042-foo.erg prints 0043 ---
mkdir -p "$NEXTID_DIR/single"
: > "$NEXTID_DIR/single/0042-foo.erg"
result=$($ERG next-id "$NEXTID_DIR/single")
if [ "$result" = "0043" ]; then
    pass "single ticket 0042 prints 0043"
else
    fail "single ticket 0042 prints 0043 (got: $result)"
fi

# --- Gap in sequence: 0001 and 0005 prints 0006 ---
mkdir -p "$NEXTID_DIR/gap"
: > "$NEXTID_DIR/gap/0001-a.erg"
: > "$NEXTID_DIR/gap/0005-b.erg"
result=$($ERG next-id "$NEXTID_DIR/gap")
if [ "$result" = "0006" ]; then
    pass "gap in sequence prints 0006"
else
    fail "gap in sequence prints 0006 (got: $result)"
fi

# --- Subdirectory entries not counted ---
mkdir -p "$NEXTID_DIR/subdir/archive"
: > "$NEXTID_DIR/subdir/0003-active.erg"
: > "$NEXTID_DIR/subdir/archive/0099-archived.erg"
result=$($ERG next-id "$NEXTID_DIR/subdir")
if [ "$result" = "0004" ]; then
    pass "subdirectory entries not counted"
else
    fail "subdirectory entries not counted (got: $result)"
fi

# --- Non-.erg files and non-numeric stems ignored ---
mkdir -p "$NEXTID_DIR/mixed"
: > "$NEXTID_DIR/mixed/0099-foo.txt"
: > "$NEXTID_DIR/mixed/notes.erg"
: > "$NEXTID_DIR/mixed/0010-real.erg"
result=$($ERG next-id "$NEXTID_DIR/mixed")
if [ "$result" = "0011" ]; then
    pass "non-.erg files and non-numeric stems ignored"
else
    fail "non-.erg files and non-numeric stems ignored (got: $result)"
fi

# --- Custom dir argument honored ---
CUSTOM_DIR=$(mktemp -d)
: > "$CUSTOM_DIR/0007-x.erg"
result=$($ERG next-id "$CUSTOM_DIR")
if [ "$result" = "0008" ]; then
    pass "custom dir argument honored"
else
    fail "custom dir argument honored (got: $result)"
fi

echo "nextid: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
