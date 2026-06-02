#!/bin/sh
# Integration tests for: erg install
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg install ==="

TDIR=$(mktemp -d)
trap 'rm -rf "$TDIR"' EXIT

REPO="$TDIR/repo"
mkdir -p "$REPO/tickets"
touch "$REPO/tickets/erg"

# --- install exists as a subcommand ---

if $ERG install --help >/dev/null 2>&1; then
    pass "install subcommand exists"
else
    fail "install subcommand does not exist"
fi

# --- install help starts with ## erg install ---

help_out=$($ERG install --help 2>/dev/null)
if echo "$help_out" | grep -q "^## erg install"; then
    pass "install help starts with '## erg install'"
else
    fail "install help header (got: $(echo "$help_out" | head -1))"
fi

# --- install with no flags does not mutate outside tickets/ ---

$ERG install "$REPO" >/dev/null 2>&1
if [ ! -f "$REPO/.git/hooks/pre-commit" ] && [ ! -f "$REPO/AGENTS.md" ]; then
    pass "install with no flags: no mutation outside tickets/"
else
    fail "install with no flags: created files outside tickets/"
fi

# --- --hooks flag is recognized ---

out=$($ERG install "$REPO" --hooks 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 0 ]; then
    pass "--hooks flag recognized (exit 0)"
else
    fail "--hooks flag not recognized (rc=$rc, out: $out)"
fi

# --- --inject-agents flag is recognized ---

out=$($ERG install "$REPO" --inject-agents 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 0 ]; then
    pass "--inject-agents flag recognized (exit 0)"
else
    fail "--inject-agents flag not recognized (rc=$rc, out: $out)"
fi

# --- both flags together are recognized ---

out=$($ERG install "$REPO" --hooks --inject-agents 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 0 ]; then
    pass "both flags together recognized (exit 0)"
else
    fail "both flags together not recognized (rc=$rc, out: $out)"
fi

# --- unknown flag rejected ---

out=$($ERG install --bogus 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "unknown flag"; then
    pass "unknown flag rejected with message"
else
    fail "unknown flag not rejected (rc=$rc, got: $out)"
fi

echo ""
echo "install: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
