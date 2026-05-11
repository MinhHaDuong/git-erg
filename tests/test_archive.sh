#!/bin/sh
# Integration tests for: erg archive
set -eu

ERG="${ERG_BIN:-build/erg}"
FIXTURES=$(mktemp -d)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

trap 'rm -rf "$FIXTURES"' EXIT

echo "=== erg archive ==="

# Helper: write a minimal closed ticket.
write_closed() {
    cat > "$1" <<EOF
%erg v1
Title: $2
Created: 2026-01-01
Author: claude
Closed: done

--- log ---
2026-01-01T10:00Z claude created
2026-01-01T11:00Z claude closed — done

--- body ---
EOF
}

# Helper: write a minimal open ticket.
write_open() {
    cat > "$1" <<EOF
%erg v1
Title: $2
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
}

# Helper: write an open ticket blocked by another.
write_open_blocked_by() {
    cat > "$1" <<EOF
%erg v1
Title: $2
Created: 2026-01-01
Author: claude
Blocked-by: $3

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
}

# --- Default mode: unblocked closed ticket is archived ---
write_closed  "$FIXTURES/7001-closed-unblocked.erg" "Closed Unblocked"
write_open    "$FIXTURES/7002-open-ticket.erg"      "Open Ticket"
write_closed  "$FIXTURES/7003-closed-blocked.erg"   "Closed But Blocking"
write_open_blocked_by "$FIXTURES/7004-open-blocked-by-7003.erg" "Open blocked by 7003" "7003"

OUT=$($ERG archive "$FIXTURES" 2>&1)

if [ -f "$FIXTURES/closed/7001-closed-unblocked.erg" ]; then
    pass "default: unblocked closed ticket moved to closed/"
else
    fail "default: unblocked closed ticket moved to closed/ (got: $OUT)"
fi

if [ ! -f "$FIXTURES/7001-closed-unblocked.erg" ]; then
    pass "default: unblocked closed ticket removed from top-level"
else
    fail "default: unblocked closed ticket removed from top-level"
fi

if [ -f "$FIXTURES/7002-open-ticket.erg" ]; then
    pass "default: open ticket untouched"
else
    fail "default: open ticket untouched"
fi

if [ -f "$FIXTURES/7003-closed-blocked.erg" ] && [ ! -f "$FIXTURES/closed/7003-closed-blocked.erg" ]; then
    pass "default: closed-but-blocking ticket stays in top-level"
else
    fail "default: closed-but-blocking ticket stays in top-level"
fi

if echo "$OUT" | grep -q "SKIPPED 7003-closed-blocked.erg"; then
    pass "default: SKIPPED message emitted for blocking ticket"
else
    fail "default: SKIPPED message emitted for blocking ticket (got: $OUT)"
fi

if echo "$OUT" | grep -q "needed by 7004"; then
    pass "default: SKIPPED message says 'needed by'"
else
    fail "default: SKIPPED message says 'needed by' (got: $OUT)"
fi

if echo "$OUT" | grep -q "ARCHIVED 7001-closed-unblocked.erg"; then
    pass "default: ARCHIVED message emitted for moved ticket"
else
    fail "default: ARCHIVED message emitted for moved ticket (got: $OUT)"
fi

# --- Default mode: open ticket not mentioned in output (silent skip) ---
if echo "$OUT" | grep -q "7002-open-ticket.erg"; then
    fail "default: open ticket silently ignored (appeared in output)"
else
    pass "default: open ticket silently ignored"
fi

# --- ID mode: closed ticket still blocking open ticket is skipped ---
[ -f "$FIXTURES/7003-closed-blocked.erg" ] || fail "fixture 7003-closed-blocked.erg unexpectedly missing"
OUT_7003=$($ERG archive 7003 "$FIXTURES" 2>&1)
if echo "$OUT_7003" | grep -q "SKIPPED 7003-closed-blocked.erg"; then
    pass "id mode: closed-blocking ticket emits SKIPPED"
else
    fail "id mode: closed-blocking ticket emits SKIPPED (got: $OUT_7003)"
fi

if echo "$OUT_7003" | grep -q "needed by 7004"; then
    pass "id mode: SKIPPED message says 'needed by'"
else
    fail "id mode: SKIPPED message says 'needed by' (got: $OUT_7003)"
fi

