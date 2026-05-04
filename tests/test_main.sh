#!/bin/sh
# Integration tests for: erg top-level dispatch (main.go)
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg main dispatch ==="

# --- no args: exit 1, output contains usage info ---
if out=$("$ERG" 2>&1); then code=0; else code=$?; fi
if [ "$code" -ne 1 ]; then
    fail "no args: expected exit 1, got $code"
elif echo "$out" | grep -q "Usage:"; then
    pass "no args: exit 1 with usage"
else
    fail "no args: exit 1 with usage (got: $out)"
fi

# --- unknown command: exit 1, stderr contains "Unknown command" ---
if out=$("$ERG" unknown-cmd 2>&1); then code=0; else code=$?; fi
if [ "$code" -ne 1 ]; then
    fail "unknown command: expected exit 1, got $code"
elif echo "$out" | grep -q "Unknown command"; then
    pass "unknown command: exit 1 with 'Unknown command'"
else
    fail "unknown command: exit 1 with 'Unknown command' (got: $out)"
fi

# --- -h: exit 0, output contains usage ---
if out=$("$ERG" -h 2>&1); then code=0; else code=$?; fi
if [ "$code" -ne 0 ]; then
    fail "-h: expected exit 0, got $code"
elif echo "$out" | grep -q "Usage:"; then
    pass "-h: exit 0 with usage"
else
    fail "-h: exit 0 with usage (got: $out)"
fi

# --- --help: exit 0, output contains usage ---
if out=$("$ERG" --help 2>&1); then code=0; else code=$?; fi
if [ "$code" -ne 0 ]; then
    fail "--help: expected exit 0, got $code"
elif echo "$out" | grep -q "Usage:"; then
    pass "--help: exit 0 with usage"
else
    fail "--help: exit 0 with usage (got: $out)"
fi

# --- help cmd: exit 0, output contains usage ---
if out=$("$ERG" help 2>&1); then code=0; else code=$?; fi
if [ "$code" -ne 0 ]; then
    fail "help cmd: expected exit 0, got $code"
elif echo "$out" | grep -q "Usage:"; then
    pass "help cmd: exit 0 with usage"
else
    fail "help cmd: exit 0 with usage (got: $out)"
fi

echo "main: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
