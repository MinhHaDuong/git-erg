#!/bin/sh
# Integration tests for: make docs target
set -eu

PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg docs ==="

# --help --all: each command has a section header
count=$("${ERG_BIN:-build/erg}" --help --all 2>/dev/null | grep -c "^# " || true)
if [ "$count" -eq 12 ]; then
    pass "--help --all: 12 section headers found"
else
    fail "--help --all: expected 12 section headers, got $count"
fi

# --help --all: output goes to stdout (stderr should be empty)
stderr_out=$("${ERG_BIN:-build/erg}" --help --all 2>&1 >/dev/null || true)
if [ -z "$stderr_out" ]; then
    pass "--help --all: no output on stderr"
else
    fail "--help --all: unexpected stderr: $stderr_out"
fi

# make docs produces a non-empty docs/erg-manual.md
make docs >/dev/null 2>&1
if [ -s docs/erg-manual.md ]; then
    pass "make docs: docs/erg-manual.md is non-empty"
else
    fail "make docs: docs/erg-manual.md is missing or empty"
fi

echo ""
if [ "$FAIL" -eq 0 ]; then
    echo "docs: PASS ($PASS checks)"
    exit 0
else
    echo "docs: FAIL ($FAIL/$((PASS + FAIL)) checks failed)"
    exit 1
fi
