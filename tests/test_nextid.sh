#!/bin/sh
# Integration tests for: erg next-id
set -e

ERG="${ERG_BIN:-tickets/tools/go/erg}"
FIXTURES="tests/fixtures/nextid"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

mkdir -p "$FIXTURES"
trap 'rm -rf "$FIXTURES"' EXIT

echo "=== erg next-id ==="

# --- Nonexistent directory → outputs 0001 ---
rm -rf "$FIXTURES"
OUT=$($ERG next-id "$FIXTURES")
if [ "$OUT" = "0001" ]; then
    pass "nonexistent directory outputs 0001"
else
    fail "nonexistent directory outputs 0001 (got: $OUT)"
fi

# --- Empty directory → outputs 0001 ---
mkdir -p "$FIXTURES"
OUT=$($ERG next-id "$FIXTURES")
if [ "$OUT" = "0001" ]; then
    pass "empty directory outputs 0001"
else
    fail "empty directory outputs 0001 (got: $OUT)"
fi

# --- Single ticket 0042-foo.erg → outputs 0043 ---
rm -rf "$FIXTURES" && mkdir -p "$FIXTURES"
touch "$FIXTURES/0042-foo.erg"
OUT=$($ERG next-id "$FIXTURES")
if [ "$OUT" = "0043" ]; then
    pass "single ticket 0042 outputs 0043"
else
    fail "single ticket 0042 outputs 0043 (got: $OUT)"
fi

# --- Gap in sequence (0001 and 0005 only) → outputs 0006 ---
rm -rf "$FIXTURES" && mkdir -p "$FIXTURES"
touch "$FIXTURES/0001-a.erg"
touch "$FIXTURES/0005-b.erg"
OUT=$($ERG next-id "$FIXTURES")
if [ "$OUT" = "0006" ]; then
    pass "gap in sequence outputs 0006"
else
    fail "gap in sequence outputs 0006 (got: $OUT)"
fi

# --- ID lives only in archive/ → still respected ---
rm -rf "$FIXTURES" && mkdir -p "$FIXTURES" "$FIXTURES/archive"
touch "$FIXTURES/0005-a.erg"
touch "$FIXTURES/archive/0010-old.erg"
OUT=$($ERG next-id "$FIXTURES")
if [ "$OUT" = "0011" ]; then
    pass "archive ID respected outputs 0011"
else
    fail "archive ID respected outputs 0011 (got: $OUT)"
fi

# --- Non-.erg files ignored ---
rm -rf "$FIXTURES" && mkdir -p "$FIXTURES"
touch "$FIXTURES/README.md"
touch "$FIXTURES/0099-notes.txt"
touch "$FIXTURES/0003-real.erg"
OUT=$($ERG next-id "$FIXTURES")
if [ "$OUT" = "0004" ]; then
    pass "non-.erg files ignored"
else
    fail "non-.erg files ignored (got: $OUT)"
fi

# --- Non-numeric prefix ignored ---
rm -rf "$FIXTURES" && mkdir -p "$FIXTURES"
touch "$FIXTURES/abc-foo.erg"
touch "$FIXTURES/0002-bar.erg"
OUT=$($ERG next-id "$FIXTURES")
if [ "$OUT" = "0003" ]; then
    pass "non-numeric prefix ignored"
else
    fail "non-numeric prefix ignored (got: $OUT)"
fi

echo "next-id: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
