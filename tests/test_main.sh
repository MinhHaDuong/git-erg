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
out=$("$ERG" 2>&1 || true)
if "$ERG" >/dev/null 2>&1; then
    fail "no args exits 1"
else
    if echo "$out" | grep -q "Usage:"; then
        pass "no args: exit 1 with usage"
    else
        fail "no args: exit 1 with usage (got: $out)"
    fi
fi

# --- unknown command: exit 1, stderr contains "Unknown command" ---
out=$("$ERG" unknown-cmd 2>&1 || true)
if "$ERG" unknown-cmd >/dev/null 2>&1; then
    fail "unknown command exits 1"
else
    if echo "$out" | grep -q "Unknown command"; then
        pass "unknown command: exit 1 with 'Unknown command'"
    else
        fail "unknown command: exit 1 with 'Unknown command' (got: $out)"
    fi
fi

# --- -h: exit 0, output contains usage ---
out=$("$ERG" -h 2>&1 || true)
if "$ERG" -h >/dev/null 2>&1; then
    if echo "$out" | grep -q "Usage:"; then
        pass "-h: exit 0 with usage"
    else
        fail "-h: exit 0 with usage (got: $out)"
    fi
else
    fail "-h: expected exit 0"
fi

# --- --help: exit 0, output contains usage ---
out=$("$ERG" --help 2>&1 || true)
if "$ERG" --help >/dev/null 2>&1; then
    if echo "$out" | grep -q "Usage:"; then
        pass "--help: exit 0 with usage"
    else
        fail "--help: exit 0 with usage (got: $out)"
    fi
else
    fail "--help: expected exit 0"
fi

# --- help cmd: exit 0, output contains usage ---
out=$("$ERG" help 2>&1 || true)
if "$ERG" help >/dev/null 2>&1; then
    if echo "$out" | grep -q "Usage:"; then
        pass "help cmd: exit 0 with usage"
    else
        fail "help cmd: exit 0 with usage (got: $out)"
    fi
else
    fail "help cmd: expected exit 0"
fi

echo "main: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
