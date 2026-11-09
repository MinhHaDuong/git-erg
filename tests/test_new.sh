#!/bin/sh
# Integration tests for: erg new
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

# Claims are required (ticket 0307): every store below is made compliant with
# plain files first, so each case fails for its own defect.
. "$(dirname "$0")/claims_fixture.sh"

TDIR=$(mktemp -d)
WDIR=
cleanup() {
    find "$TDIR" -mindepth 1 -delete 2>/dev/null || true
    [ -n "$WDIR" ] && rm -rf "$WDIR"
}
trap cleanup EXIT

echo "=== erg new ==="

# --- Basic creation: correct filename emitted ---
claim_store "$TDIR/basic"
RAW=$($ERG new "Add branch-as-claim to the ready filter" "$TDIR/basic")
if echo "$RAW" | grep -q "^CREATED "; then pass "erg new output starts with CREATED"; else fail "erg new output starts with CREATED (got: $RAW)"; fi
OUT=$(echo "$RAW" | sed 's/^CREATED //')
if [ "$OUT" = "0001-add-branch-as-claim-to-the-ready-filter.erg" ]; then
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

# Subsumes: magic line, log/body separators, Title/Author/Created headers.
# --- File passes erg validate ---
if $ERG validate "$TDIR/basic/$OUT" > /dev/null 2>&1; then
    pass "generated file passes erg validate"
else
    fail "generated file passes erg validate"
fi

