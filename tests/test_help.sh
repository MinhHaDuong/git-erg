#!/bin/sh
# Integration tests for: erg help output (encoding and completeness)
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg help ==="

# --- -h: exit 0 ---
if out=$("$ERG" -h 2>&1); then
    pass "-h: exit 0"
else
    fail "-h: expected exit 0"
fi

# --- --help: exit 0 ---
if out=$("$ERG" --help 2>&1); then
    pass "--help: exit 0"
else
    fail "--help: expected exit 0"
fi

# --- help: exit 0 ---
if out=$("$ERG" help 2>&1); then
    pass "help cmd: exit 0"
else
    fail "help cmd: expected exit 0"
fi

# Capture help output once for remaining assertions
help_out=$("$ERG" -h 2>&1)

# --- each canonical command name appears in help output ---
for cmd in validate check ready next-id new close log archive migrate init version update; do
    if echo "$help_out" | grep -q "$cmd"; then
        pass "help mentions command: $cmd"
    else
        fail "help missing command: $cmd"
    fi
done

# --- no angle brackets in help output ---
if echo "$help_out" | grep -q '[<>]'; then
    fail "help output contains angle brackets"
else
    pass "help output: no angle brackets"
fi

for cmd in close validate new log; do
    stderr=$($ERG $cmd 2>&1 || true)
    if echo "$stderr" | grep -q '[<>]'; then
        fail "per-command usage: '$cmd' contains angle brackets (got: $stderr)"
    else
        pass "per-command usage: '$cmd' has no angle brackets"
    fi
done

# subcommand --help/-h must exit 0 and not create a --help/ directory
for sub in init check ready next-id archive migrate; do
    tmp=$(mktemp -d)
    (cd "$tmp" && "$ERG" "$sub" --help >/dev/null 2>&1) && pass "$sub --help: exits 0" || fail "$sub --help: exits 0"
    if [ -d "$tmp/--help" ]; then
        fail "$sub --help: spurious --help/ directory created"
    else
        pass "$sub --help: no spurious directory"
    fi
    rm -rf "$tmp"
done

echo "help: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
