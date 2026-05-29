#!/bin/sh
# Contract guardrail tests — the design-contract invariants from AGENTS.md.
#
# The project rests on six claims: agnostic, offline, standalone, stateless,
# fast, small. This suite guards all six; each guard is *falsifiable* — it
# ships with a negative control that proves the check actually trips when the
# invariant is violated.
#
# The `fast` invariant's deterministic guards (parse-once + linear-vs-quadratic
# op-counts) live in src/go/contract_test.go; this file hosts the wall-clock
# backstop (non-blocking, generous ceiling — raise don't delete).
#
# The suite obeys the contract it tests: POSIX shell + the Go toolchain only,
# runs fully offline, no third-party test deps. ldd / dynamic-namespace bits
# are used only when present (graceful skip).
set -eu

ERG="${ERG_BIN:-build/erg}"
# Resolve to an absolute path (the stateless guard runs the binary from a
# throwaway cwd). Prefer `readlink -f`, but fall back to a POSIX construction so
# a missing/non-GNU readlink reports clearly instead of silently aborting the
# suite under `set -e`.
ERG_ABS=$(readlink -f "$ERG" 2>/dev/null || true)
[ -n "$ERG_ABS" ] || ERG_ABS=$(cd "$(dirname "$ERG")" 2>/dev/null && pwd)/$(basename "$ERG")
SRC=src/go
PASS=0; FAIL=0; SKIP=0
pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }
skip() { SKIP=$((SKIP + 1)); echo "  SKIP: $1"; }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

HAVE_GO=$(command -v go >/dev/null 2>&1 && echo yes || echo no)
# Resolve the dependency graph once, capturing success separately: a failed
# `go list` (compile error, offline toolchain fetch) must surface as a FAIL,
# never as a silent abort or a vacuous "no net package" pass.
DEPS=""; DEPS_OK=no
if [ "$HAVE_GO" = yes ]; then
    if DEPS=$(cd "$SRC" && go list -deps . 2>/dev/null); then DEPS_OK=yes; fi
fi

echo "=== contract guardrails (design-contract invariants, 0146) ==="

# --- 1. agnostic: the file is the contract, the binary is optional ----------
STORE1="$WORK/agnostic"
mkdir -p "$STORE1"
# Hand-author a ticket with a heredoc — plain LF/UTF-8 text, no binary involved.
cat > "$STORE1/0001-hand-authored.erg" <<'EOF'
%erg 0.1
Title: Hand authored without the binary
Created: 2026-05-29
Author: human

--- log ---
2026-05-29T00:00Z human created

--- body ---
Written in a text editor; no erg binary required to produce a valid ticket.
EOF

if "$ERG_ABS" validate "$STORE1/0001-hand-authored.erg" >/dev/null 2>&1; then
    pass "agnostic: a hand-authored .erg validates (binary not needed to write one)"
else
    fail "agnostic: hand-authored .erg was rejected"
fi

# A POSIX-extracted header field must agree with the binary's own parse.
POSIX_TITLE=$(awk -F': ' '/^Title:/{print $2; exit}' "$STORE1/0001-hand-authored.erg")
if [ -n "$POSIX_TITLE" ] && "$ERG_ABS" list "$STORE1" 2>/dev/null | grep -qF "$POSIX_TITLE"; then
    pass "agnostic: grep/awk-extracted Title agrees with the binary's parse"
else
    fail "agnostic: POSIX-extracted field empty or disagrees with the binary's parse"
fi
# Negative control: a string the file never contained must NOT surface in the
# parse — proves the agreement check above is not vacuous.
if "$ERG_ABS" list "$STORE1" 2>/dev/null | grep -qF "a title that was never written"; then
    fail "agnostic (neg control): binary reported a field the file never had"
else
    pass "agnostic (neg control): binary reports only fields present in the file"
fi

# --- 2. offline: no networking anywhere (0148 removed the last exception) ----
if [ "$DEPS_OK" = yes ]; then
    if printf '%s\n' "$DEPS" | grep -qE '^net($|/)'; then
        NETPKGS=$(printf '%s\n' "$DEPS" | grep -E '^net($|/)' | tr '\n' ' ')
        fail "offline: a net* package is reachable from the binary: $NETPKGS"
    else
        pass "offline: no net / net-* package in the dependency graph"
    fi
    # Negative control: a throwaway package importing net/http must be flagged by
    # the very same go-list check — proves the detector has teeth (offline build).
    NEG="$WORK/neg-offline"
    mkdir -p "$NEG"
    printf 'module negoffline\ngo 1.21\n' > "$NEG/go.mod"
    printf 'package main\nimport _ "net/http"\nfunc main() {}\n' > "$NEG/main.go"
    if (cd "$NEG" && go list -deps . 2>/dev/null) | grep -qE '^net/http$'; then
        pass "offline (neg control): go-list check detects an injected net/http import"
    else
        fail "offline (neg control): detector failed to flag net/http"
    fi
elif [ "$HAVE_GO" = no ]; then
    skip "offline: Go toolchain absent — dependency-graph check skipped"
