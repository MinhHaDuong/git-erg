#!/bin/sh
# Integration tests for: erg rm
set -eu

ERG="${ERG_BIN:-build/erg}"
FIXTURES=$(mktemp -d)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

trap 'rm -rf "$FIXTURES"' EXIT

echo "=== erg rm ==="

# Helper: write a minimal open ticket.
write_open() {
    cat > "$1" <<EOF
%erg 0.1
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
%erg 0.1
Title: $2
Created: 2026-01-01
Author: claude
Blocked-by: $3

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
}

# Helper: write a closed ticket blocked by another (historical ref).
write_closed_blocked_by() {
    cat > "$1" <<EOF
%erg 0.1
Title: $2
Created: 2026-01-01
Author: claude
Closed: done
Blocked-by: $3

--- log ---
2026-01-01T10:00Z claude created
2026-01-01T11:00Z claude closed — done

--- body ---
EOF
}

# --- Refuse-on-dependent: rm without --force is blocked by an open dependent ---
write_open            "$FIXTURES/0001-blocker.erg"          "Blocker ticket"
write_open_blocked_by "$FIXTURES/0002-blocked.erg" "Blocked ticket" "0001"

out=$($ERG rm 0001 "$FIXTURES" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ]; then
    pass "refuse: rm with dependent exits non-zero"
else
    fail "refuse: rm with dependent exits non-zero (rc=$rc, got: $out)"
fi
if echo "$out" | grep -q "0002"; then
    pass "refuse: dependent named in output"
else
    fail "refuse: dependent named in output (got: $out)"
fi
if [ -f "$FIXTURES/0001-blocker.erg" ]; then
    pass "refuse: target file untouched on refusal"
else
    fail "refuse: target file untouched on refusal"
fi
if [ -f "$FIXTURES/0002-blocked.erg" ] && grep -q "Blocked-by: 0001" "$FIXTURES/0002-blocked.erg"; then
    pass "refuse: dependent file untouched on refusal"
else
    fail "refuse: dependent file untouched on refusal"
fi

# --- --force: deletes target and clears the dangling Blocked-by line ---
out=$($ERG rm --force 0001 "$FIXTURES" 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 0 ]; then
    pass "force: rm --force exits 0"
else
    fail "force: rm --force exits 0 (rc=$rc, got: $out)"
fi
if [ ! -f "$FIXTURES/0001-blocker.erg" ]; then
    pass "force: target deleted"
else
    fail "force: target deleted"
fi
if [ -f "$FIXTURES/0002-blocked.erg" ] && ! grep -q "Blocked-by: 0001" "$FIXTURES/0002-blocked.erg"; then
    pass "force: dangling Blocked-by line stripped from dependent"
else
    fail "force: dangling Blocked-by line stripped from dependent"
fi
if grep -q "note blocker 0001 removed — ticket deleted." "$FIXTURES/0002-blocked.erg"; then
    pass "force: dependent gained a log entry"
else
    fail "force: dependent gained a log entry (got: $(cat "$FIXTURES/0002-blocked.erg"))"
fi

# --- No dependents: rm deletes without --force ---
out=$($ERG rm 0002 "$FIXTURES" 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 0 ] && [ ! -f "$FIXTURES/0002-blocked.erg" ]; then
    pass "no-deps: rm deletes without --force"
else
    fail "no-deps: rm deletes without --force (rc=$rc, got: $out)"
fi

# --- Non-existent ID: exit non-zero with resolver message ---
out=$($ERG rm 9999 "$FIXTURES" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "no ticket found"; then
    pass "missing: non-existent ID exits non-zero with resolver message"
else
    fail "missing: non-existent ID exits non-zero with resolver message (rc=$rc, got: $out)"
fi

# --- Closed dependent is also a guard (and --force clears it) ---
CLOSED_DIR=$(mktemp -d)
write_open                  "$CLOSED_DIR/0001-blocker.erg"           "Blocker"
write_closed_blocked_by     "$CLOSED_DIR/0003-closed-dependent.erg" "Closed dependent" "0001"

out=$($ERG rm 0001 "$CLOSED_DIR" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "0003"; then
    pass "closed-dep: guard scans closed tickets too"
else
    fail "closed-dep: guard scans closed tickets too (rc=$rc, got: $out)"
fi

out=$($ERG rm --force 0001 "$CLOSED_DIR" 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 0 ] && ! grep -q "Blocked-by: 0001" "$CLOSED_DIR/0003-closed-dependent.erg"; then
    pass "closed-dep: --force clears ref in closed dependent"
else
    fail "closed-dep: --force clears ref in closed dependent (rc=$rc, got: $out)"
fi

# --- Corpus stays check-clean after a --force cascade ---
if $ERG check "$CLOSED_DIR" >/dev/null 2>&1; then
    pass "corpus: erg check clean after --force (no dangling ref)"
else
    fail "corpus: erg check clean after --force (got: $($ERG check "$CLOSED_DIR" 2>&1))"
fi
rm -rf "$CLOSED_DIR"

# --- FILE form: rm accepts a full filename ---
write_open "$FIXTURES/0010-by-filename.erg" "Delete by filename"
out=$($ERG rm "$FIXTURES/0010-by-filename.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 0 ] && [ ! -f "$FIXTURES/0010-by-filename.erg" ]; then
    pass "file-form: rm accepts a full path"
else
    fail "file-form: rm accepts a full path (rc=$rc, got: $out)"
fi

echo "rm: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
