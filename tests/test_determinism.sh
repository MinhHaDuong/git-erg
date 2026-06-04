#!/bin/sh
# Determinism guard: erg is single-threaded by design and its output ordering is
# deterministic (PEP section 10). Any future parallelism must be gated behind an
# explicit --jobs flag defaulting to 1. This suite enforces both halves of that
# contract -- with negative controls that prove each check can fail, matching the
# 0146/0160/0167 house style.
#
# Layer 1 -- goroutine grep ratchet: zero goroutine-launch statements (a line
# whose first token is the `go` keyword) in src/go non-test code. Zero today, so
# the ratchet starts clean; it trips the moment someone adds concurrency without
# the --jobs gate.
#
# Layer 2 -- output determinism probe: a hand-written fixture corpus (fixed IDs,
# varied Blocked-by edges) is built in ONE store dir, then check/list/ready are
# captured. The same dir is rebuilt from scratch in REVERSED file-write order and
# captured again; the two captures are byte-compared. A THIRD capture is taken on
# the unchanged corpus (rerun stability). All comparisons must be byte-identical.
# IDs are written by hand (not via `erg new`, whose IDs are creation-order
# dependent) so that file-write order is the ONLY variable. Using one store dir
# throughout also keeps any path that might leak into output constant.
set -eu

ERG="${ERG_BIN:-build/erg}"

PASS=0; FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== determinism (single-threaded, byte-stable output; ticket 0226) ==="

