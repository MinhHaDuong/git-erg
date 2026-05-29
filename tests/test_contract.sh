#!/bin/sh
# Contract guardrail tests — the design-contract invariants from AGENTS.md (0146).
#
# The project rests on six claims: agnostic, offline, standalone, stateless,
# fast, small. This suite guards the deterministic five; each guard is
# *falsifiable* — it ships with a negative control that proves the check
# actually trips when the invariant is violated.
#
# `fast` (parse-once + linear-vs-quadratic op-counts) needs loader
# instrumentation and is tracked separately — see the ticket noted in
# tickets/ — so it is intentionally absent here.
#
# The suite obeys the contract it tests: POSIX shell + the Go toolchain only,
# runs fully offline, no third-party test deps. ldd / dynamic-namespace bits
# are used only when present (graceful skip).
set -eu

ERG="${ERG_BIN:-build/erg}"
ERG_ABS=$(readlink -f "$ERG")
SRC=src/go
PASS=0; FAIL=0; SKIP=0
pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }
skip() { SKIP=$((SKIP + 1)); echo "  SKIP: $1"; }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

HAVE_GO=$(command -v go >/dev/null 2>&1 && echo yes || echo no)

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
if "$ERG_ABS" list "$STORE1" 2>/dev/null | grep -qF "$POSIX_TITLE"; then
    pass "agnostic: grep/awk-extracted Title agrees with the binary's parse"
else
    fail "agnostic: POSIX-extracted field disagrees with the binary's parse"
fi
# Negative control: a string the file never contained must NOT surface in the
# parse — proves the agreement check above is not vacuous.
if "$ERG_ABS" list "$STORE1" 2>/dev/null | grep -qF "a title that was never written"; then
    fail "agnostic (neg control): binary reported a field the file never had"
else
    pass "agnostic (neg control): binary reports only fields present in the file"
fi

# --- 2. offline: no networking anywhere (0148 removed the last exception) ----
if [ "$HAVE_GO" = yes ]; then
    DEPS=$(cd "$SRC" && go list -deps . 2>/dev/null)
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
else
    skip "offline: Go toolchain absent — dependency-graph check skipped"
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
if [ "$HAVE_GO" = yes ]; then
    THIRD=$(printf '%s\n' "$DEPS" | grep -E '^[^/]+\.[^/]+/' || true)
    if [ -z "$THIRD" ]; then
        pass "standalone: every dependency is a stdlib package"
    else
        fail "standalone: third-party packages in the graph: $(printf '%s' "$THIRD" | tr '\n' ' ')"
    fi
else
    skip "standalone: Go toolchain absent — dependency-graph check skipped"
fi

# Static link: Linux ldd ⇒ "not a dynamic executable" (skip if ldd absent).
if command -v ldd >/dev/null 2>&1; then
    if ldd "$ERG_ABS" 2>&1 | grep -q "not a dynamic executable"; then
        pass "standalone: the binary is statically linked (ldd)"
    else
        fail "standalone: binary is dynamically linked: $(ldd "$ERG_ABS" 2>&1 | head -1)"
    fi
    # Negative control: find any genuinely dynamic binary and assert ldd does NOT
    # call it static — proves the static check distinguishes the two.
    DYN=""
    for c in /bin/sh /usr/bin/file /usr/bin/ldd /bin/cat /bin/ls; do
        [ -e "$c" ] || continue
        if ldd "$c" 2>&1 | grep -q '=>'; then DYN="$c"; break; fi
    done
    if [ -n "$DYN" ]; then
        if ldd "$DYN" 2>&1 | grep -q "not a dynamic executable"; then
            fail "standalone (neg control): ldd called the dynamic binary $DYN static"
        else
            pass "standalone (neg control): ldd correctly sees $DYN as dynamic"
        fi
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
snapshot()  { find "$FAKEHOME" "$RUNCWD" -type f 2>/dev/null | sort; }

BEFORE_STORE=$(store_sum); BEFORE_FS=$(snapshot)
# Run every read command from a throwaway cwd with a throwaway HOME.
( cd "$RUNCWD" && HOME="$FAKEHOME" "$ERG_ABS" list    "$STORE4"          >/dev/null 2>&1 ) || true
( cd "$RUNCWD" && HOME="$FAKEHOME" "$ERG_ABS" ready   "$STORE4"          >/dev/null 2>&1 ) || true
( cd "$RUNCWD" && HOME="$FAKEHOME" "$ERG_ABS" check   "$STORE4"          >/dev/null 2>&1 ) || true
( cd "$RUNCWD" && HOME="$FAKEHOME" "$ERG_ABS" validate "$STORE4/0001-a.erg" >/dev/null 2>&1 ) || true
( cd "$RUNCWD" && HOME="$FAKEHOME" "$ERG_ABS" next-id "$STORE4"          >/dev/null 2>&1 ) || true
AFTER_STORE=$(store_sum); AFTER_FS=$(snapshot)

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
# Negative control: the snapshot mechanism must catch a real out-of-store write.
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
SIZE=$(wc -c < "$ERG_ABS")
if [ "$SIZE" -le "$CEILING" ]; then
    pass "small: binary is $SIZE bytes ≤ $CEILING (5 MB) ceiling"
else
    fail "small: binary is $SIZE bytes — exceeds the $CEILING (5 MB) ceiling"
fi
# zero-dep is the root cause of small — already asserted under standalone above.
# Negative control: the size comparison must reject a value above the ceiling.
if [ "$((SIZE + CEILING))" -le "$CEILING" ]; then
    fail "small (neg control): size comparison accepted an over-ceiling value"
else
    pass "small (neg control): size comparison rejects an over-ceiling value"
fi

echo "contract: $PASS passed, $FAIL failed, $SKIP skipped"
[ "$FAIL" -eq 0 ]
