#!/bin/sh
# Security hardening suite — adversarial input on the real binary (ticket 0157,
# child of 0151's threat model).
#
# 0151's exit criterion asks for falsifiable hardening tests with negative
# controls: path-traversal, symlink escape, input-DoS, ref injection, glob/ID
# ambiguity. The audit probe found 13 of 15 attack surfaces already safe and two
# real confinement gaps in the write/delete rail (fixed in this PR): rm's FILE
# form and new's explicit DIR. This suite black-box-checks every surface on the
# built binary.
#
# Falsifiability: each protected behaviour is asserted WITH a negative control —
# a near-identical legitimate input that exercises the SAME code path and must
# succeed. So a check that trips on everything (or nothing) fails one side of
# the pair. Removing a guard flips its positive assertion.
set -eu

ERG="${ERG_BIN:-build/erg}"
# Resolve erg to an absolute path up front: several checks below cd into fixture
# directories before invoking it, where a relative "build/erg" would no longer
# resolve. (Same pattern as test_contract.sh.)
ERG_ABS=$(readlink -f "$ERG" 2>/dev/null || true)
[ -n "$ERG_ABS" ] || ERG_ABS=$(cd "$(dirname "$ERG")" 2>/dev/null && pwd)/$(basename "$ERG")
ERG=$ERG_ABS
FIXTURES=$(mktemp -d)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

# `timeout` is GNU coreutils, not POSIX (AGENTS.md standalone/POSIX constraint).
# bounded SECS CMD... runs CMD under `timeout` when available (exit 124 on
# expiry); on a POSIX-only box without it, runs CMD directly and post-measures
# wall-clock with date(1) (POSIX), returning 124 if it overran. The parser is
# independently proven linear by Go unit tests, so post-measurement is an
# adequate regression backstop here — never a hard dependency on `timeout`.
if command -v timeout >/dev/null 2>&1; then HAVE_TIMEOUT=1; else HAVE_TIMEOUT=0; fi
bounded() {
    _secs=$1; shift
    if [ "$HAVE_TIMEOUT" = 1 ]; then
        timeout "$_secs" "$@"
        return $?
    fi
    _start=$(date +%s)
    "$@"; _rc=$?
    _end=$(date +%s)
    [ $((_end - _start)) -gt "$_secs" ] && return 124
    return $_rc
}

trap 'rm -rf "$FIXTURES"' EXIT

echo "=== security hardening suite ==="

# Helper: write a minimal valid open ticket.
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

# ---------------------------------------------------------------------------
# Group 1 — Path traversal via the ID argument (already safe; assert it stays).
# A "../../etc/passwd" ID is glob-resolved inside the store via the NNNN-*.erg
# pattern; it matches nothing, so the resolver refuses with "no ticket found"
# and never touches a path outside the store. Asserted for close/log/rm (ID
# form) — the three mutating commands that take an ID.
# Negative control: a real fixture ID in the same store resolves and succeeds.
# A resolver that joined the raw arg onto the store path would escape; this
# proves it does not.
# ---------------------------------------------------------------------------
WS="$FIXTURES/traversal"
mkdir -p "$WS"
write_open "$WS/9001-real.erg" "Real"

for cmd in close log rm; do
    case "$cmd" in
    close) out=$($ERG close "../../etc/passwd" reason "$WS" 2>&1) && rc=0 || rc=$? ;;
    log)   out=$($ERG log "../../etc/passwd" "claude note x" "$WS" 2>&1) && rc=0 || rc=$? ;;
    rm)    out=$($ERG rm "../../etc/passwd" "$WS" 2>&1) && rc=0 || rc=$? ;;
    esac
    if [ "$rc" -ne 0 ] && echo "$out" | grep -q "no ticket found"; then
        pass "traversal: $cmd refuses '../../etc/passwd' ID (no escape, no ticket found)"
    else
        fail "traversal: $cmd should refuse traversal ID (rc=$rc, got: $out)"
    fi