# --- Rule 14: erg new refuses a status-word-edge title (would self-invalidate) ---
out=$($ERG new "ready: do the thing" "$TDIR/r14" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "status word 'ready'"; then
    pass "rule 14: erg new rejects status-word-edge title"
else
    fail "rule 14: erg new rejects status-word-edge title (rc=$rc, got: $out)"
fi
if [ ! -d "$TDIR/r14" ] || [ -z "$(ls -A "$TDIR/r14" 2>/dev/null)" ]; then
    pass "rule 14: erg new creates no file on rejection"
else
    fail "rule 14: erg new creates no file on rejection (found: $(ls -A "$TDIR/r14"))"
fi
# Mid-title status word is still allowed.
claim_store "$TDIR/r14ok"
if $ERG new "respect the open flag mid title" "$TDIR/r14ok" >/dev/null 2>&1; then
    pass "rule 14: erg new allows mid-title status word"
else
    fail "rule 14: erg new allows mid-title status word"
fi

# --- Sequential IDs: second ticket gets ID 0002 ---
OUT2=$($ERG new "Second ticket" "$TDIR/basic" | sed 's/^CREATED //')
if echo "$OUT2" | grep -q "^0002-"; then
    pass "sequential ID assigned for second ticket"
else
    fail "sequential ID assigned for second ticket (got: $OUT2)"
fi

# --- Slug: special chars and uppercase collapsed to kebab ---
claim_store "$TDIR/slug"
OUT3=$($ERG new "My TICKET: with special—chars & more!" "$TDIR/slug" | sed 's/^CREATED //')
if echo "$OUT3" | grep -q "^0001-my-ticket-with-special-chars-more\.erg$"; then
    pass "slug: special chars collapsed to kebab"
else
    fail "slug: special chars collapsed to kebab (got: $OUT3)"
fi

# --- Slug: long title truncated to 40 chars ---
LONG="this is a very long title that exceeds forty characters definitely"
claim_store "$TDIR/truncate"
OUT4=$($ERG new "$LONG" "$TDIR/truncate" | sed 's/^CREATED //')
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

# --- File creation error handling ---
# Test the adjacent error path: erg new exits non-zero when the target
# directory is not writable. The store is compliant and readable (555), so
# the refusal is the ticket write's, not a missing or unreadable .claims/.
if [ "$(id -u)" -eq 0 ]; then
    # chmod-based write protection is a no-op for root; this path can only be
    # exercised as an unprivileged user.
    echo "  SKIP (root): erg new exits non-zero on unwritable dir"
else
    claim_store "$TDIR/dupe"
    chmod 555 "$TDIR/dupe"
    out=$($ERG new "dupe test" "$TDIR/dupe" 2>&1) && rc=0 || rc=$?
    chmod 755 "$TDIR/dupe"
    if [ "$rc" -ne 0 ] && echo "$out" | grep -qF "cannot create $TDIR/dupe/"; then
        pass "file creation error: erg new exits non-zero on unwritable dir"
    else
        fail "file creation error: erg new exits non-zero on unwritable dir (rc=$rc: $out)"
    fi
fi

# --- Default dir is 'tickets' (relative) ---
# Run from a temp dir with a 'tickets' subdirectory; assert erg creates a
# .erg file there (not just that the dir exists, which mkdir already ensured).
ERG_ABS=$(cd "$(dirname "$ERG")" && pwd)/$(basename "$ERG")
WDIR=$(mktemp -d)
mkdir "$WDIR/tickets"
claim_store "$WDIR/tickets"
(cd "$WDIR" && "$ERG_ABS" new "Default dir test" > /dev/null 2>&1)
if ls "$WDIR/tickets/"*.erg 2>/dev/null | grep -q .; then
    pass "default dir: ticket file created in tickets/ subdirectory"
else
    fail "default dir: no ticket file found in tickets/ subdirectory"
fi
find "$WDIR" -delete

# ERG_AUTHOR: Author header and log line use the override
claim_store "$TDIR/authtest"
RAW_A=$(ERG_AUTHOR=testuser $ERG new "author override test" "$TDIR/authtest")
FNAME_A=$(echo "$RAW_A" | sed 's/^CREATED //')
FILE_A="$TDIR/authtest/$FNAME_A"
if grep -q "^Author: testuser$" "$FILE_A" && grep -q "testuser created" "$FILE_A"; then
    pass "ERG_AUTHOR: Author header and log line use override"
else
    fail "ERG_AUTHOR: Author header and log line use override"
fi

# --- Sequential negative control: N=10 distinct titles produce distinct IDs ---
# Proves the concurrent assertions below are not vacuously true.
SEQDIR=$(mktemp -d)
claim_store "$SEQDIR"
seq_fail=0
for i in 1 2 3 4 5 6 7 8 9 10; do
    $ERG new "Sequential ticket number $i" "$SEQDIR" >/dev/null 2>&1 || seq_fail=1
done
if [ "$seq_fail" -eq 0 ]; then
    SEQCOUNT=$(ls "$SEQDIR"/*.erg 2>/dev/null | wc -l)
    SEQIDS=$(ls "$SEQDIR"/*.erg 2>/dev/null | xargs -I{} basename {} .erg | cut -d- -f1 | sort -u | wc -l)
    if [ "$SEQCOUNT" -eq 10 ] && [ "$SEQIDS" -eq 10 ]; then
        pass "sequential N=10: all 10 succeed with distinct IDs"
    else
        fail "sequential N=10: expected 10 files with 10 distinct IDs (files=$SEQCOUNT, unique_ids=$SEQIDS)"
    fi
else
    fail "sequential N=10: one or more erg new invocations failed"
fi
find "$SEQDIR" -delete 2>/dev/null || true

# --- Concurrent N=10: parallel erg new with distinct titles produces distinct IDs ---
# Uses a named pipe barrier to start all workers simultaneously.
# CDIR is outside the repo so nextID Pass 2/3 (sibling worktrees/branches) does
# not inject existing IDs and break the assertions. In a claims store (every
# store, ticket 0307) the exclusive create of the claim keeps racing workers
# apart; a store without .claims/ refuses every worker and gains nothing.
concurrent_n10() {
    mode=$1
    CDIR=$(mktemp -d)
    [ "$mode" = claims ] && claim_store "$CDIR"
    OUTDIR=$(mktemp -d)
    BARRIER="$OUTDIR/barrier"
    mkfifo "$BARRIER"

    # Start 10 workers; each blocks on the barrier FIFO read before invoking erg new.
    for i in 1 2 3 4 5 6 7 8 9 10; do
        (
            read -r _ < "$BARRIER" 2>/dev/null || true
            $ERG new "Concurrent worker title $i" "$CDIR" > "$OUTDIR/out.$i" 2> "$OUTDIR/err.$i"
        ) &
    done

    # Give workers time to reach the FIFO read, then release all simultaneously.
    # Writing once and closing the write end sends EOF to all blocked readers.
    sleep 0.2
    printf "go\n" > "$BARRIER"

    wait

    rm -f "$BARRIER"

    # Count how many workers produced a CREATED line, and how many were
    # refused for the missing .claims/.
    concurrent_created=0
    concurrent_refused=0
    for i in 1 2 3 4 5 6 7 8 9 10; do
        if grep -q "^CREATED " "$OUTDIR/out.$i" 2>/dev/null; then
            concurrent_created=$((concurrent_created + 1))
        fi
        if grep -qF "has no .claims/: ID claims are required -- run: erg migrate --claims $CDIR" "$OUTDIR/err.$i" 2>/dev/null; then
            concurrent_refused=$((concurrent_refused + 1))
        fi
    done
    CTOTAL=$(ls "$CDIR"/*.erg 2>/dev/null | wc -l)

    if [ "$mode" = no-claims ]; then
        if [ "$concurrent_refused" -eq 10 ] && [ "$CTOTAL" -eq 0 ] && [ ! -e "$CDIR/.claims" ]; then
            pass "concurrent N=10 (no .claims/): all 10 workers refused, naming the repair; nothing written"
        else
            fail "concurrent N=10 (no .claims/): refused=$concurrent_refused files=$CTOTAL, want 10 refusals and no file"
        fi
        rm -rf "$CDIR" "$OUTDIR"
        return 0
    fi

    if [ "$concurrent_created" -eq 10 ]; then
        pass "concurrent N=10 ($mode): all 10 workers produced CREATED"
    else
        fail "concurrent N=10 ($mode): only $concurrent_created/10 workers produced CREATED"
    fi

    # All 10 NNNN prefixes must be distinct.
    CUNIQ=$(ls "$CDIR"/*.erg 2>/dev/null | xargs -I{} basename {} .erg | cut -d- -f1 | sort -u | wc -l)
    if [ "$CUNIQ" -eq 10 ]; then
        pass "concurrent N=10 ($mode): all 10 NNNN prefixes are distinct"
    else
        fail "concurrent N=10 ($mode): expected 10 distinct IDs, got $CUNIQ unique out of $CTOTAL files"
    fi

    # Every worker's number is claimed exactly once.
    CCLAIMS=$(ls "$CDIR/.claims" 2>/dev/null | wc -l)
    if [ "$CCLAIMS" -eq 10 ]; then
        pass "concurrent N=10 ($mode): 10 claims, one per ticket"
    else
        fail "concurrent N=10 ($mode): expected 10 claims, got $CCLAIMS"
    fi

    # erg check must pass on the resulting store.
    if $ERG check "$CDIR" > /dev/null 2>&1; then
        pass "concurrent N=10 ($mode): erg check passes on resulting store"
    else
        fail "concurrent N=10 ($mode): erg check fails on resulting store"
    fi

    rm -rf "$CDIR" "$OUTDIR"
}
concurrent_n10 claims
concurrent_n10 no-claims

# --- --author flag: explicit author header ---
claim_store "$TDIR/flagauth"
RAW_F=$($ERG new "flag author test" "$TDIR/flagauth" --author alice)
FNAME_F=$(echo "$RAW_F" | sed 's/^CREATED //')
FILE_F="$TDIR/flagauth/$FNAME_F"
if grep -q "^Author: alice$" "$FILE_F" && grep -q "alice created" "$FILE_F"; then
    pass "--author flag: Author header and log line use flag value"
else
    fail "--author flag: Author header and log line use flag value"
fi

# --- -a shorthand ---
claim_store "$TDIR/shortauth"
RAW_S=$($ERG new "short auth test" "$TDIR/shortauth" -a bob)
FNAME_S=$(echo "$RAW_S" | sed 's/^CREATED //')
FILE_S="$TDIR/shortauth/$FNAME_S"
if grep -q "^Author: bob$" "$FILE_S"; then
    pass "-a flag: Author header uses flag value"
else
    fail "-a flag: Author header uses flag value"
fi

# --- --author=value form ---
claim_store "$TDIR/eqauth"
RAW_E=$($ERG new "eq auth test" "$TDIR/eqauth" --author=carol)
FNAME_E=$(echo "$RAW_E" | sed 's/^CREATED //')
FILE_E="$TDIR/eqauth/$FNAME_E"
if grep -q "^Author: carol$" "$FILE_E"; then
    pass "--author=value form: Author header uses flag value"
else
    fail "--author=value form: Author header uses flag value"
fi

# --- --author overrides ERG_AUTHOR env var ---
claim_store "$TDIR/overrideauth"
RAW_O=$(ERG_AUTHOR=env-author $ERG new "override auth test" "$TDIR/overrideauth" --author flag-author)
FNAME_O=$(echo "$RAW_O" | sed 's/^CREATED //')
FILE_O="$TDIR/overrideauth/$FNAME_O"
if grep -q "^Author: flag-author$" "$FILE_O"; then
    pass "--author overrides ERG_AUTHOR env var"
else
    fail "--author overrides ERG_AUTHOR env var (expected flag-author, got: $(grep '^Author:' "$FILE_O" 2>/dev/null || echo 'no Author line'))"
fi

# --- unknown flag exits non-zero and does NOT create a --bogus directory ---
BOGUS_DIR="$TDIR/bogustest"
mkdir -p "$BOGUS_DIR"
# Run from $BOGUS_DIR as cwd; the old bug silently created ./--bogus/ relative to cwd
out_bogus=$( (cd "$BOGUS_DIR" && $ERG new "bogus test" --bogus) 2>&1) && rc_bogus=0 || rc_bogus=$?
if [ "$rc_bogus" -ne 0 ]; then
    pass "unknown flag: exits non-zero"
else
    fail "unknown flag: exits non-zero (got rc=0)"
fi
if [ ! -d "$BOGUS_DIR/--bogus" ]; then
    pass "unknown flag: --bogus directory not created"
else
    fail "unknown flag: --bogus directory was created"
fi

echo "new: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
