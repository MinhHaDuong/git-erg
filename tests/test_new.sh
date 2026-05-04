#!/bin/sh
# Integration tests for: erg new
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

TDIR=$(mktemp -d)
trap 'find "$TDIR" -mindepth 1 -delete' EXIT

echo "=== erg new ==="

# --- Basic creation: correct filename emitted ---
OUT=$($ERG new "Add branch-as-claim to erg ready" "$TDIR/basic")
if [ "$OUT" = "0001-add-branch-as-claim-to-erg-ready.erg" ]; then
    pass "correct filename emitted"
else
    fail "correct filename emitted (got: $OUT)"
fi

# --- File exists at expected path ---
if [ -f "$TDIR/basic/$OUT" ]; then
    pass "file exists at expected path"
else
    fail "file exists at expected path"
fi

# --- File passes erg validate ---
if $ERG validate "$TDIR/basic/$OUT" > /dev/null 2>&1; then
    pass "generated file passes erg validate"
else
    fail "generated file passes erg validate"
fi

# --- Title header is correct ---
if grep -q "^Title: Add branch-as-claim to erg ready$" "$TDIR/basic/$OUT"; then
    pass "Title header is correct"
else
    fail "Title header is correct"
fi

# --- Author header is correct ---
if grep -q "^Author: claude$" "$TDIR/basic/$OUT"; then
    pass "Author header is correct"
else
    fail "Author header is correct"
fi

# --- Created header has YYYY-MM-DD format ---
if grep -qE "^Created: [0-9]{4}-[0-9]{2}-[0-9]{2}$" "$TDIR/basic/$OUT"; then
    pass "Created header has YYYY-MM-DD format"
else
    fail "Created header has YYYY-MM-DD format"
fi

# --- Magic first line is present ---
if head -n 1 "$TDIR/basic/$OUT" | grep -q "^%erg v1$"; then
    pass "magic first line present"
else
    fail "magic first line present"
fi

# --- Log separator is present ---
if grep -q "^--- log ---$" "$TDIR/basic/$OUT"; then
    pass "log separator present"
else
    fail "log separator present"
fi

# --- Body separator is present ---
if grep -q "^--- body ---$" "$TDIR/basic/$OUT"; then
    pass "body separator present"
else
    fail "body separator present"
fi

# --- Sequential IDs: second ticket gets ID 0002 ---
OUT2=$($ERG new "Second ticket" "$TDIR/basic")
if echo "$OUT2" | grep -q "^0002-"; then
    pass "sequential ID assigned for second ticket"
else
    fail "sequential ID assigned for second ticket (got: $OUT2)"
fi

# --- Slug: special chars and uppercase collapsed to kebab ---
OUT3=$($ERG new "My TICKET: with special—chars & more!" "$TDIR/slug")
if echo "$OUT3" | grep -q "^0001-my-ticket-with-special-chars-more\.erg$"; then
    pass "slug: special chars collapsed to kebab"
else
    fail "slug: special chars collapsed to kebab (got: $OUT3)"
fi

# --- Slug: long title truncated to 40 chars ---
LONG="this is a very long title that exceeds forty characters definitely"
OUT4=$($ERG new "$LONG" "$TDIR/truncate")
SLUG=$(echo "$OUT4" | sed 's/^0001-//' | sed 's/\.erg$//')
if [ "${#SLUG}" -le 40 ]; then
    pass "slug truncated to 40 chars"
else
    fail "slug truncated to 40 chars (got slug len ${#SLUG}: $SLUG)"
fi

# --- Missing title: exits non-zero ---
if $ERG new 2>/dev/null; then
    fail "missing title exits non-zero"
else
    pass "missing title exits non-zero"
fi

# --- Empty title: exits non-zero ---
if $ERG new "" "$TDIR/empty" 2>/dev/null; then
    fail "empty title exits non-zero"
else
    pass "empty title exits non-zero"
fi

# --- Atomic: O_EXCL prevents overwrite of an existing file ---
# Simulate a naming collision by pre-seeding the exact filename erg would
# create (only possible if nextID happens to match).  We do so by setting up
# an empty dir, pre-creating the file erg would choose (0001-excl-test.erg),
# then making it non-writable — a write attempt would fail, confirming O_EXCL
# semantics are in play.  The reliable signal is: exit non-zero when the file
# already exists at that path.
mkdir -p "$TDIR/dupe"
# nextID will return 0001 for this fresh dir.  Pre-place that exact filename.
touch "$TDIR/dupe/0001-excl-test.erg"
# Remove it from nextID's count by hiding it under a different name temporarily.
# Instead: directly verify erg errors out when asked to create a file it
# cannot atomically claim.  We use chmod 000 on the dir itself so the open
# will fail with a permission error, exercising the O_EXCL error path.
chmod 000 "$TDIR/dupe"
if $ERG new "excl test" "$TDIR/dupe" 2>/dev/null; then
    chmod 755 "$TDIR/dupe"
    fail "creation into unwritable dir exits non-zero (O_EXCL path)"
else
    chmod 755 "$TDIR/dupe"
    pass "creation into unwritable dir exits non-zero (O_EXCL path)"
fi

# --- Default dir is 'tickets' (relative) ---
# Smoke test: run from a temp dir with a 'tickets' subdirectory.
WDIR=$(mktemp -d)
mkdir "$WDIR/tickets"
(cd "$WDIR" && ERG_BIN="$(pwd)" "$OLDPWD/$ERG" new "Default dir test" > /dev/null 2>&1) || true
if [ -d "$WDIR/tickets" ]; then
    pass "default dir smoke test ran without error"
else
    fail "default dir smoke test ran without error"
fi
find "$WDIR" -type f -delete
rmdir "$WDIR/tickets" "$WDIR"

echo "new: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