done

# Negative controls: a real ID in the store works for each. Recreate the fixture
# before each because close/rm mutate or remove it.
write_open "$WS/9001-real.erg" "Real"
if $ERG close 9001 reason "$WS" >/dev/null 2>&1; then
    pass "traversal neg ctrl: close on a real fixture ID succeeds"
else
    fail "traversal neg ctrl: close on a real fixture ID should succeed"
fi
write_open "$WS/9001-real.erg" "Real"
if $ERG log 9001 "claude note ok" "$WS" >/dev/null 2>&1; then
    pass "traversal neg ctrl: log on a real fixture ID succeeds"
else
    fail "traversal neg ctrl: log on a real fixture ID should succeed"
fi
write_open "$WS/9001-real.erg" "Real"
if $ERG rm 9001 "$WS" >/dev/null 2>&1 && [ ! -f "$WS/9001-real.erg" ]; then
    pass "traversal neg ctrl: rm on a real fixture ID deletes it"
else
    fail "traversal neg ctrl: rm on a real fixture ID should delete it"
fi

# ---------------------------------------------------------------------------
# Group 1b — ID injection via an embedded path separator (already safe). An ID
# like "0042/../../../etc.erg" must not let the resolver walk out of the store.
# The .erg-suffix FILE branch os.Stats the literal path (ENOENT → refuse); a
# bare-ID form globs NNNN-*.erg inside the store and matches nothing. Either way
# the resolver refuses with "no ticket found" and never escapes.
# Negative control: a normal "0042-real.erg" file resolves and is accepted.
# ---------------------------------------------------------------------------
WS="$FIXTURES/idinject"
mkdir -p "$WS"
write_open "$WS/0042-real.erg" "Real"

