#!/bin/sh
# Integration tests for: erg log
set -eu

ERG="${ERG_BIN:-build/erg}"
FIXTURES="tests/fixtures/log"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

mkdir -p "$FIXTURES"
trap 'rm -rf "$FIXTURES"' EXIT

echo "=== erg log ==="

# --- Log a line into an open ticket by ID ---
cat > "$FIXTURES/0042-smoke.erg" <<'EOF'
%erg v1
Title: Smoke test ticket
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
EOF

OUT=$($ERG log 0042 "claude bump test — smoke" "$FIXTURES")
if [ "$OUT" = "LOGGED" ]; then
    if grep -q "claude bump test — smoke" "$FIXTURES/0042-smoke.erg"; then
        pass "log appends line and prints LOGGED"
    else
        fail "log appends line and prints LOGGED (line not found in file)"
    fi
else
    fail "log appends line and prints LOGGED (output: $OUT)"
fi

# --- Logged line appears in log section (before --- body ---) ---
log_line_no=$(grep -n "claude bump test" "$FIXTURES/0042-smoke.erg" | cut -d: -f1)
body_line_no=$(grep -n "^--- body ---$" "$FIXTURES/0042-smoke.erg" | cut -d: -f1)
if [ -n "$log_line_no" ] && [ -n "$body_line_no" ] && [ "$log_line_no" -lt "$body_line_no" ]; then
    pass "log line appears before --- body ---"
else
    fail "log line appears before --- body ---"
fi

# --- Timestamp is prepended automatically ---
if grep -qE "^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}Z claude bump test" "$FIXTURES/0042-smoke.erg"; then
    pass "log line has ISO8601 UTC timestamp prefix"
else
    fail "log line has ISO8601 UTC timestamp prefix"
fi

# --- Two calls produce two entries (append-only) ---
$ERG log 0042 "claude bump test — second" "$FIXTURES" > /dev/null
count=$(grep -c "claude bump test" "$FIXTURES/0042-smoke.erg" || true)
if [ "$count" -eq 2 ]; then
    pass "log is append-only (two calls produce two entries)"
else
    fail "log is append-only (expected 2 entries, got $count)"
fi

# --- Missing args exits non-zero ---
if $ERG log 2>/dev/null; then
    fail "missing args exits non-zero"
else
    pass "missing args exits non-zero"
fi

# --- Missing line arg exits non-zero ---
if $ERG log 0042 2>/dev/null; then
    fail "missing line arg exits non-zero"
else
    pass "missing line arg exits non-zero"
fi

# --- Empty line rejected ---
cat > "$FIXTURES/0043-empty-line.erg" <<'EOF'
%erg v1
Title: Empty line test
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
if $ERG log 0043 "" "$FIXTURES" 2>/dev/null; then
    fail "empty line rejected"
else
    pass "empty line rejected"
fi

# --- Non-existent ID exits non-zero ---
if $ERG log 8888 "some line" "$FIXTURES" 2>/dev/null; then
    fail "non-existent ID exits non-zero"
else
    pass "non-existent ID exits non-zero"
fi

# --- Ambiguous ID exits non-zero with 'ambiguous' ---
cat > "$FIXTURES/0044-alpha.erg" <<'EOF'
%erg v1
Title: Alpha
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
cat > "$FIXTURES/0044-beta.erg" <<'EOF'
%erg v1
Title: Beta
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
err=$($ERG log 0044 "some line" "$FIXTURES" 2>&1 || true)
if echo "$err" | grep -q "ambiguous"; then
    pass "ambiguous ID exits non-zero with 'ambiguous'"
else
    fail "ambiguous ID exits non-zero with 'ambiguous' (got: $err)"
fi
rm -f "$FIXTURES/0044-alpha.erg" "$FIXTURES/0044-beta.erg"

echo "log: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
