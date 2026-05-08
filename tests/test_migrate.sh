#!/bin/sh
# Integration tests for: erg migrate
set -eu

ERG="${ERG_BIN:-build/erg}"
FIXTURES=$(mktemp -d)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

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

# --- After migration the migrated files pass check ---
# Use 'check' for directory-level validation; 'validate' takes files only.
out=$($ERG check "$FIXTURES" 2>&1 || true)
# Ignore ID collision errors caused by repeated 0001/0002 IDs across older
# fixtures; just verify there is no Status: complaint.
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

# --- Layout migration: tools/, FORMAT.md removed; archive/ → closed/; init refreshed ---
LDIR=$(mktemp -d)
trap 'rm -rf "$LDIR"' EXIT
mkdir -p "$LDIR/tickets/tools"
touch "$LDIR/tickets/FORMAT.md"
mkdir -p "$LDIR/archive"
# Place erg binary so cmdInit can find it
cp "$ERG" "$LDIR/tickets/erg"

$ERG migrate "$LDIR/tickets" >/dev/null 2>&1

if [ -d "$LDIR/tickets/tools" ]; then
    fail "layout migration: tickets/tools/ removed"
else
    pass "layout migration: tickets/tools/ removed"
fi
if [ -f "$LDIR/tickets/FORMAT.md" ]; then
    fail "layout migration: tickets/FORMAT.md removed"
else
    pass "layout migration: tickets/FORMAT.md removed"
fi
if [ -d "$LDIR/archive" ]; then
    fail "layout migration: archive/ gone after rename"
else
    pass "layout migration: archive/ gone after rename"
fi
if [ -d "$LDIR/closed" ]; then
    pass "layout migration: closed/ exists after rename"
else
    fail "layout migration: closed/ exists after rename"
fi

# --- Layout migration idempotency ---
if $ERG migrate "$LDIR/tickets" >/dev/null 2>&1; then
    pass "layout migration: idempotent (second run succeeds)"
else
    fail "layout migration: idempotent (second run must not error)"
fi

# --- Layout migration: self-copy binary when tickets/erg absent ---
BDIR=$(mktemp -d)
mkdir -p "$BDIR/tickets"
$ERG migrate "$BDIR/tickets" >/dev/null 2>&1
if [ -x "$BDIR/tickets/erg" ]; then
    pass "layout migration: self-copied tickets/erg when absent"
else
    fail "layout migration: self-copied tickets/erg when absent"
fi
mtime1=$(stat -c %Y "$BDIR/tickets/erg")
$ERG migrate "$BDIR/tickets" >/dev/null 2>&1
mtime2=$(stat -c %Y "$BDIR/tickets/erg")
if [ "$mtime1" = "$mtime2" ]; then
    pass "layout migration: self-copy idempotent (no overwrite)"
else
    fail "layout migration: self-copy idempotent (no overwrite)"
fi
rm -rf "$BDIR"

# --- Layout migration: merge archive/ into existing closed/ (no conflict) ---
MDIR2=$(mktemp -d)
mkdir -p "$MDIR2/tickets"
mkdir -p "$MDIR2/archive"
printf '%%erg v1\nTitle: Old\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n' > "$MDIR2/archive/0001-alpha.erg"
mkdir -p "$MDIR2/closed"
printf '%%erg v1\nTitle: Existing\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n' > "$MDIR2/closed/0002-beta.erg"
cp "$ERG" "$MDIR2/tickets/erg"
"$ERG" migrate "$MDIR2/tickets" >/dev/null 2>&1
if [ ! -d "$MDIR2/archive" ] && [ -f "$MDIR2/closed/0001-alpha.erg" ]; then
    pass "layout migration: merged archive/ into closed/ (no conflict)"
else
    fail "layout migration: merged archive/ into closed/ (no conflict)"
fi
rm -rf "$MDIR2"

# --- Layout migration: collision-abort when archive/ and closed/ share a filename ---
MDIR3=$(mktemp -d)
mkdir -p "$MDIR3/tickets"
mkdir -p "$MDIR3/archive"
echo "a" > "$MDIR3/archive/0001-conflict.erg"
mkdir -p "$MDIR3/closed"
echo "b" > "$MDIR3/closed/0001-conflict.erg"
cp "$ERG" "$MDIR3/tickets/erg"
EXIT_CODE=0
"$ERG" migrate "$MDIR3/tickets" >/dev/null 2>&1 || EXIT_CODE=$?
if [ "$EXIT_CODE" -ne 0 ] && [ -d "$MDIR3/archive" ] && [ -f "$MDIR3/archive/0001-conflict.erg" ]; then
    pass "layout migration: collision-abort exits non-zero and leaves archive/ untouched"
else
    fail "layout migration: collision-abort exits non-zero and leaves archive/ untouched"
fi
rm -rf "$MDIR3"

echo "migrate: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
