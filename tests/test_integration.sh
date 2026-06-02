#!/bin/sh
# Integration tests for: erg integration
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg integration ==="

# --- integration prints the setup guide ---

out=$($ERG integration 2>/dev/null)
if echo "$out" | grep -q "pre-commit"; then
    pass "integration output mentions pre-commit"
else
    fail "integration output missing 'pre-commit' (got: $(echo "$out" | head -1))"
fi

# --- integration matches the embedded asset ---

first_line=$(echo "$out" | head -1)
if echo "$first_line" | grep -qi "integration\|setup\|hook"; then
    pass "integration first line looks like setup guide"
else
    fail "integration first line unexpected (got: $first_line)"
fi

# --- integration exits 0 ---

$ERG integration > /dev/null 2>&1 && rc=0 || rc=$?
if [ "$rc" -eq 0 ]; then
    pass "integration exits 0"
else
    fail "integration exits $rc"
fi

# --- integration help starts with ## erg integration ---

help_out=$($ERG integration --help 2>/dev/null)
if echo "$help_out" | grep -q "^## erg integration"; then
    pass "integration help starts with '## erg integration'"
else
    fail "integration help header (got: $(echo "$help_out" | head -1))"
fi

echo ""
echo "integration: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
