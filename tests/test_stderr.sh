#!/bin/sh
# Integration tests: all error output goes to stderr, stdout stays clean
set -eu

ERG="${ERG_BIN:-build/erg}"
FIXTURES=$(mktemp -d)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

trap 'rm -rf "$FIXTURES"' EXIT

echo "=== erg stderr routing ==="

# --- erg close: nonexistent ticket writes nothing to stdout ---
out=$($ERG close NONEXISTENT reason "$FIXTURES" 2>/dev/null || true)
if [ -z "$out" ]; then
    pass "close nonexistent: stdout empty"
else
    fail "close nonexistent: stdout not empty (got: $out)"
fi

# --- erg log: nonexistent ticket writes nothing to stdout ---
out=$($ERG log NONEXISTENT "a line" "$FIXTURES" 2>/dev/null || true)
if [ -z "$out" ]; then
    pass "log nonexistent: stdout empty"
else
    fail "log nonexistent: stdout not empty (got: $out)"
fi

# --- erg rm: nonexistent ticket writes nothing to stdout ---
out=$($ERG rm NONEXISTENT "$FIXTURES" 2>/dev/null || true)
if [ -z "$out" ]; then
    pass "rm nonexistent: stdout empty"
else
    fail "rm nonexistent: stdout not empty (got: $out)"
fi

# --- erg new: empty title writes nothing to stdout ---
out=$($ERG new "" "$FIXTURES" 2>/dev/null || true)
if [ -z "$out" ]; then
    pass "new empty title: stdout empty"
else
    fail "new empty title: stdout not empty (got: $out)"
fi

# --- erg check: duplicate IDs writes nothing to stdout ---
mkdir -p "$FIXTURES/dup"
cat > "$FIXTURES/dup/0001-alpha.erg" <<'EOF'
%erg 0.1
Title: Alpha
Created: 2026-01-01
Author: a

--- log ---
2026-01-01T10:00Z a created

--- body ---
EOF
cat > "$FIXTURES/dup/0001-beta.erg" <<'EOF'
%erg 0.1
Title: Beta
Created: 2026-01-01
Author: a

--- log ---
2026-01-01T10:00Z a created

--- body ---
EOF
out=$($ERG check "$FIXTURES/dup" 2>/dev/null || true)
if [ -z "$out" ]; then
    pass "check duplicate IDs: stdout empty"
else
    fail "check duplicate IDs: stdout not empty (got: $out)"
fi

# --- erg validate: malformed file writes nothing to stdout ---
mkdir -p "$FIXTURES/bad"
# File missing the %erg magic line — syntactically invalid
printf 'Title: No magic line\nAuthor: a\n' > "$FIXTURES/bad/0001-bad.erg"
out=$($ERG validate "$FIXTURES/bad/0001-bad.erg" 2>/dev/null || true)
if [ -z "$out" ]; then
    pass "validate malformed file: stdout empty"
else
    fail "validate malformed file: stdout not empty (got: $out)"
fi

echo "stderr: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