# --- ID mode: source file unchanged after blocker skip ---
[ -f "$FIXTURES/7003-closed-blocked.erg" ] || fail "fixture 7003-closed-blocked.erg unexpectedly missing"
if [ -f "$FIXTURES/7003-closed-blocked.erg" ] && [ ! -f "$FIXTURES/closed/7003-closed-blocked.erg" ]; then
    pass "id mode: source file unchanged after blocker skip"
else
    fail "id mode: source file unchanged after blocker skip"
fi

# --- ID mode: archive a specific ticket by ID ---
write_closed "$FIXTURES/7010-specific.erg" "Specific Ticket"

OUT2=$($ERG archive 7010 "$FIXTURES" 2>&1)
if [ -f "$FIXTURES/closed/7010-specific.erg" ]; then
    pass "id mode: specific closed ticket moved to closed/"
else
    fail "id mode: specific closed ticket moved to closed/ (got: $OUT2)"
fi
if echo "$OUT2" | grep -q "ARCHIVED 7010-specific.erg"; then
    pass "id mode: ARCHIVED message emitted"
else
    fail "id mode: ARCHIVED message emitted (got: $OUT2)"
fi

# --- ID mode: open ticket given explicitly is silently skipped ---
write_open "$FIXTURES/7011-open-explicit.erg" "Open Explicit"
OUT3=$($ERG archive 7011 "$FIXTURES" 2>&1)
if [ -f "$FIXTURES/7011-open-explicit.erg" ] && [ ! -f "$FIXTURES/closed/7011-open-explicit.erg" ]; then
    pass "id mode: open ticket silently ignored"
else
    fail "id mode: open ticket silently ignored (got: $OUT3)"
fi

# Note: double-archive is a no-op by design. The scanner only examines
# top-level *.erg files, so a ticket already in closed/ is never re-examined.
# There is no meaningful test to write here: the absence of a collision error
# would be vacuous (the file is simply never visited by the scanner).

# --- Destination collision: skip with error message (idempotent) ---
write_closed "$FIXTURES/7020-collision.erg" "Collision Ticket"
mkdir -p "$FIXTURES/closed"
write_closed "$FIXTURES/closed/7020-collision.erg" "Already There"
OUT5=$($ERG archive 7020 "$FIXTURES" 2>&1)
if echo "$OUT5" | grep -q "already exists"; then
    pass "collision: destination-exists emits error message"
else
    fail "collision: destination-exists emits error message (got: $OUT5)"
fi
if [ -f "$FIXTURES/7020-collision.erg" ]; then
    pass "collision: source not removed on collision"
else
    fail "collision: source not removed on collision"
fi

# --- Non-existent ID: warning printed, exit 0 ---
OUT6=$($ERG archive 9999 "$FIXTURES" 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 0 ] && echo "$OUT6" | grep -q "no ticket found"; then
    pass "id mode: non-existent ID prints warning"
else
    fail "id mode: non-existent ID prints warning (rc=$rc, got: $OUT6)"
fi

# --- filepath.Clean regression: trailing slash on directory arg ---
# PR #126 added filepath.Clean to ticketDir so that `tickets/` and
# `tickets` compare equal. Without it, default-mode filtering in
# archive.go:116 (filepath.Dir(t.Path) == ticketDir) silently drops
# every ticket because `tickets/` != `tickets`.
SLASH_DIR=$(mktemp -d)
write_closed "$SLASH_DIR/8001-trailing-slash.erg" "Trailing Slash Test"
# Pass dir WITH trailing slash — must behave identically to without.
OUT_SLASH=$($ERG archive "$SLASH_DIR/" 2>&1)
if [ -f "$SLASH_DIR/closed/8001-trailing-slash.erg" ]; then
    pass "trailing slash: ticket archived with dir/ arg"
else
    fail "trailing slash: ticket archived with dir/ arg (got: $OUT_SLASH)"
fi
OUT_NOSLASH_DIR=$(mktemp -d)
write_closed "$OUT_NOSLASH_DIR/8002-no-slash.erg" "No Slash Test"
OUT_NOSLASH=$($ERG archive "$OUT_NOSLASH_DIR" 2>&1)
if [ -f "$OUT_NOSLASH_DIR/closed/8002-no-slash.erg" ]; then
    pass "trailing slash: ticket archived with dir arg (no slash)"
else
    fail "trailing slash: ticket archived with dir arg (no slash) (got: $OUT_NOSLASH)"
fi
rm -rf "$SLASH_DIR" "$OUT_NOSLASH_DIR"

echo "archive: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