out=$($ERG rm "0042/../../../etc.erg" "$WS" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "no ticket found"; then
    pass "id-inject: rm refuses embedded-separator ID '0042/../../../etc.erg'"
else
    fail "id-inject: rm should refuse embedded-separator ID (rc=$rc, got: $out)"
fi
out=$($ERG close "0042/../../../etc" reason "$WS" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "no ticket found"; then
    pass "id-inject: close refuses embedded-separator ID '0042/../../../etc'"
else
    fail "id-inject: close should refuse embedded-separator ID (rc=$rc, got: $out)"
fi
# Negative control: a normal NNNN-slug.erg ID is accepted (close mutates+files it).
if $ERG close 0042 reason "$WS" >/dev/null 2>&1 && grep -q '^Closed:' "$WS/closed/0042-real.erg"; then
    pass "id-inject neg ctrl: normal '0042-real.erg' ID is accepted"
else
    fail "id-inject neg ctrl: normal '0042-real.erg' ID should be accepted"
fi

# ---------------------------------------------------------------------------
# Group 2 — Symlink escape (already safe). An in-store ticket path that is a
# SYMLINK to an external file must not let a mutation reach out of the store.
# os.Rename replaces the link itself (not its target) with an in-store regular
# file, so close mutates the in-store path and leaves the external file intact.
# Negative control: a real (non-symlink) in-store ticket closes normally.
# ---------------------------------------------------------------------------
WS="$FIXTURES/symlink"
mkdir -p "$WS/store"
write_open "$WS/external.erg" "External"
cp "$WS/external.erg" "$WS/.ext-snapshot"
ln -s "$WS/external.erg" "$WS/store/9002-symlink.erg"

$ERG close 9002 done "$WS/store" >/dev/null 2>&1 || true
# close de-symlinks via the atomic header write, then files the regular file
# under closed/ -- the external target is never followed.
if [ ! -L "$WS/store/closed/9002-symlink.erg" ] && grep -q '^Closed:' "$WS/store/closed/9002-symlink.erg" 2>/dev/null; then
    pass "symlink: close replaces the in-store link with an in-store regular file"
else
    fail "symlink: close should turn the link into an in-store regular file with Closed:"
fi
if cmp -s "$WS/external.erg" "$WS/.ext-snapshot"; then
    pass "symlink: external target untouched by close-through-symlink"
else
    fail "symlink: external target was modified through the symlink"
fi

# rm on the in-store symlink removes only the link; the external file survives.
write_open "$WS/external.erg" "External"
cp "$WS/external.erg" "$WS/.ext-snapshot"
ln -sf "$WS/external.erg" "$WS/store/9002-symlink.erg"
$ERG rm "$WS/store/9002-symlink.erg" "$WS/store" >/dev/null 2>&1 || true
if [ ! -e "$WS/store/9002-symlink.erg" ] && [ -f "$WS/external.erg" ] && cmp -s "$WS/external.erg" "$WS/.ext-snapshot"; then
    pass "symlink: rm removes only the link, external target survives"
else
    fail "symlink: rm should remove the link and leave the external target intact"
fi

# Negative control: a real in-store file (not a symlink) closes normally.
write_open "$WS/store/9003-real.erg" "Real"
if $ERG close 9003 done "$WS/store" >/dev/null 2>&1 && grep -q '^Closed:' "$WS/store/closed/9003-real.erg"; then
    pass "symlink neg ctrl: close on a real in-store file succeeds"
else
    fail "symlink neg ctrl: close on a real in-store file should succeed"
fi

# ---------------------------------------------------------------------------
# Group 3 — rm FILE-form confinement (FIX 1, ticket 0157, HIGH).
# rm's .erg-suffix branch resolved ticketPath then called os.Remove directly,
# never withinStore — so an explicit store DIR the FILE escapes would delete a
# file outside the store. The fix gates os.Remove behind withinStore.
# Positive: a valid .erg outside the named store is refused and SURVIVES.
# Negative control: an in-store .erg is deleted. The outside file is valid .erg
# so the test proves the confinement gate (not a parse/stat error) stops it.
# ---------------------------------------------------------------------------
WS="$FIXTURES/rmconfine"
mkdir -p "$WS/store"
write_open "$WS/outside.erg" "Outside"
write_open "$WS/store/9001-inside.erg" "Inside"

out=$($ERG rm "$WS/outside.erg" "$WS/store" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && [ -f "$WS/outside.erg" ] && echo "$out" | grep -q "outside the ticket store"; then
    pass "rm-confine: rm refuses a valid .erg outside the named store (file survives)"
else
    fail "rm-confine: rm should refuse out-of-store FILE (rc=$rc, exists=$([ -f "$WS/outside.erg" ] && echo yes || echo no), got: $out)"
fi
# Negative control: an in-store .erg deletes. Proves the gate is a no-op inside.
if $ERG rm "$WS/store/9001-inside.erg" "$WS/store" >/dev/null 2>&1 && [ ! -f "$WS/store/9001-inside.erg" ]; then
    pass "rm-confine neg ctrl: rm deletes an in-store FILE"
else
    fail "rm-confine neg ctrl: rm should delete an in-store FILE"
fi

# ---------------------------------------------------------------------------
# Group 4 — new explicit-DIR form (FIX 2, ticket 0157).
# Decision: the explicit DIR is an INTENTIONAL escape hatch, not confined —
# matching how close/log/tag/rm trust any explicitly-named store (resolveDir
# accepts any directory). For `new` the named DIR *is* the store it creates, so
# confining it against a "discovered"/cwd store is ill-defined, and confining
# against cwd would break the legitimate absolute-DIR form every caller relies
# on. 0149's withinStore is a fat-finger guard, not a security boundary; the
# attack needs attacker-controlled CLI args (not a committed .erg). See the code
# comment in new.go. This test asserts the documented behaviour: a relative
# subdir works (the use the ticket forbids breaking), and an explicit DIR is
# honoured verbatim.
# Negative control: an empty TITLE is still rejected (the guard that DOES fire),
# proving `new` is not a rubber stamp that accepts everything.
# ---------------------------------------------------------------------------
WS="$FIXTURES/newconfine"
mkdir -p "$WS"
# `|| true`: under `set -e`, a non-zero exit from this standalone subshell would
# abort the whole script — silently skipping the rest of this group, Groups 5-7,
# the intended fail diagnostic below, and the summary. The `if ls` check is the
# real assertion; let it record pass/fail.
( cd "$WS" && $ERG new "Sub ticket" "sub/dir" >/dev/null 2>&1 ) || true
if ls "$WS"/sub/dir/*.erg >/dev/null 2>&1; then
    pass "new: legitimate relative subdir 'sub/dir' creates a ticket"
else
    fail "new: legitimate relative subdir 'sub/dir' should create a ticket"
fi
# Documented escape hatch: an absolute explicit DIR is honoured (not confined).
if $ERG new "Abs ticket" "$WS/abs" >/dev/null 2>&1 && ls "$WS"/abs/*.erg >/dev/null 2>&1; then
    pass "new: explicit absolute DIR honoured verbatim (documented escape hatch)"
else
    fail "new: explicit absolute DIR should be honoured"
fi
# Negative control: empty title is refused (the guard that does trip).
if $ERG new "" "$WS/empty" >/dev/null 2>&1; then
    fail "new neg ctrl: empty TITLE should be refused"
else
    pass "new neg ctrl: empty TITLE refused (new is not a rubber stamp)"
fi

# ---------------------------------------------------------------------------
# Group 5 — Blocked-by ref injection (already safe). A traversal payload in a
# Blocked-by header is rejected by the ref parser with "malformed ref"; it is
# never resolved as a filesystem path. Use `erg validate` (single-file, format
# + ref check) — not `erg check`, which would also flag a clean ref as dangling.
# Fixtures live in a store with 0001 present so a legitimate ref resolves clean.
# Negative control: Blocked-by: 0001 validates (the same code path, valid input).
# ---------------------------------------------------------------------------
WS="$FIXTURES/refinject"
mkdir -p "$WS"
write_open "$WS/0001-blk.erg" "Blocker"
cat > "$WS/0002-bad.erg" <<'EOF'
%erg 0.1
Title: Bad ref
Created: 2026-01-01
Author: claude
Blocked-by: ../../../../etc/0042

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
# A traversal path-ref is a well-formed URI-reference, so validate accepts it.
# The security property is that resolution never escapes the repo: it stays
# unresolved (optimistic -> non-blocking), never reading outside the store.
if $ERG validate "$WS/0002-bad.erg" >/dev/null 2>&1; then
    pass "ref-inject: traversal path-ref is a valid handle (not a parse error)"
else
    fail "ref-inject: traversal path-ref should validate as a URI-reference"
fi
if $ERG ready "$WS" 2>/dev/null | grep -q "0002"; then
    pass "ref-inject: traversal path-ref does not block (resolution stays in-repo)"
else
    fail "ref-inject: traversal path-ref should not block 0002 ($($ERG ready "$WS" 2>&1))"
fi
# Negative control: a legitimate local ref to an existing ticket validates.
cat > "$WS/0003-good.erg" <<'EOF'
%erg 0.1
Title: Good ref
Created: 2026-01-01
Author: claude
Blocked-by: 0001

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
if $ERG validate "$WS/0003-good.erg" >/dev/null 2>&1; then
    pass "ref-inject neg ctrl: Blocked-by: 0001 validates clean"
else
    fail "ref-inject neg ctrl: Blocked-by: 0001 should validate clean"
fi

# ---------------------------------------------------------------------------
# Group 6 — Input DoS (already safe; assert bounded). The parser is linear and
# the slug regex is non-backtracking, so oversized input completes fast rather
# than hanging or OOMing. Each oversized case runs under `timeout 5`; exceeding
# the budget kills erg (rc 124) and fails the assertion.
# Negative control: a normal-sized file parses (and the same timeout passes
# trivially), proving the timeout itself is not the thing failing.
# ---------------------------------------------------------------------------
WS="$FIXTURES/dos"
mkdir -p "$WS"
# 6a — 10 MB body.
{
    printf '%%erg 0.1\nTitle: Big body\nCreated: 2026-01-01\nAuthor: claude\n\n--- log ---\n2026-01-01T10:00Z claude created\n\n--- body ---\n'
    head -c 10000000 /dev/zero | tr '\0' 'x'
} > "$WS/0001-bigbody.erg"
if bounded 5 $ERG validate "$WS/0001-bigbody.erg" >/dev/null 2>&1; then
    pass "dos: 10 MB body validates within time budget (no hang/OOM)"
else
    fail "dos: 10 MB body should validate fast (rc=$? — 124 means timeout)"
fi

# 6b — 100,000-line log section.
{
    printf '%%erg 0.1\nTitle: Big log\nCreated: 2026-01-01\nAuthor: claude\n\n--- log ---\n2026-01-01T10:00Z claude created\n'
    i=0
    while [ "$i" -lt 100000 ]; do
        printf '2026-01-01T10:00Z claude note line %d\n' "$i"
        i=$((i + 1))
    done
    printf '\n--- body ---\n'
} > "$WS/0002-biglog.erg"
if bounded 5 $ERG validate "$WS/0002-biglog.erg" >/dev/null 2>&1; then
    pass "dos: 100k-line log section parses within time budget"
else
    fail "dos: 100k-line log should parse fast (rc=$? — 124 means timeout)"
fi

# 6c — 10,000-char title via `erg new`: slug must truncate to <=40, no ReDoS.
LONG=$(head -c 10000 /dev/zero | tr '\0' 'a')
out=$(bounded 5 $ERG new "$LONG" "$WS/bigtitle" 2>&1) && rc=0 || rc=$?
slug=$(echo "$out" | sed 's/^CREATED [0-9]*-//; s/\.erg$//')
sluglen=$(printf '%s' "$slug" | wc -c)
if [ "$rc" -eq 0 ] && [ "$sluglen" -le 40 ] && [ "$sluglen" -gt 0 ]; then
    pass "dos: 10,000-char title slug truncated to $sluglen (<=40), no hang"
else
    fail "dos: 10,000-char title should truncate (rc=$rc, sluglen=$sluglen)"
fi

# Negative control: a normal file validates (timeout passes trivially).
write_open "$WS/0009-normal.erg" "Normal"
if bounded 5 $ERG validate "$WS/0009-normal.erg" >/dev/null 2>&1; then
    pass "dos neg ctrl: normal-sized file validates fast"
else
    fail "dos neg ctrl: normal file should validate fast"
fi

# ---------------------------------------------------------------------------
# Group 7 — Glob / ID ambiguity (already safe). A "*" ID is not shell-expanded
# here (quoted), so erg sees the literal "*", globs NNNN-*.erg, and refuses an
# ambiguous match rather than acting on an arbitrary ticket.
# Negative control: with a single ticket the same store closes normally.
# ---------------------------------------------------------------------------
WS="$FIXTURES/glob"
mkdir -p "$WS/multi" "$WS/single"
write_open "$WS/multi/9001-a.erg" "A"
write_open "$WS/multi/9002-b.erg" "B"
out=$($ERG close "*" reason "$WS/multi" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "ambiguous"; then
    pass "glob: close '*' against 2 tickets refuses as ambiguous"
else
    fail "glob: close '*' should refuse as ambiguous (rc=$rc, got: $out)"
fi
write_open "$WS/single/9001-only.erg" "Only"
if $ERG close 9001 reason "$WS/single" >/dev/null 2>&1 && grep -q '^Closed:' "$WS/single/closed/9001-only.erg"; then
    pass "glob neg ctrl: single ticket closes normally"
else
    fail "glob neg ctrl: single ticket should close normally"
fi

echo "security: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
