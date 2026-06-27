#!/bin/sh
# Integration tests for: end-to-end pipeline (new → log → close → archive)
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

TDIR=$(mktemp -d)
trap 'rm -rf "$TDIR"' EXIT

echo "=== pipeline: new → log → close → archive ==="

# 1. Create a ticket
FILE=$($ERG new "Pipeline smoke test" "$TDIR" | sed 's/^CREATED //')
ID=$(echo "$FILE" | cut -c1-4)
if [ -f "$TDIR/$FILE" ]; then
    pass "new: ticket file created"
else
    fail "new: ticket file created (got: $FILE)"
fi

# 2. Log a note
$ERG log "$ID" "claude progress — phase 1 done" "$TDIR" > /dev/null
if grep -q "phase 1 done" "$TDIR/$FILE"; then
    pass "log: line appended"
else
    fail "log: line appended"
fi

# 3. Close it -- close now files the ticket under closed/ in one step.
$ERG close "$ID" "pipeline complete" "$TDIR" > /dev/null
if grep -q "^Closed: pipeline complete" "$TDIR/closed/$FILE"; then
    pass "close: Closed header written"
else
    fail "close: Closed header written"
fi
if [ -f "$TDIR/closed/$FILE" ] && [ ! -f "$TDIR/$FILE" ]; then
    pass "close: file filed under closed/"
else
    fail "close: file filed under closed/"
fi

# 4. Archive is now a no-op (close already filed it); the ticket stays put.
$ERG archive "$TDIR" > /dev/null
if [ -f "$TDIR/closed/$FILE" ]; then
    pass "archive: closed ticket remains under closed/"
else
    fail "archive: closed ticket remains under closed/"
fi
if [ ! -f "$TDIR/$FILE" ]; then
    pass "archive: nothing left at top-level"
else
    fail "archive: nothing left at top-level"
fi

# 5. erg check passes on the result
if $ERG check "$TDIR" > /dev/null 2>&1; then
    pass "check: corpus clean after pipeline"
else
    fail "check: corpus clean after pipeline"
fi

echo "pipeline: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
