#!/bin/sh
# Integration tests for: erg spec
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg spec ==="

# --- spec prints the format specification ---

out=$($ERG spec 2>/dev/null)
if echo "$out" | grep -q "%erg 0.1"; then
    pass "spec output contains '%erg 0.1'"
else
    fail "spec output missing '%erg 0.1' (got: $(echo "$out" | head -1))"
fi

# --- spec matches the embedded asset ---

first_line=$(echo "$out" | head -1)
if echo "$first_line" | grep -q "# Ticket format spec"; then
    pass "spec first line matches embedded spec-erg-v1.md"
else
    fail "spec first line mismatch (got: $first_line)"
fi

# --- spec exits 0 ---

$ERG spec > /dev/null 2>&1 && rc=0 || rc=$?
if [ "$rc" -eq 0 ]; then
    pass "spec exits 0"
else
    fail "spec exits $rc"
fi

# --- spec help starts with ## erg spec ---

help_out=$($ERG spec --help 2>/dev/null)
if echo "$help_out" | grep -q "^## erg spec"; then
    pass "spec help starts with '## erg spec'"
else
    fail "spec help header (got: $(echo "$help_out" | head -1))"
fi

echo ""
echo "spec: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