else
    fail "offline: 'go list -deps' failed — cannot verify the dependency graph"
fi

# Optional dynamic guard: a read command must succeed with the network dropped.
if command -v unshare >/dev/null 2>&1 && unshare -rn true >/dev/null 2>&1; then
    if unshare -rn "$ERG_ABS" list "$STORE1" >/dev/null 2>&1; then
        pass "offline (dynamic): list runs with the network namespace dropped"
    else
        fail "offline (dynamic): list failed with no network"
    fi
else
    skip "offline (dynamic): unshare -rn unavailable — namespace check skipped"
fi

# --- 3. standalone: zero third-party deps, single static binary -------------
if grep -qE '^require' "$SRC/go.mod"; then
    fail "standalone: go.mod declares a require (third-party dependency)"
else
    pass "standalone: go.mod is stdlib-only (no require block)"
fi
if [ "$DEPS_OK" = yes ]; then
    THIRD=$(printf '%s\n' "$DEPS" | grep -E '^[^/]+\.[^/]+/' || true)
    if [ -z "$THIRD" ]; then
        pass "standalone: every dependency is a stdlib package"
    else
        fail "standalone: third-party packages in the graph: $(printf '%s' "$THIRD" | tr '\n' ' ')"
    fi
elif [ "$HAVE_GO" = no ]; then
    skip "standalone: Go toolchain absent — dependency-graph check skipped"
else
    fail "standalone: 'go list -deps' failed — cannot verify the dependency graph"
fi

# Static link: a static binary has no dynamic library dependencies, so `ldd`
# prints no "=>" lines. Defining static as "no => in ldd output" is portable
# across glibc ("not a dynamic executable") and musl ("Not a valid dynamic
# program") — both of which the Makefile targets — and is the exact inverse of
# the dynamic-binary probe used in the negative control.
is_static() { ! ldd "$1" 2>&1 | grep -q '=>'; }
if command -v ldd >/dev/null 2>&1; then
    if is_static "$ERG_ABS"; then
        pass "standalone: the binary is statically linked (ldd lists no shared libs)"
    else
        fail "standalone: binary is dynamically linked: $(ldd "$ERG_ABS" 2>&1 | grep '=>' | head -1)"
    fi
    # Negative control: the same predicate must report *dynamic* for a genuinely
    # dynamic binary — proving it is not vacuously always-static.
    DYN=""
    for c in /bin/sh /usr/bin/file /usr/bin/ldd /bin/cat /bin/ls; do
        [ -e "$c" ] || continue
        if ! is_static "$c"; then DYN="$c"; break; fi
    done
    if [ -n "$DYN" ]; then
        pass "standalone (neg control): static predicate flags $DYN as dynamic"
    else
        skip "standalone (neg control): no dynamic probe binary found — skipped"
    fi
else
    skip "standalone: ldd absent — static-link check skipped"
fi

# --- 4. stateless: the files are the only state -----------------------------
STORE4="$WORK/stateless/tickets"
mkdir -p "$STORE4"
cat > "$STORE4/0001-a.erg" <<'EOF'
%erg 0.1
Title: First
Created: 2026-05-29
Author: a

--- log ---
2026-05-29T00:00Z a created

--- body ---
EOF
cat > "$STORE4/0002-b.erg" <<'EOF'
%erg 0.1
Title: Second
Created: 2026-05-29
Author: b
Blocked-by: 0001

--- log ---
2026-05-29T00:00Z b created

--- body ---
EOF
FAKEHOME="$WORK/stateless/home"; mkdir -p "$FAKEHOME"
RUNCWD="$WORK/stateless/cwd"; mkdir -p "$RUNCWD"

store_sum() { (cd "$STORE4" && for f in *.erg; do printf '%s ' "$f"; cksum < "$f"; done); }
# Capture *all* entries, not just regular files: an empty cache directory under
# HOME or cwd is a write outside the store too, and would slip past a -type f scan.
snapshot()  { find "$FAKEHOME" "$RUNCWD" 2>/dev/null | sort; }

# Run a read command from a throwaway cwd with a throwaway HOME; track exit codes
# so a crash can't pass silently behind an unchanged filesystem.
READ_RC=0
run_read() { ( cd "$RUNCWD" && HOME="$FAKEHOME" "$ERG_ABS" "$@" >/dev/null 2>&1 ); }

BEFORE_STORE=$(store_sum); BEFORE_FS=$(snapshot)
run_read list    "$STORE4"             || READ_RC=1
run_read ready   "$STORE4"             || READ_RC=1
run_read check   "$STORE4"             || READ_RC=1
run_read validate "$STORE4/0001-a.erg" || READ_RC=1
run_read next-id "$STORE4"             || READ_RC=1
AFTER_STORE=$(store_sum); AFTER_FS=$(snapshot)

if [ "$READ_RC" -eq 0 ]; then
    pass "stateless: read commands all exit 0 (no crash hiding behind a clean FS)"
