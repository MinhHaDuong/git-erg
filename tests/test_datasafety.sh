#!/bin/sh
# Data-safety guard suite — never lose or corrupt a ticket (ticket 0149).
#
# erg's one job is custody of tickets; losing or corrupting one is the worst,
# irreversible failure. This suite black-box-checks the data-safety invariants
# on the real binary. Each guard is falsifiable: it asserts the protected
# behaviour AND that the file survives the refusal intact, so removing the
# safety code makes the test fail.
#
# Companion unit tests in src/go/atomicwrite_test.go own the function-level
# guards (atomic temp+rename, validate-before-replace on arbitrary bytes,
# confinement predicate, byte round-trip).
set -eu

ERG="${ERG_BIN:-build/erg}"
FIXTURES=$(mktemp -d)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

trap 'rm -rf "$FIXTURES"' EXIT

echo "=== data-safety guard suite ==="

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

# Inode of a file. Prefer `stat -c %i` (GNU/BusyBox) for an unambiguous value;
# fall back to `ls -i` where stat is absent.
inode_of() { stat -c %i "$1" 2>/dev/null || ls -i "$1" | awk '{print $1}'; }

# ---------------------------------------------------------------------------
# Guard 1 — Atomic replace (temp-then-rename, never truncate-in-place).
# Negative control: a temp+rename gives the file a NEW inode every mutation.
# An in-place truncating writer keeps the same inode, failing this assertion.
#
# This is also the structural basis for crash/interrupt safety: because the
# target is replaced by rename and never opened for truncation, an erg killed
# before the rename leaves the complete old file and one killed after leaves the
# complete new file — never a truncated one. The inode change proves the no-in-
# place-write property; it is a structural proxy, not a power-loss test (fsync
# durability is implemented in atomicWriteFile but cannot be unit-tested without
# simulating power loss).
# ---------------------------------------------------------------------------
WS="$FIXTURES/atomic"
mkdir -p "$WS"
write_open "$WS/0001-atom.erg" "Atom"
for cmd in log label close; do
    # close files the ticket under closed/ in one step, so the post-mutation
    # file lands there; log/label rewrite in place.
    fpath="$WS/0001-atom.erg"
    case "$cmd" in
    log) write_open "$WS/0001-atom.erg" "Atom"; before=$(inode_of "$WS/0001-atom.erg")
         $ERG log 0001 "claude note touched" "$WS" >/dev/null 2>&1 ;;
    label) write_open "$WS/0001-atom.erg" "Atom"; before=$(inode_of "$WS/0001-atom.erg")
         $ERG label 0001 needs-human "$WS" >/dev/null 2>&1 ;;
    close) write_open "$WS/0001-atom.erg" "Atom"; before=$(inode_of "$WS/0001-atom.erg")
         $ERG close 0001 "done" "$WS" >/dev/null 2>&1; fpath="$WS/closed/0001-atom.erg" ;;
    esac
    after=$(inode_of "$fpath")
    if [ "$before" != "$after" ] && $ERG validate "$fpath" >/dev/null 2>&1; then
        pass "atomic: erg $cmd replaces via rename (inode $before → $after) leaving a valid file"
    else
        fail "atomic: erg $cmd should rename, not truncate-in-place (before=$before after=$after)"
    fi
done

# ---------------------------------------------------------------------------
# Guard 2 — Lossless round-trip: a mutation preserves the body and prior log
# verbatim, adding only the intended line.
# Negative control: the body bytes before and after must be identical, and the
# log must grow by exactly one line. A writer that reorders/drops content fails.
# ---------------------------------------------------------------------------
WS="$FIXTURES/lossless"
mkdir -p "$WS"
cat > "$WS/0001-rt.erg" <<'EOF'
%erg 0.1
Title: Round trip
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created
2026-01-02T11:30Z claude note investigated the corpus

--- body ---
## Context

