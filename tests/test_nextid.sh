#!/bin/sh
# Integration tests for: erg next-id
set -e

ERG="${ERG_BIN:-tickets/tools/go/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "=== erg next-id ==="

# --- Missing dir → 0001 ---
out=$($ERG next-id "$TMPDIR/nonexistent")
if [ "$out" = "0001" ]; then
    pass "missing dir returns 0001"
else
    fail "missing dir returns 0001 (got: $out)"
fi

# --- Existing empty dir → 0001 ---
mkdir -p "$TMPDIR/empty"
out=$($ERG next-id "$TMPDIR/empty")
if [ "$out" = "0001" ]; then
    pass "empty dir returns 0001"
else
    fail "empty dir returns 0001 (got: $out)"
fi

# --- Single ticket 0042-foo.erg → 0043 ---
mkdir -p "$TMPDIR/single"
touch "$TMPDIR/single/0042-foo.erg"
out=$($ERG next-id "$TMPDIR/single")
if [ "$out" = "0043" ]; then
    pass "single ticket 0042 returns 0043"
else
    fail "single ticket 0042 returns 0043 (got: $out)"
fi

# --- Gap in sequence (0001, 0005) → 0006 ---
mkdir -p "$TMPDIR/gap"
touch "$TMPDIR/gap/0001-first.erg"
touch "$TMPDIR/gap/0005-fifth.erg"
out=$($ERG next-id "$TMPDIR/gap")
if [ "$out" = "0006" ]; then
    pass "gap in sequence returns 0006"
else
    fail "gap in sequence returns 0006 (got: $out)"
fi

# --- IDs in archive/ subdirectory NOT counted (non-recursive) ---
mkdir -p "$TMPDIR/scoped/archive"
touch "$TMPDIR/scoped/0003-top.erg"
touch "$TMPDIR/scoped/archive/0099-archived.erg"
out=$($ERG next-id "$TMPDIR/scoped")
if [ "$out" = "0004" ]; then
    pass "archive subdir not counted"
else
    fail "archive subdir not counted (got: $out)"
fi

# --- Non-.erg files ignored; non-numeric .erg files ignored ---
mkdir -p "$TMPDIR/mixed"
touch "$TMPDIR/mixed/0099-foo.txt"
touch "$TMPDIR/mixed/notes.erg"
touch "$TMPDIR/mixed/0010-real.erg"
out=$($ERG next-id "$TMPDIR/mixed")
if [ "$out" = "0011" ]; then
    pass "non-.erg and non-numeric .erg ignored"
else
    fail "non-.erg and non-numeric .erg ignored (got: $out)"
fi

# --- Custom dir argument is used (not hardcoded default) ---
mkdir -p "$TMPDIR/custom"
touch "$TMPDIR/custom/0020-ticket.erg"
out=$($ERG next-id "$TMPDIR/custom")
if [ "$out" = "0021" ]; then
    pass "custom dir argument used"
else
    fail "custom dir argument used (got: $out)"
fi

echo "next-id: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
