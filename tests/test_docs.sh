#!/bin/sh
# Integration tests for: make docs target
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg docs ==="

# --help --all: each command has a section header
count=$("$ERG" --help --all 2>/dev/null | grep -c "^## erg " || true)
if [ "$count" -eq 16 ]; then
    pass "--help --all: 16 section headers found"
else
    fail "--help --all: expected 16 section headers, got $count"
fi

# --help=all: alternate form also prints 16 section headers
count=$("$ERG" --help=all 2>/dev/null | grep -c "^## erg " || true)
if [ "$count" -eq 16 ]; then
    pass "--help=all: 16 section headers found"
else
    fail "--help=all: expected 16 section headers, got $count"
fi

# --help --all: output goes to stdout (stderr should be empty)
stderr_out=$("$ERG" --help --all 2>&1 >/dev/null || true)
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

# README must not show `erg validate <ID>` — validate takes file paths, not IDs
# (regression guard for 0161; the broken example was `tickets/erg validate 01`).
if grep -Eq 'erg validate [0-9]+( |$)' README.md; then
    fail "README has an erg validate example with a bare ID (validate takes paths)"
else
    pass "README erg validate examples use file paths, not bare IDs"
fi

echo ""
if [ "$FAIL" -eq 0 ]; then
    echo "docs: PASS ($PASS checks)"
    exit 0
else
    echo "docs: FAIL ($FAIL/$((PASS + FAIL)) checks failed)"
    exit 1
fi