Body text with    odd   spacing and a trailing line.
- a bullet
- another
EOF
# Snapshot body (everything after the body separator) and the log line count.
# Body is extracted to files and compared with cmp — byte-for-byte, so a
# trailing-newline-only difference is caught (command substitution would strip
# it and mask such a change).
extract_body() { awk '/^--- body ---$/{f=1;next} f' "$1" > "$2"; }
extract_body "$WS/0001-rt.erg" "$WS/.body_before"
logcount_before=$(awk '/^--- log ---$/{f=1;next} /^--- body ---$/{f=0} f && NF' "$WS/0001-rt.erg" | wc -l)
$ERG log 0001 "claude note round-trip check" "$WS" >/dev/null 2>&1
extract_body "$WS/0001-rt.erg" "$WS/.body_after"
logcount_after=$(awk '/^--- log ---$/{f=1;next} /^--- body ---$/{f=0} f && NF' "$WS/0001-rt.erg" | wc -l)
if cmp -s "$WS/.body_before" "$WS/.body_after"; then
    pass "lossless: body preserved verbatim across erg log"
else
    fail "lossless: body changed across erg log"
fi
if [ "$logcount_after" -eq "$((logcount_before + 1))" ]; then
    pass "lossless: log grew by exactly one line ($logcount_before → $logcount_after)"
else
    fail "lossless: log line count $logcount_before → $logcount_after (expected +1)"
fi

# The property also holds for close/label/unlabel — the other log/header mutators
# (the ticket names "close/label/unlabel/rm --force"). They funnel through the same
# appendLogLine + writeTicketAtomic path; assert the body survives each
# byte-for-byte (cmp on extracted files, same as the log case above).
write_rt2() {
    cat > "$WS/0002-rt.erg" <<'EOF'
%erg 0.1
Title: Round trip two
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
## Body

Verbatim    spacing and bullets:
- one
- two
EOF
}
write_rt2; extract_body "$WS/0002-rt.erg" "$WS/.b_want"
$ERG close 0002 "done" "$WS" >/dev/null 2>&1
extract_body "$WS/closed/0002-rt.erg" "$WS/.b_after"
if cmp -s "$WS/.b_want" "$WS/.b_after"; then
    pass "lossless: body preserved verbatim across erg close"
else
    fail "lossless: body changed across erg close"
fi
write_rt2; extract_body "$WS/0002-rt.erg" "$WS/.b_want"
$ERG label 0002 needs-human "$WS" >/dev/null 2>&1
$ERG unlabel 0002 needs-human "$WS" >/dev/null 2>&1
extract_body "$WS/0002-rt.erg" "$WS/.b_after"
if cmp -s "$WS/.b_want" "$WS/.b_after"; then
    pass "lossless: body preserved verbatim across erg label + unlabel"
else
    fail "lossless: body changed across erg label/unlabel"
fi

# ---------------------------------------------------------------------------
# Guard 3 — Validate-before-replace (correct semantics).
# The gate refuses a write only when it would turn a CLEAN ticket invalid —
# "never write garbage over a good ticket". That refusal is unit-tested in
# atomicwrite_test.go (TestWriteTicketAtomicRefusesInvalid): a CLI command never
# produces invalid output from valid input, so the refusal cannot be triggered
# black-box. Here we lock the complementary direction an over-strict first cut
# got wrong — the gate must NOT block a mutation on an ALREADY-invalid ticket,
# or close would leave a dangling Blocked-by on a dependent that carries an
# unrelated pre-existing violation.
# Negative control: a refuse-on-any-error gate leaves "Blocked-by: 0001" in the
# (invalid) dependent, and erg check then flags the dangling ref.
# ---------------------------------------------------------------------------
WS="$FIXTURES/validate"
mkdir -p "$WS"
write_open "$WS/0001-blk.erg" "Blocker"
cat > "$WS/0002-dep.erg" <<'EOF'
%erg 0.1
Title: Dependent
Created: 2026-01-01
Author: claude
Bogus: x
Blocked-by: 0001

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
if $ERG validate "$WS/0002-dep.erg" >/dev/null 2>&1; then
    fail "validate-before-replace: fixture precondition — dependent should already be invalid"
