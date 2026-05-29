#!/bin/sh
# Integration tests for: UX smoke — first-five-minutes paths work end-to-end
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg ux ==="

TDIR=$(mktemp -d)
trap 'rm -rf "$TDIR"' EXIT

# ---------------------------------------------------------------
# 1. POSIX zero-install path (README "Start in 10 seconds")
# ---------------------------------------------------------------

POSIX_DIR="$TDIR/posix"
mkdir -p "$POSIX_DIR/tickets"

cat > "$POSIX_DIR/tickets/0001-add-auth.erg" <<'TICKET'
%erg 0.1
Title: Add authentication flow
Created: 2026-05-29
Author: me

--- log ---
2026-05-29T10:00Z me created

--- body ---
Need auth before shipping the API.
TICKET

if $ERG validate "$POSIX_DIR/tickets/0001-add-auth.erg" >/dev/null 2>&1; then
    pass "posix path: hand-written ticket validates"
else
    fail "posix path: hand-written ticket should validate"
fi

# grep -L '^Closed:' finds open tickets — the README's query
open=$( grep -L '^Closed:' "$POSIX_DIR/tickets/"*.erg ) || true
if echo "$open" | grep -q "0001-add-auth.erg"; then
    pass "posix path: grep finds open ticket"
else
    fail "posix path: grep should find the open ticket"
fi

# ---------------------------------------------------------------
# 2. Time-to-first-ticket budget (POSIX path)
# ---------------------------------------------------------------

BUDGET_DIR="$TDIR/budget"
t_start=$(date +%s)
mkdir -p "$BUDGET_DIR/tickets"
cat > "$BUDGET_DIR/tickets/0001-quick.erg" <<'TICKET'
%erg 0.1
Title: Quick ticket
Created: 2026-05-29
Author: me

--- log ---
2026-05-29T10:00Z me created

--- body ---
Speed test.
TICKET
$ERG validate "$BUDGET_DIR/tickets/0001-quick.erg" >/dev/null 2>&1
t_end=$(date +%s)
elapsed=$((t_end - t_start))

if [ "$elapsed" -le 5 ]; then
    pass "time-to-first-ticket: ${elapsed}s (budget: 5s)"
else
    fail "time-to-first-ticket: ${elapsed}s exceeds 5s budget"
fi

# ---------------------------------------------------------------
# 3. Orientation: no-args prints usage and exits nonzero
# ---------------------------------------------------------------

noargs_out=$($ERG 2>&1) || noargs_rc=$?
noargs_rc=${noargs_rc:-0}

if [ "$noargs_rc" -ne 0 ]; then
    pass "no-args: exits nonzero (rc=$noargs_rc)"
else
    fail "no-args: should exit nonzero"
fi

if echo "$noargs_out" | grep -q "^Usage:"; then
    pass "no-args: prints Usage header"
else
    fail "no-args: should print Usage header"
fi

if echo "$noargs_out" | grep -q "COMMAND"; then
    pass "no-args: mentions COMMAND in usage"
else
    fail "no-args: should mention COMMAND in usage"
fi

# ---------------------------------------------------------------
# 4. Binary init path: self-describing install
# ---------------------------------------------------------------

INIT_DIR="$TDIR/initpath"
mkdir -p "$INIT_DIR/tickets"
touch "$INIT_DIR/tickets/erg"

$ERG init "$INIT_DIR" >/dev/null 2>&1

for f in AGENTS.md spec-erg-v1.md integration.md .ergrc; do
    target="$INIT_DIR/tickets/$f"
    if [ -f "$target" ] && [ -s "$target" ]; then
        pass "init path: $f exists and is non-empty"
    else
        fail "init path: $f should exist and be non-empty"
    fi
done

if grep -q "pre-commit" "$INIT_DIR/tickets/integration.md"; then
    pass "init path: integration.md mentions pre-commit setup"
else
    fail "init path: integration.md should mention pre-commit setup"
fi

if grep -q "%erg" "$INIT_DIR/tickets/spec-erg-v1.md"; then
    pass "init path: spec-erg-v1.md mentions %erg format"
else
    fail "init path: spec-erg-v1.md should mention %erg format"
fi

# ---------------------------------------------------------------
# 5. Error recovery: helpful messages on common mistakes
# ---------------------------------------------------------------

ERR_DIR="$TDIR/errtest"
mkdir -p "$ERR_DIR/tickets"

# close nonexistent ticket
close_err=$($ERG close 9999 "test" "$ERR_DIR/tickets" 2>&1) || true
if echo "$close_err" | grep -qi "no ticket found\|not found\|9999"; then
    pass "error recovery: close 9999 mentions the missing ID"
else
    fail "error recovery: close 9999 should mention the missing ID (got: $close_err)"
fi

# validate nonexistent file
val_err=$($ERG validate "$TDIR/nonexistent.erg" 2>&1) || true
if echo "$val_err" | grep -qi "skip\|warning\|no .erg files\|not found\|no such"; then
    pass "error recovery: validate nonexistent file gives useful feedback"
else
    fail "error recovery: validate nonexistent file should give useful feedback (got: $val_err)"
fi

# unknown command
unk_err=$($ERG badcommand 2>&1) || true
if echo "$unk_err" | grep -qi "unknown command\|badcommand"; then
    pass "error recovery: unknown command names the bad input"
else
    fail "error recovery: unknown command should name the bad input (got: $unk_err)"
fi

# ---------------------------------------------------------------
# 6. erg new: binary path to first ticket
# ---------------------------------------------------------------

NEW_DIR="$TDIR/newtest"
mkdir -p "$NEW_DIR/tickets"

new_out=$($ERG new "My first ticket" "$NEW_DIR/tickets" 2>&1) || new_rc=$?
new_rc=${new_rc:-0}

if [ "$new_rc" -eq 0 ]; then
    pass "erg new: exits 0"
else
    fail "erg new: should exit 0 (rc=$new_rc, out=$new_out)"
fi

new_count=$(find "$NEW_DIR/tickets" -name '*.erg' | wc -l)
if [ "$new_count" -ge 1 ]; then
    pass "erg new: creates a .erg file"
    new_file=$(find "$NEW_DIR/tickets" -name '*.erg' -print -quit)
    if $ERG validate "$new_file" >/dev/null 2>&1; then
        pass "erg new: created ticket validates"
    else
        fail "erg new: created ticket should validate"
    fi
else
    fail "erg new: should create a .erg file"
fi

echo "ux: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
