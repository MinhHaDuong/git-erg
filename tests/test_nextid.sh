#!/bin/sh
# Integration tests for: erg next-id
set -e

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

TDIR=$(mktemp -d)
trap 'rm -rf "$TDIR"' EXIT

echo "=== erg next-id ==="

# --- Missing dir → 0001 ---
out=$($ERG next-id "$TDIR/nonexistent")
if [ "$out" = "0001" ]; then
    pass "missing dir returns 0001"
else
    fail "missing dir returns 0001 (got: $out)"
fi

# --- Empty dir → 0001 ---
mkdir -p "$TDIR/empty"
out=$($ERG next-id "$TDIR/empty")
if [ "$out" = "0001" ]; then
    pass "empty dir returns 0001"
else
    fail "empty dir returns 0001 (got: $out)"
fi

# --- Single ticket 0042-foo.erg → 0043 ---
mkdir -p "$TDIR/single"
touch "$TDIR/single/0042-foo.erg"
out=$($ERG next-id "$TDIR/single")
if [ "$out" = "0043" ]; then
    pass "single ticket 0042 returns 0043"
else
    fail "single ticket 0042 returns 0043 (got: $out)"
fi

# --- Bare numeric filename 0042.erg → 0043 ---
mkdir -p "$TDIR/bare"
touch "$TDIR/bare/0042.erg"
out=$($ERG next-id "$TDIR/bare")
if [ "$out" = "0043" ]; then
    pass "bare numeric filename 0042.erg returns 0043"
else
    fail "bare numeric filename 0042.erg returns 0043 (got: $out)"
fi

# --- Gap in sequence (0001, 0005) → 0006 ---
mkdir -p "$TDIR/gap"
touch "$TDIR/gap/0001-alpha.erg"
touch "$TDIR/gap/0005-beta.erg"
out=$($ERG next-id "$TDIR/gap")
if [ "$out" = "0006" ]; then
    pass "gap sequence returns 0006"
else
    fail "gap sequence returns 0006 (got: $out)"
fi

# --- Closed/archive subdirectory IDs are NOT counted (non-recursive) ---
mkdir -p "$TDIR/scoped/archive"
touch "$TDIR/scoped/0003-low.erg"
touch "$TDIR/scoped/archive/0099-high.erg"
out=$($ERG next-id "$TDIR/scoped")
if [ "$out" = "0004" ]; then
    pass "archive subdir ignored"
else
    fail "archive subdir ignored (got: $out)"
fi

# --- Non-.erg files ignored ---
mkdir -p "$TDIR/filter"
touch "$TDIR/filter/0010-real.erg"
touch "$TDIR/filter/0099-fake.txt"
out=$($ERG next-id "$TDIR/filter")
if [ "$out" = "0011" ]; then
    pass "non-.erg file ignored"
else
    fail "non-.erg file ignored (got: $out)"
fi

# --- .erg file without numeric prefix ignored ---
mkdir -p "$TDIR/nonum"
touch "$TDIR/nonum/0005-valid.erg"
touch "$TDIR/nonum/notes.erg"
out=$($ERG next-id "$TDIR/nonum")
if [ "$out" = "0006" ]; then
    pass "non-numeric .erg file ignored"
else
    fail "non-numeric .erg file ignored (got: $out)"
fi

# --- Custom dir argument is used (not hardcoded default) ---
mkdir -p "$TDIR/custom"
touch "$TDIR/custom/0007-item.erg"
out=$($ERG next-id "$TDIR/custom")
if [ "$out" = "0008" ]; then
    pass "custom dir argument used"
else
    fail "custom dir argument used (got: $out)"
fi

echo "next-id: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
