#!/bin/sh
# Integration tests for: erg close
set -e

ERG="${ERG_BIN:-tickets/tools/go/erg}"
FIXTURES="tests/fixtures/close"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

mkdir -p "$FIXTURES"
# Single EXIT trap — bash semantics overwrite any later trap, so all fixture
# cleanup must live here.
trap 'rm -rf "$FIXTURES"' EXIT

echo "=== erg close ==="

# --- Close an open ticket by ID ---
cat > "$FIXTURES/9001-closable.erg" <<'EOF'
%erg v1
Title: Closable ticket
Status: open
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
EOF

OUT=$($ERG close 9001 "done with this" "$FIXTURES")
if [ "$OUT" = "CLOSED" ]; then
    # Verify status changed in file
    if grep -q "^Status: closed$" "$FIXTURES/9001-closable.erg"; then
        # Verify log line was appended
        if grep -q "claude status closed — done with this" "$FIXTURES/9001-closable.erg"; then
            pass "close open ticket by ID"
        else
            fail "close open ticket by ID (missing log line)"
        fi
    else
        fail "close open ticket by ID (status not changed)"
    fi
else
    fail "close open ticket by ID (output: $OUT)"
fi

# --- Close already-closed ticket (idempotent) ---
cat > "$FIXTURES/9002-already-closed.erg" <<'EOF'
%erg v1
Title: Already closed
Status: closed
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created
2026-01-02T10:00Z claude status closed — first close

--- body ---
Test body.
EOF

OUT=$($ERG close 9002 "closing again" "$FIXTURES")
if [ "$OUT" = "ALREADY_CLOSED" ]; then
    pass "already-closed ticket returns ALREADY_CLOSED"
else
    fail "already-closed ticket returns ALREADY_CLOSED (output: $OUT)"
fi

# --- Close by file path ---
cat > "$FIXTURES/9003-by-path.erg" <<'EOF'
%erg v1
Title: Close by path
Status: doing
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
EOF

OUT=$($ERG close "$FIXTURES/9003-by-path.erg" "switching approach")
if [ "$OUT" = "CLOSED" ]; then
    if grep -q "^Status: closed$" "$FIXTURES/9003-by-path.erg"; then
        pass "close by file path"
    else
        fail "close by file path (status not changed)"
    fi
else
    fail "close by file path (output: $OUT)"
fi

# --- Status: line in body must NOT be rewritten ---
cat > "$FIXTURES/9004-body-status.erg" <<'EOF'
%erg v1
Title: Body contains a Status line
Status: open
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Example log shape:
Status: open
(this line lives inside the body and must be preserved verbatim)
EOF

OUT=$($ERG close 9004 "header only" "$FIXTURES")
if [ "$OUT" = "CLOSED" ]; then
    # Body line must still say "Status: open"
    if grep -q "^Status: open$" "$FIXTURES/9004-body-status.erg"; then
        # And the header must now say "Status: closed"
        if head -n 6 "$FIXTURES/9004-body-status.erg" | grep -q "^Status: closed$"; then
            pass "body Status: line preserved, header updated"
        else
            fail "header Status not updated to closed"
        fi
    else
        fail "body Status: line was incorrectly rewritten"
    fi
else
    fail "close with body Status: line (output: $OUT)"
fi

# --- Missing args (no ID) ---
if $ERG close 2>/dev/null; then
    fail "missing args exits non-zero"
else
    pass "missing args exits non-zero"
fi

# --- Missing reason ---
if $ERG close 9001 2>/dev/null; then
    fail "missing reason exits non-zero"
else
    pass "missing reason exits non-zero"
fi

# --- Non-existent ID ---
if $ERG close 8888 "reason" "$FIXTURES" 2>/dev/null; then
    fail "non-existent ID exits non-zero"
else
    pass "non-existent ID exits non-zero"
fi

echo "close: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
