#!/bin/sh
# Integration tests for: pre-commit hook — reject tickets/erg on feature branches
set -eu

PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== pre-commit hook: tickets/erg guard ==="

TDIR=$(mktemp -d)
trap 'rm -rf "$TDIR"' EXIT

REPO="$TDIR/repo"
mkdir -p "$REPO/tickets"
cd "$REPO"
git init -q -b main
git config user.email "test@example.com"
git config user.name "Test"

# Install the hook fragment
mkdir -p .git/hooks
cat > .git/hooks/pre-commit << 'HOOK'
# Reject tickets/erg commit on non-main branches
if git diff --cached --name-only | grep -q '^tickets/erg$'; then
    branch=$(git branch --show-current)
    if [ "$branch" != "main" ]; then
        echo "pre-commit: do not commit tickets/erg in feature branches." >&2
        echo " CI rebuilds the binary after merge. Use 'make build' and test" >&2
        echo " with build/erg. To override: git commit --no-verify" >&2
        exit 1
    fi
fi
HOOK
chmod +x .git/hooks/pre-commit

# Create a fake binary
printf '\x7fELF' > tickets/erg
chmod +x tickets/erg

# Create an initial commit on main so we can branch
echo "init" > README
git add README
git commit -q -m "init"

# --- Test 1: tickets/erg staged on feature branch → rejected ---
git checkout -q -b feature/test
git add tickets/erg
if git commit -q -m "add binary" 2>/dev/null; then
    fail "feature branch: commit with tickets/erg must be rejected"
else
    pass "feature branch: commit with tickets/erg is rejected"
fi

# --- Test 2: error message names the right remedy ---
git add tickets/erg
msg=$(git commit -m "add binary" 2>&1 || true)
if echo "$msg" | grep -q "make build"; then
    pass "feature branch: error message mentions make build"
else
    fail "feature branch: error message mentions make build (got: $msg)"
fi
if echo "$msg" | grep -q "build/erg"; then
    pass "feature branch: error message mentions build/erg"
else
    fail "feature branch: error message mentions build/erg (got: $msg)"
fi
if echo "$msg" | grep -q "no-verify"; then
    pass "feature branch: error message mentions --no-verify override"
else
    fail "feature branch: error message mentions --no-verify override (got: $msg)"
fi

# --- Test 3: tickets/erg staged on main → allowed ---
git checkout -q main
git add tickets/erg
if git commit -q -m "CI rebuild binary" 2>/dev/null; then
    pass "main: commit with tickets/erg is allowed"
else
    fail "main: commit with tickets/erg must be allowed"
fi

# --- Test 4: .erg ticket files on feature branch → allowed ---
git checkout -q -b feature/ticket
mkdir -p tickets
cat > tickets/0001-test-ticket.erg << 'ERG'
%erg 0.1
Title: Test ticket
Created: 2026-05-06
Author: test

--- log ---
2026-05-06T10:00Z test created

--- body ---
Test body.
ERG
git add tickets/0001-test-ticket.erg
if git commit -q -m "add ticket" 2>/dev/null; then
    pass "feature branch: .erg ticket files are allowed"
else
    fail "feature branch: .erg ticket files must be allowed"
fi

echo ""
echo "hook: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
