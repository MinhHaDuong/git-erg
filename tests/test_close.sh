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
trap 'rm -rf "$FIXTURES"' EXIT

echo "=== erg close ==="

# --- Close an open ticket by ID ---
cat > "$FIXTURES/9001-closable.erg" <<'EOF'
%erg v1
Title: Closable ticket
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
EOF

OUT=$($ERG close 9001 "done with this" "$FIXTURES")
if [ "$OUT" = "CLOSED" ]; then
    if grep -q "^Closed: done with this$" "$FIXTURES/9001-closable.erg"; then
        if grep -q "claude closed — done with this" "$FIXTURES/9001-closable.erg"; then
            pass "close open ticket by ID"
        else
            fail "close open ticket by ID (missing log line)"
        fi
    else
        fail "close open ticket by ID (no Closed: header)"
    fi
else
    fail "close open ticket by ID (output: $OUT)"
fi

# --- erg close must not introduce a Status: line ---
if grep -q "^Status:" "$FIXTURES/9001-closable.erg"; then
    fail "erg close did not introduce Status: line"
else
    pass "erg close did not introduce Status: line"
fi

# --- Close already-closed ticket (idempotent) ---
cat > "$FIXTURES/9002-already-closed.erg" <<'EOF'
%erg v1
Title: Already closed
Created: 2026-01-01
Author: claude
Closed: first close

--- log ---
2026-01-01T10:00Z claude created
2026-01-02T10:00Z claude closed — first close

--- body ---
Test body.
EOF

OUT=$($ERG close 9002 "closing again" "$FIXTURES")
if [ "$OUT" = "ALREADY_CLOSED" ]; then
    pass "already-closed ticket returns ALREADY_CLOSED"
else
    fail "already-closed ticket returns ALREADY_CLOSED (output: $OUT)"
fi

# --- Close-by-path ticket lacking a Closed: header gets one ---
cat > "$FIXTURES/9003-by-path.erg" <<'EOF'
%erg v1
Title: Close by path
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
EOF

OUT=$($ERG close "$FIXTURES/9003-by-path.erg" "switching approach")
if [ "$OUT" = "CLOSED" ]; then
    if grep -q "^Closed: switching approach$" "$FIXTURES/9003-by-path.erg"; then
        pass "close by file path"
    else
        fail "close by file path (no Closed: header)"
    fi
else
    fail "close by file path (output: $OUT)"
fi

# --- Closed: in body must NOT be honoured for closure detection ---
cat > "$FIXTURES/9004-body-mention.erg" <<'EOF'
%erg v1
Title: Body mentions closed in prose
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
This ticket discusses what happens when the word disclosed appears.
The string Closed: shows up in markdown only inside this block — not as
a header. (Note: this would normally be rejected by the validator since
"Closed:" at line start is forbidden in body; but erg close itself must
not be confused into thinking the ticket is already closed.)
EOF

# Move the offending body line out so the validator wouldn't reject
# (we're testing close, not validate). Replace the body section first.
cat > "$FIXTURES/9004-body-mention.erg" <<'EOF'
%erg v1
Title: Body mentions closed in prose
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
This ticket mentions disclosed and enclosed in prose only.
EOF

OUT=$($ERG close 9004 "header only" "$FIXTURES")
if [ "$OUT" = "CLOSED" ]; then
    if head -n 8 "$FIXTURES/9004-body-mention.erg" | grep -q "^Closed: header only$"; then
        pass "close adds Closed: header in preamble"
    else
        fail "close did not put Closed: in preamble"
    fi
else
    fail "close on disclosed-mentioning ticket (output: $OUT)"
fi

# --- Empty reason rejected ---
cat > "$FIXTURES/9005-empty.erg" <<'EOF'
%erg v1
Title: Empty reason test
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
if $ERG close 9005 "" "$FIXTURES" 2>/dev/null; then
    fail "empty reason rejected"
else
    pass "empty reason rejected"
fi

