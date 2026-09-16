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
PASS=0; FAIL=0; SKIP=0; WARN=0
pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }
skip() { SKIP=$((SKIP + 1)); echo "  SKIP: $1"; }
warn() { WARN=$((WARN + 1)); echo "  WARN: $1"; }

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
# Helpers for the offline negative control below. Defined at top level, beside
# pass/fail/skip, rather than nested inside the guard that uses them.
neg_offline_module() {  # $1 = module directory to create
    mkdir -p "$1"
    printf 'module negoffline\ngo 1.21\n' > "$1/go.mod"
    printf 'package main\nimport _ "net/http"\nfunc main() {}\n' > "$1/main.go"
}
# `-buildvcs=false` is load-bearing, not tidying (0287). Go's buildvcs walks
# *upward* from the module directory hunting for a VCS root; an empty `.git`
# directory anywhere above $TMPDIR — hosts grow them — makes git exit 128,
# `go list` print nothing, and this control announce a blind detector. The flag
# suppresses VCS *stamping* only; import resolution is untouched, so the control
# keeps its teeth. Arm 2 below proves both halves of that claim.
neg_offline_detects() {  # $1 = module directory; true when net/http is seen
    (cd "$1" && go list -buildvcs=false -deps . 2>/dev/null) | grep -qE '^net/http$'
}
# The same probe with the flag taken away — used only to confirm the adversarial
# condition really bites before arm 2 credits the flag for surviving it.
neg_offline_detects_unflagged() {  # $1 = module directory
    (cd "$1" && go list -deps . 2>/dev/null) | grep -qE '^net/http$'
}
# Arm 2's environment. Go can be handed `-buildvcs=false` through three doors,
# and a test that leaves two of them open is measuring the machine rather than
# the code: the explicit argument above, an exported `GOFLAGS`, and `go env -w`,
# which an empty-but-set `GOFLAGS` silently falls through to. Someone hitting
# this very bug would plausibly set either of the latter two as a workaround —
# and then arm 2 passes, and keeps passing after the fix is reverted. Shut both
# ambient doors so the explicit argument is the only flag in play.
neg_offline_pin_env() {
    GOFLAGS=
    GOENV=off
    export GOFLAGS GOENV
}

if [ "$DEPS_OK" = yes ]; then
    if printf '%s\n' "$DEPS" | grep -qE '^net($|/)'; then
        NETPKGS=$(printf '%s\n' "$DEPS" | grep -E '^net($|/)' | tr '\n' ' ')
        fail "offline: a net* package is reachable from the binary: $NETPKGS"
    else
        pass "offline: no net / net-* package in the dependency graph"
    fi
    # Negative control, arm 1 — a throwaway package importing net/http must be
    # flagged by the very same go-list check, under the ambient environment,
    # whatever that happens to be. Proves the detector has teeth (offline build).
    NEG="$WORK/neg-offline"
    neg_offline_module "$NEG"
    if neg_offline_detects "$NEG"; then
        pass "offline (neg control): go-list check detects an injected net/http import"
    else
        fail "offline (neg control): detector failed to flag net/http"
    fi

    # Arm 2 — the same assertion with a stray VCS directory above the build dir,
    # manufactured here rather than borrowed from whatever the host carries, so
    # the regression is caught on any machine (0287).
    NEG_STRAY="$WORK/neg-offline-stray"
    mkdir -p "$NEG_STRAY/.git"   # an empty directory, not a repository
    neg_offline_module "$NEG_STRAY/mod"
    # Red control first. buildvcs only trips when Go can actually shell out to
    # git — with no git on PATH it skips stamping silently — so without this the
    # arm would go green with or without the flag, an all-clear indistinguishable
    # from "I could not look", which is the shape this suite exists to refuse.
    # Prove the unflagged probe really is blinded before crediting the flagged
    # one, and name no cause: the point is that the arm was not exercised, and
    # guessing why in the message is how a skip starts lying.
    # Both calls run under neg_offline_pin_env, in a subshell so the pinning does
    # not leak to arm 1 or to the rest of the suite. They call the very same
    # detector arm 1 does, so stripping the flag there is caught here.
    if ( neg_offline_pin_env; neg_offline_detects_unflagged "$NEG_STRAY/mod" ); then
        skip "offline (neg control): stray .git did not blind an unflagged go list — arm not exercised"
    elif ( neg_offline_pin_env; neg_offline_detects "$NEG_STRAY/mod" ); then
        pass "offline (neg control): detects net/http despite a stray .git above the build dir"
    else
        fail "offline (neg control): a stray .git above the build dir blinded the detector"
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

if [ "$CHECK_RC" -ne 0 ]; then
    fail "fast (wall-clock): erg check on $FAST_N tickets exited $CHECK_RC"
elif [ "$ELAPSED" -le "$FAST_CEILING" ]; then
    pass "fast (wall-clock): erg check on $FAST_N tickets completed in ${ELAPSED}s ≤ ${FAST_CEILING}s ceiling"
else
    # Non-blocking: a slow CI box must not break the suite. Warn, don't fail.
    # If this triggers, RAISE the ceiling — do not delete this test.
    warn "fast (wall-clock): erg check on $FAST_N tickets took ${ELAPSED}s — exceeds ${FAST_CEILING}s ceiling (raise ceiling, do not delete)"
fi

echo "contract: $PASS passed, $FAIL failed, $WARN warned, $SKIP skipped"
[ "$FAIL" -eq 0 ]
