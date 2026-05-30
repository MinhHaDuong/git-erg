#!/bin/sh
# Integration tests for: erg log
set -eu

ERG="${ERG_BIN:-build/erg}"
FIXTURES=$(mktemp -d)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

trap 'rm -rf "$FIXTURES"' EXIT

echo "=== erg log ==="

# --- Log a line into an open ticket by ID ---
cat > "$FIXTURES/0042-smoke.erg" <<'EOF'
%erg 0.1
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

# --- UTC timestamp format survives TZ env override ---
TZ=America/Los_Angeles $ERG log 0042 "claude note tz-check" "$FIXTURES" > /dev/null
if grep -qE "T[0-9][0-9]:[0-9][0-9]Z claude note tz-check" "$FIXTURES/0042-smoke.erg"; then
    pass "UTC timestamp format survives TZ=America/Los_Angeles override"
else
    fail "UTC timestamp format survives TZ=America/Los_Angeles override (expected HH:MMZ, got local offset)"
fi

# --- Negative control: local-offset string does NOT match the Z pattern ---
if echo "2026-05-30T10:30-0700 author note local" | grep -qE "T[0-9][0-9]:[0-9][0-9]Z "; then
    fail "negative control: local-offset string should NOT match UTC pattern"
else
    pass "negative control: local-offset string correctly rejected by UTC pattern"
fi

# --- Two calls produce two entries (append-only) ---
$ERG log 0042 "claude bump test — second" "$FIXTURES" > /dev/null
# || true: grep -c exits 1 when count is 0; the assertion below catches that case.
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
%erg 0.1
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
%erg 0.1
Title: Alpha
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
cat > "$FIXTURES/0044-beta.erg" <<'EOF'
%erg 0.1
Title: Beta
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
err=$($ERG log 0044 "some line" "$FIXTURES" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$err" | grep -q "ambiguous"; then
    pass "ambiguous ID exits non-zero with 'ambiguous'"
else
    fail "ambiguous ID exits non-zero with 'ambiguous' (rc=$rc, got: $err)"
fi
rm -f "$FIXTURES/0044-alpha.erg" "$FIXTURES/0044-beta.erg"

# --- Missing body separator: exits non-zero with error ---
cat > "$FIXTURES/0045-no-body-sep.erg" <<'EOF'
%erg 0.1
Title: No body separator
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created
EOF
err=$($ERG log 0045 "should fail" "$FIXTURES" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$err" | grep -q "body"; then
    pass "missing body separator: exits non-zero with error"
else
    fail "missing body separator: exits non-zero with error (rc=$rc, got: $err)"
fi

# --- Rule 11: a single-word LINE is rejected (would write an invalid log line) ---
err=$($ERG log 0042 "garbage" "$FIXTURES" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$err" | grep -q "valid log entry"; then
    pass "rule 11: single-word LINE rejected"
else
    fail "rule 11: single-word LINE rejected (rc=$rc, got: $err)"
fi
# And nothing was written — the target file still validates and gained no line.
if $ERG validate "$FIXTURES/0042-smoke.erg" >/dev/null 2>&1 && ! grep -qw "garbage" "$FIXTURES/0042-smoke.erg"; then
    pass "rule 11: target unchanged after rejected log (no bad line written)"
else
    fail "rule 11: target unchanged after rejected log"
fi

# unknown flag rejection (ticket 0178)
    out=$($ERG log 0001 "note" --bogus 2>&1) && rc=0 || rc=$?
    if [ "$rc" -ne 0 ] && echo "$out" | grep -q "unknown flag"; then
        pass "unknown flag rejected with usage message"
    else
        fail "unknown flag not rejected (rc=$rc, got: $out)"
    fi


echo "log: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
