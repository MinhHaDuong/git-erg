#!/bin/sh
# Integration tests for: the pre-commit hook installed by erg install --hooks
set -eu

ERG="${ERG_BIN:-build/erg}"
ERG_ABS=$(CDPATH= cd "$(dirname "$ERG")" && pwd)/$(basename "$ERG")
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

# Claims are required (ticket 0307): fixture stores are made compliant with
# plain files (claim_store), so each case fails for its own defect.
. "$(dirname "$0")/claims_fixture.sh"

echo "=== pre-commit hook ==="

TDIR=$(mktemp -d)
trap 'rm -rf "$TDIR"' EXIT

REPO="$TDIR/repo"
mkdir -p "$REPO/tickets"
claim_store "$REPO/tickets"
cd "$REPO"
git init -q -b main
git config user.email "test@example.com"
git config user.name "Test"
# Keep the sandbox repo hermetic: this suite exercises the pre-commit hook,
# not commit signing. Disable signing locally so the test does not depend on
# the host's global git config (which may point at an unreachable signer).
git config commit.gpgsign false

# Install the real shipped hook (not a hand-rolled copy that can silently
# diverge from hookBody). The cp places the binary that the .erg validate path
# in Test 4 will exec; "$ERG_ABS" is required because we have cd'd into "$REPO".
# install creates the hooks dir itself (install.go resolveHooksDir → MkdirAll).
cp "$ERG_ABS" tickets/erg
"$ERG_ABS" install . --hooks >/dev/null

# Create an initial commit on main so we can branch
echo "init" > README
git add README
git commit -q -m "init"

# --- Test 1: tickets/erg staged on a feature branch -> allowed (ticket 0311) ---
# An adopter lands a binary refresh by pull request, so the hook must not
# refuse the vendored binary on a branch.
git checkout -q -b feature/test
git add tickets/erg
if msg=$(git commit -q -m "refresh binary" 2>&1); then
    pass "feature branch: commit with tickets/erg is allowed"
else
    fail "feature branch: commit with tickets/erg must be allowed (got: $msg)"
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
claim_store tickets
git add tickets/0001-test-ticket.erg tickets/.claims
if git commit -q -m "add ticket" 2>/dev/null; then
    pass "feature branch: .erg ticket files are allowed"
else
    fail "feature branch: .erg ticket files must be allowed"
fi

# --- Test 5: closed-but-unarchived ticket → pre-commit blocks (0241) ---
# The escape this ticket closes: someone runs erg close but not erg archive,
# then commits. erg check (in the pre-commit hook) now rejects this.
cat > tickets/0002-closed-unarchived.erg << 'ERG'
%erg 0.1
Title: Closed but not archived
Created: 2026-06-08
Author: test
Closed: done

--- log ---
2026-06-08T10:00Z test created
2026-06-08T10:01Z test closed — done

--- body ---
ERG
claim_store tickets
git add tickets/0002-closed-unarchived.erg tickets/.claims/0002
msg=$(git commit -q -m "close without archive" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$msg" | grep -qF "closed ticket not in closed/ directory"; then
    pass "closed-unarchived: pre-commit hook blocks closed-but-unarchived ticket"
else
    fail "closed-unarchived: commit must be rejected by pre-commit hook on folder closure (0241) (rc=$rc: $msg)"
fi

# --- Test 6: closed ticket properly archived → pre-commit allows ---
mkdir -p tickets/closed
mv tickets/0002-closed-unarchived.erg tickets/closed/
git add tickets/
if git commit -q -m "close and archive" 2>/dev/null; then
    pass "closed-archived: commit allowed when ticket is in closed/"
else
    fail "closed-archived: commit must be allowed when ticket is properly archived"
fi

echo ""
echo "hook: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
