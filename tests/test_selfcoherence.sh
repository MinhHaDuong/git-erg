#!/bin/sh
# Self-coherence guard (ticket 0213, charter 4d / blocker #3).
#
# The infeasible test was "deployed tickets/ == embedded src/go/assets/ byte for
# byte across the whole dir." The correct, weaker invariant is:
#   (i)  erg init in a CLEAN tree reproduces the embedded assets exactly; and
#   (ii) git-erg's own deployed copy matches the embedded source (it dogfoods
#        the defaults verbatim -- no local customization).
# (ii) is what `make regen-assets && git diff --exit-code tickets/` enforces;
# this test asserts the same equality so drift fails CI loudly.
set -eu

ERG="${ERG_BIN:-build/erg}"
ERG_ABS=$(CDPATH= cd "$(dirname "$ERG")" && pwd)/$(basename "$ERG")
# Repo root (this script lives in tests/), for the embedded source of truth.
ROOT=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg self-coherence ==="

# The 2 retained assets (the spec + integration guide are print-on-demand now).
ASSETS=".ergrc AGENTS.md"

# (i) erg init in a clean tmpdir reproduces the embedded assets byte for byte.
TDIR=$(mktemp -d)
trap 'rm -rf "$TDIR"' EXIT
mkdir -p "$TDIR/tickets"
cp "$ERG_ABS" "$TDIR/tickets/erg"
$ERG init "$TDIR" >/dev/null 2>&1
for a in $ASSETS; do
    if cmp -s "$TDIR/tickets/$a" "$ROOT/src/go/assets/$a"; then
        pass "clean init reproduces embedded $a"
    else
        fail "clean init: tickets/$a differs from src/go/assets/$a"
    fi
done

# (ii) git-erg's own deployed copy matches the embedded source (dogfood, no
# customization). This is the regen-clean invariant: if these drift, a
# maintainer edited src/go/assets/ without running `make regen-assets`.
for a in $ASSETS; do
    if [ -f "$ROOT/tickets/$a" ]; then
        if cmp -s "$ROOT/tickets/$a" "$ROOT/src/go/assets/$a"; then
            pass "deployed tickets/$a matches embedded (run 'make regen-assets' if this fails)"
        else
            fail "deployed tickets/$a drifted from src/go/assets/$a -- run 'make regen-assets'"
        fi
    else
        # Not deployed -> not covered (the guard only checks assets actually present).
        pass "tickets/$a not deployed by git-erg (skipped)"
    fi
done

# Negative control: the guard must FAIL if the deployed copy is desynchronised.
# Prove it on a throwaway copy so we never touch the real tree.
PROBE=$(mktemp -d)
cp "$ROOT/src/go/assets/AGENTS.md" "$PROBE/embedded"
cp "$ROOT/tickets/AGENTS.md" "$PROBE/deployed"
printf 'LOCAL DRIFT\n' >> "$PROBE/deployed"
if cmp -s "$PROBE/deployed" "$PROBE/embedded"; then
    fail "negative control: a drifted copy was NOT detected (cmp too lax)"
else
    pass "negative control: a drifted copy is detected by cmp"
fi
rm -rf "$PROBE"

echo ""
echo "selfcoherence: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
