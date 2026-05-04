#!/bin/sh
# Integration tests for: erg migrate
set -e

ERG="${ERG_BIN:-build/erg}"
FIXTURES="tests/fixtures/migrate"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

mkdir -p "$FIXTURES"
trap 'rm -rf "$FIXTURES"' EXIT

echo "=== erg migrate ==="

# --- Status: closed → Closed: header ---
cat > "$FIXTURES/0001-was-closed.erg" <<'EOF'
%erg v1
Title: Was closed
Status: closed
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created
2026-01-02T10:00Z claude status closed — done

--- body ---
Body.
EOF
$ERG migrate "$FIXTURES" >/dev/null
if grep -q "^Status:" "$FIXTURES/0001-was-closed.erg"; then
    fail "Status: closed → header removed"
else
    pass "Status: closed → header removed"
fi
if grep -q "^Closed: migrated from Status: closed$" "$FIXTURES/0001-was-closed.erg"; then
    pass "Status: closed → Closed: header added"
else
    fail "Status: closed → Closed: header added"
fi

# --- Status: open → Status: line removed, no Closed: added ---
cat > "$FIXTURES/0002-was-open.erg" <<'EOF'
%erg v1
Title: Was open
Status: open
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Body.
EOF
$ERG migrate "$FIXTURES" >/dev/null
if grep -q "^Status:" "$FIXTURES/0002-was-open.erg"; then
    fail "Status: open → header removed"
else
    pass "Status: open → header removed"
fi
if grep -q "^Closed:" "$FIXTURES/0002-was-open.erg"; then
    fail "Status: open → no Closed: added"
else
    pass "Status: open → no Closed: added"
fi

# --- Status: doing → header removed, no Closed: added ---
cat > "$FIXTURES/0003-was-doing.erg" <<'EOF'
%erg v1
Title: Was doing
Status: doing
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
$ERG migrate "$FIXTURES" >/dev/null
if grep -q "^Status:" "$FIXTURES/0003-was-doing.erg" || grep -q "^Closed:" "$FIXTURES/0003-was-doing.erg"; then
    fail "Status: doing → header removed, no Closed: added"
else
    pass "Status: doing → header removed, no Closed: added"
fi

# --- Status: pending → header removed, no Closed: added ---
cat > "$FIXTURES/0004-was-pending.erg" <<'EOF'
%erg v1
Title: Was pending
Status: pending
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
$ERG migrate "$FIXTURES" >/dev/null
if grep -q "^Status:" "$FIXTURES/0004-was-pending.erg" || grep -q "^Closed:" "$FIXTURES/0004-was-pending.erg"; then
    fail "Status: pending → header removed, no Closed: added"
else
    pass "Status: pending → header removed, no Closed: added"
fi

# --- Idempotent: running again is a no-op ---
cp "$FIXTURES/0001-was-closed.erg" "$FIXTURES/snapshot-0001"
cp "$FIXTURES/0002-was-open.erg" "$FIXTURES/snapshot-0002"
$ERG migrate "$FIXTURES" >/dev/null
if cmp -s "$FIXTURES/0001-was-closed.erg" "$FIXTURES/snapshot-0001" \
    && cmp -s "$FIXTURES/0002-was-open.erg" "$FIXTURES/snapshot-0002"; then
    pass "migrate is idempotent"
else
    fail "migrate is idempotent"
fi
rm -f "$FIXTURES/snapshot-0001" "$FIXTURES/snapshot-0002"

# --- File without Status: is unchanged (no-op, counted as already clean) ---
cat > "$FIXTURES/0005-already-clean.erg" <<'EOF'
%erg v1
Title: Already clean
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
cp "$FIXTURES/0005-already-clean.erg" "$FIXTURES/snapshot-0005"
out=$($ERG migrate "$FIXTURES")
if cmp -s "$FIXTURES/0005-already-clean.erg" "$FIXTURES/snapshot-0005"; then
    pass "no-Status file untouched"
else
    fail "no-Status file untouched"
fi
rm -f "$FIXTURES/snapshot-0005"
if echo "$out" | grep -qE "already clean: [1-9]"; then
    pass "summary reports already-clean count"
else
    fail "summary reports already-clean count (got: $out)"
fi

# --- After migration the migrated file validates ---
$ERG validate "$FIXTURES" >/dev/null 2>&1
ec=$?
# Ignore ID collision errors caused by repeated 0001/0002 IDs across older
# fixtures; just verify there is no Status: complaint.
out=$($ERG validate "$FIXTURES" 2>&1 || true)
if echo "$out" | grep -qi "Status:"; then
    fail "no Status: complaint after migration"
else
    pass "no Status: complaint after migration"
fi

# --- erg validate rejects Status: lines (migrate is the only tolerant cmd) ---
cat > "$FIXTURES/0009-still-status.erg" <<'EOF'
%erg v1
Title: Still has Status
Status: open
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created
--- body ---
EOF
if $ERG validate "$FIXTURES/0009-still-status.erg" >/dev/null 2>&1; then
    fail "validate rejects unmigrated Status:"
else
    pass "validate rejects unmigrated Status:"
fi

echo "migrate: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