else
    pass "validate-before-replace: fixture dependent is already invalid (unrelated violation)"
fi
$ERG close 0001 "done" "$WS" >/dev/null 2>&1
if grep -q '^Blocked-by' "$WS/0002-dep.erg"; then
    fail "validate-before-replace: gate over-blocked — dangling Blocked-by left on invalid dependent"
else
    pass "validate-before-replace: close still clears the edge on an already-invalid dependent"
fi

# ---------------------------------------------------------------------------
# Guard 4 — Write-confinement: a mutation whose target resolves outside the
# named store is refused, and nothing outside the store is written.
# Negative control: close a FILE that lives outside the explicit DIR. Remove
# the confinement rail and the out-of-store file would be rewritten.
# ---------------------------------------------------------------------------
WS="$FIXTURES/confine"
mkdir -p "$WS/store"
write_open "$WS/0009-stray.erg" "Stray"
cp "$WS/0009-stray.erg" "$WS/.snapshot"
out=$($ERG close "$WS/0009-stray.erg" "done" "$WS/store" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "outside the ticket store"; then
    pass "confinement: close refuses a FILE outside the named store"
else
    fail "confinement: close should refuse out-of-store target (rc=$rc, got: $out)"
fi
if cmp -s "$WS/0009-stray.erg" "$WS/.snapshot"; then
    pass "confinement: out-of-store file left intact after refusal"
else
    fail "confinement: out-of-store file was modified despite refusal"
fi

# ---------------------------------------------------------------------------
# Guard 5 — No-clobber: archive never overwrites an existing destination.
# Negative control: pre-place a different file at the archive destination. The
# source must remain and the destination must keep its contents. Remove the
# stat check and os.Rename would clobber the destination and delete the source.
# ---------------------------------------------------------------------------
WS="$FIXTURES/noclobber"
mkdir -p "$WS/closed"
cat > "$WS/0001-clob.erg" <<'EOF'
%erg 0.1
Title: Clob
Created: 2026-01-01
Author: claude
Closed: done

--- log ---
2026-01-01T10:00Z claude created
2026-01-01T11:00Z claude closed — done

--- body ---
EOF
echo "PRE-EXISTING DESTINATION — must not be overwritten" > "$WS/closed/0001-clob.erg"
cp "$WS/closed/0001-clob.erg" "$WS/.dst-snapshot"
out=$($ERG archive "$WS" 2>&1) || true
if [ -f "$WS/0001-clob.erg" ]; then
    pass "no-clobber: source ticket preserved when destination exists"
else
    fail "no-clobber: source ticket disappeared (destination was clobbered)"
fi
if cmp -s "$WS/closed/0001-clob.erg" "$WS/.dst-snapshot"; then
    pass "no-clobber: existing destination left untouched"
else
    fail "no-clobber: existing destination was overwritten"
fi

# ---------------------------------------------------------------------------
# Guard 6 — DAG guard (regression of 0142): rm refuses to orphan dependents
# without --force, and deletes nothing on refusal.
# Negative control: rm a blocker that an open ticket depends on. Remove the DAG
# check and the blocker would be deleted, leaving a dangling Blocked-by.
# ---------------------------------------------------------------------------
WS="$FIXTURES/dag"
mkdir -p "$WS"
write_open "$WS/0001-blocker.erg" "Blocker"
cat > "$WS/0002-dependent.erg" <<'EOF'
%erg 0.1
Title: Dependent
Created: 2026-01-01
Author: claude
Blocked-by: 0001

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
out=$($ERG rm 0001 "$WS" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && [ -f "$WS/0001-blocker.erg" ]; then
    pass "DAG: rm refuses to delete a blocker with open dependents (file preserved)"
else
    fail "DAG: rm should refuse and preserve the blocker (rc=$rc, exists=$([ -f "$WS/0001-blocker.erg" ] && echo yes || echo no))"
fi

echo "datasafety: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