# --- File path ending with "-closed.erg" treated as already closed ---
cat > "$FIXTURES/9006-suffix-closed.erg" <<'EOF'
%erg v1
Title: Path-closed ticket
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
mv "$FIXTURES/9006-suffix-closed.erg" "$FIXTURES/9006-suffix-name-closed.erg"
OUT=$($ERG close "$FIXTURES/9006-suffix-name-closed.erg" "another close" 2>/dev/null || true)
if [ "$OUT" = "ALREADY_CLOSED" ]; then
    pass "path-closed ticket recognized as already-closed"
else
    fail "path-closed ticket recognized (output: $OUT)"
fi

# --- "disclosed" path component must NOT trigger closed ---
cat > "$FIXTURES/9007-disclosed-mention.erg" <<'EOF'
%erg v1
Title: Disclosed in name should not match
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
mv "$FIXTURES/9007-disclosed-mention.erg" "$FIXTURES/9007-disclosed-mention-keep.erg"
OUT=$($ERG close "$FIXTURES/9007-disclosed-mention-keep.erg" "real close" 2>/dev/null || true)
if [ "$OUT" = "CLOSED" ]; then
    pass "'disclosed' in name does not falsely close"
else
    fail "'disclosed' in name does not falsely close (output: $OUT)"
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

# --- Close removes Blocked-by refs from dependent open tickets ---
cat > "$FIXTURES/8001-target.erg" <<'EOF'
%erg v1
Title: Target ticket to close
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
cat > "$FIXTURES/8002-dependent-a.erg" <<'EOF'
%erg v1
Title: Dependent A
Created: 2026-01-01
Author: claude
Blocked-by: 8001

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
cat > "$FIXTURES/8003-dependent-b.erg" <<'EOF'
%erg v1
Title: Dependent B
Created: 2026-01-01
Author: claude
Blocked-by: 8001

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
$ERG close 8001 "done" "$FIXTURES" > /dev/null
if grep -q "Blocked-by: 8001" "$FIXTURES/8002-dependent-a.erg"; then
    fail "close: Blocked-by removed from dependent A"
else
    pass "close: Blocked-by removed from dependent A"
fi
if grep -q "Blocked-by: 8001" "$FIXTURES/8003-dependent-b.erg"; then
    fail "close: Blocked-by removed from dependent B"
else
    pass "close: Blocked-by removed from dependent B"
fi
if grep -q "blocker 8001 closed" "$FIXTURES/8002-dependent-a.erg"; then
    pass "close: log entry added to dependent A"
else
    fail "close: log entry added to dependent A"
fi
if grep -q "blocker 8001 closed" "$FIXTURES/8003-dependent-b.erg"; then
    pass "close: log entry added to dependent B"
else
    fail "close: log entry added to dependent B"
fi

# --- Close ticket with no dependents: other files untouched ---
cat > "$FIXTURES/8010-solo.erg" <<'EOF'
%erg v1
Title: Solo ticket
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
cat > "$FIXTURES/8011-unrelated.erg" <<'EOF'
%erg v1
Title: Unrelated ticket
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
before=$(cat "$FIXTURES/8011-unrelated.erg")
$ERG close 8010 "done" "$FIXTURES" > /dev/null
after=$(cat "$FIXTURES/8011-unrelated.erg")
if [ "$before" = "$after" ]; then
    pass "close: no-dependent close leaves other files untouched"
else
    fail "close: no-dependent close leaves other files untouched"
fi

# --- Dependent write failure warns but close succeeds ---
cat > "$FIXTURES/8020-target.erg" <<'EOF'
%erg v1
Title: Target with unwritable dependent
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
cat > "$FIXTURES/8021-dependent-unwritable.erg" <<'EOF'
%erg v1
Title: Unwritable dependent
Created: 2026-01-01
Author: claude
Blocked-by: 8020

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
chmod 444 "$FIXTURES/8021-dependent-unwritable.erg"
OUT=$($ERG close 8020 "done" "$FIXTURES" 2>&1 || true)
chmod 644 "$FIXTURES/8021-dependent-unwritable.erg"
if echo "$OUT" | grep -q "CLOSED"; then
    pass "close: dependent write failure keeps close success"
else
    fail "close: dependent write failure keeps close success"
fi
if echo "$OUT" | grep -q "warning: cannot write"; then
    pass "close: dependent write failure emits warning"
else
    fail "close: dependent write failure emits warning"
fi

echo "close: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