else
    fail "stateless: a read command exited non-zero"
fi
if [ "$BEFORE_STORE" = "$AFTER_STORE" ]; then
    pass "stateless: read commands leave the store byte-identical"
else
    fail "stateless: a read command modified the store"
fi
if [ "$BEFORE_FS" = "$AFTER_FS" ]; then
    pass "stateless: read commands write nothing outside the store (HOME/cwd clean)"
else
    fail "stateless: a read command wrote outside the store"
fi
# Negative control: validates the snapshot *detector* — a read command can't be
# made to write out of store on demand, so plant a file and confirm the
# find-snapshot would have caught such a write.
touch "$FAKEHOME/.ergcache"
if [ "$BEFORE_FS" = "$(snapshot)" ]; then
    fail "stateless (neg control): snapshot missed a planted out-of-store file"
else
    pass "stateless (neg control): snapshot detects an out-of-store write"
fi
rm -f "$FAKEHOME/.ergcache"
# Idempotence + order-independence: list output must not depend on a prior command.
OUT_A=$(HOME="$FAKEHOME" "$ERG_ABS" list "$STORE4" 2>/dev/null)
HOME="$FAKEHOME" "$ERG_ABS" check "$STORE4" >/dev/null 2>&1 || true
OUT_B=$(HOME="$FAKEHOME" "$ERG_ABS" list "$STORE4" 2>/dev/null)
if [ "$OUT_A" = "$OUT_B" ]; then
    pass "stateless: list output is identical and independent of a prior command"
else
    fail "stateless: list output changed across invocations / depends on order"
fi

# --- 5. small: the committed binary stays near the Go runtime floor ---------
CEILING=$((5 * 1024 * 1024))    # 5 MB — ratcheted from 10 MB (AGENTS.md / 0146)
# size_within reports whether the file at $1 is within the ceiling — the actual
# measurement path, exercised by both the real check and its negative control.
size_within() { [ "$(wc -c < "$1")" -le "$CEILING" ]; }

if size_within "$ERG_ABS"; then
    pass "small: binary is $(wc -c < "$ERG_ABS") bytes ≤ $CEILING (5 MB) ceiling"
else
    fail "small: binary is $(wc -c < "$ERG_ABS") bytes — exceeds the $CEILING (5 MB) ceiling"
fi
# zero-dep is the root cause of small — already asserted under standalone above.
# Negative control: build a genuinely oversized artifact and run it through the
# SAME measurement — proves the guard trips on real bloat, not just arithmetic.
BLOAT="$WORK/bloat.bin"
head -c $((CEILING + 1)) /dev/zero > "$BLOAT" 2>/dev/null || dd if=/dev/zero of="$BLOAT" bs=1024 count=$(((CEILING / 1024) + 1)) >/dev/null 2>&1
if size_within "$BLOAT"; then
    fail "small (neg control): a $(wc -c < "$BLOAT")-byte file slipped under the ceiling"
else
    pass "small (neg control): size check rejects a real >5 MB artifact"
fi

# --- 6. fast (wall-clock backstop): generous absolute ceiling ---------------
# The parse-once and linear-scaling guards live in src/go/contract_test.go
# (deterministic, counter-based). This is the non-blocking wall-clock safety
# net: it trips only on catastrophic regression, not on a slow CI box.
# Rule: if it ever flakes, RAISE the ceiling — do not delete this test.
FAST_STORE="$WORK/fast-corpus"
mkdir -p "$FAST_STORE"
FAST_N=500
i=1
while [ "$i" -le "$FAST_N" ]; do
    ID=$(printf '%04d' "$i")
    cat > "$FAST_STORE/${ID}-synth-${ID}.erg" <<ERGEOF
%erg 0.1
Title: Synthetic ticket $ID
Created: 2026-01-01
Author: bench

--- log ---
2026-01-01T00:00Z bench created

--- body ---
Body text for synthetic ticket $ID.
ERGEOF
    i=$((i + 1))
done

FAST_CEILING=10   # seconds — ~10x headroom over typical <1s runtime
START_TS=$(date +%s)
"$ERG_ABS" check "$FAST_STORE" >/dev/null 2>&1; CHECK_RC=$?
END_TS=$(date +%s)
ELAPSED=$((END_TS - START_TS))

if [ "$CHECK_RC" -eq 0 ] && [ "$ELAPSED" -le "$FAST_CEILING" ]; then
    pass "fast (wall-clock): erg check on $FAST_N tickets completed in ${ELAPSED}s ≤ ${FAST_CEILING}s ceiling"
elif [ "$CHECK_RC" -ne 0 ]; then
    fail "fast (wall-clock): erg check on $FAST_N tickets exited $CHECK_RC"
else
    fail "fast (wall-clock): erg check on $FAST_N tickets took ${ELAPSED}s — exceeds ${FAST_CEILING}s ceiling (raise ceiling if this is a slow CI box, do not delete)"
fi

echo "contract: $PASS passed, $FAIL failed, $SKIP skipped"
[ "$FAIL" -eq 0 ]