# --- Layer 1: goroutine grep ratchet -----------------------------------------
# A line whose first non-space token is `go ` launches a goroutine. Exclude
# *_test.go (test scaffolding may legitimately use concurrency). grep exits 1 on
# no-match, which is the PASS case under set -e, so guard the assignment.
GO_HITS=$(grep -Pn '^\s*go\s' src/go/*.go 2>/dev/null | grep -v '_test.go' || true)
if [ -z "$GO_HITS" ]; then
    pass "no goroutine-launch statements in src/go non-test code"
else
    fail "goroutine launch found in src/go non-test code: $GO_HITS"
fi

# Negative control: plant a temp .go file under src/go/ containing a goroutine
# launch and assert the ratchet trips, then remove it. Proves the grep has teeth.
GOROUTINE_NEGCTRL=$(mktemp src/go/test_determinism_negctrl_XXXXXX.go)
trap 'rm -f "$GOROUTINE_NEGCTRL"' EXIT
cat > "$GOROUTINE_NEGCTRL" <<'EOF'
package main

func negctrl() {
	go func() {}()
}
EOF
NEG_GO_HITS=$(grep -Pn '^\s*go\s' src/go/*.go 2>/dev/null | grep -v '_test.go' || true)
if [ -n "$NEG_GO_HITS" ]; then
    pass "goroutine ratchet (neg control): planted goroutine launch detected"
else
    fail "goroutine ratchet (neg control): planted goroutine launch NOT detected -- ratchet is vacuous"
fi
rm -f "$GOROUTINE_NEGCTRL"
trap - EXIT

# --- Layer 2: output determinism probe ----------------------------------------
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
STORE="$WORK/store"

# write_ticket ID TITLE [BLOCKED_BY]: write one fixed-ID .erg file into $STORE.
# Hand-written (not `erg new`) so the ID is decoupled from creation order.
write_ticket() {
    _id="$1"; _title="$2"; _blocked="${3:-}"
    _f="$STORE/$_id-fixture.erg"
    {
        printf '%%erg 0.1\n'
        printf 'Title: %s\n' "$_title"
        printf 'Created: 2026-06-04\n'
        printf 'Author: tester\n'
        [ -n "$_blocked" ] && printf 'Blocked-by: %s\n' "$_blocked"
        printf '\n--- log ---\n'
        printf '2026-06-04T09:00Z tester created\n'
        printf '\n--- body ---\n'
        printf 'Fixture body for %s.\n' "$_id"
    } > "$_f"
}

# build_corpus ORDER: rebuild $STORE from scratch. ORDER is "forward" or
# "reverse" and controls the order in which the SAME ~20 ticket files are
# written to disk. The set of files, their IDs, titles, and Blocked-by edges are
# identical in both orders; only the filesystem write sequence differs.
build_corpus() {
    _order="$1"
    rm -rf "$STORE"
    mkdir -p "$STORE"
    # 20 tickets, fixed IDs 0001-0020, with a spread of Blocked-by edges
    # (chains, fan-in, fan-out) so check/list/ready exercise dependency logic.
    # Format: "ID|TITLE|BLOCKED_BY" (empty BLOCKED_BY allowed).
    _specs="0001|Alpha foundation|
0002|Beta layer|0001
0003|Gamma layer|0001
0004|Delta probe|0002
0005|Epsilon probe|0002 0003
0006|Zeta independent|
0007|Eta chain|0006
0008|Theta chain|0007
0009|Iota fanin|0004 0005
0010|Kappa independent|
0011|Lambda probe|0010
0012|Mu probe|0010
0013|Nu chain|0011
0014|Xi fanout|0001
0015|Omicron fanout|0001
0016|Pi independent|
0017|Rho chain|0016
0018|Sigma fanin|0014 0015
0019|Tau probe|0013
0020|Upsilon capstone|0009 0018"

    if [ "$_order" = "reverse" ]; then
        # POSIX-portable line reversal (tac is not in POSIX).
        _specs=$(printf '%s\n' "$_specs" | awk '{a[NR]=$0} END{for(i=NR;i>=1;i--)print a[i]}')
    fi

    printf '%s\n' "$_specs" | while IFS='|' read -r _id _title _blocked; do
        [ -z "$_id" ] && continue
        write_ticket "$_id" "$_title" "$_blocked"
    done
}

# capture STORE: run check/list/ready and concatenate their output (with exit
# codes) into one file. erg outputs IDs not absolute paths, and we reuse one
# store dir, so nothing path-dependent leaks in.
capture() {
    _out="$1"
    {
        echo "--- check ---"
        "$ERG" check "$STORE" 2>&1 || echo "rc=$?"
        echo "--- list ---"
        "$ERG" list "$STORE" 2>&1 || echo "rc=$?"
        echo "--- ready ---"
        "$ERG" ready "$STORE" 2>&1 || echo "rc=$?"
    } > "$_out"
}

CAP_FWD="$WORK/cap_forward"
CAP_FWD2="$WORK/cap_forward_rerun"
CAP_REV="$WORK/cap_reverse"

build_corpus forward
capture "$CAP_FWD"
# Rerun on the UNCHANGED corpus: catches per-run nondeterminism (a future
# goroutine, Go map-iteration order, an unseeded RNG) even when the files on disk
# are byte-identical. This is the comparison with the most teeth.
capture "$CAP_FWD2"

build_corpus reverse
capture "$CAP_REV"

# Pair 1: rerun stability on the unchanged corpus.
if cmp -s "$CAP_FWD" "$CAP_FWD2"; then
    pass "output is byte-identical across two consecutive runs (rerun stability)"
else
    fail "output differs between two runs on the unchanged corpus -- nondeterministic"
fi

# Pair 2: creation-order independence (forward vs reverse write order).
if cmp -s "$CAP_FWD" "$CAP_REV"; then
    pass "output is byte-identical across forward/reverse file-write order"
else
    fail "output differs when file-write order changes -- order-dependent"
fi

# Pair 3: the rerun and the reversed build must also agree (transitive sanity).
if cmp -s "$CAP_FWD2" "$CAP_REV"; then
    pass "rerun and reversed-order captures agree"
else
    fail "rerun and reversed-order captures disagree -- nondeterministic"
fi

# Negative control: copy a capture, append a byte, assert cmp flags the
# difference. Proves the byte-comparison has teeth and is not vacuously passing.
CAP_TAMPERED="$WORK/cap_tampered"
cp "$CAP_FWD" "$CAP_TAMPERED"
printf 'X' >> "$CAP_TAMPERED"
if cmp -s "$CAP_FWD" "$CAP_TAMPERED"; then
    fail "byte-compare (neg control): appended byte NOT detected -- comparison is vacuous"
else
    pass "byte-compare (neg control): appended byte detected by cmp"
fi

rm -rf "$WORK"
trap - EXIT

echo "determinism: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
