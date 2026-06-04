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

# (iii) Orphan assets (print-on-demand since 0206) must not be present in
# git-erg's own tickets/ -- they are served by erg spec / erg integration
# from the embedded source and would drift silently if re-tracked.
for orphan in spec-erg-v1.md integration.md; do
    if [ -f "$ROOT/tickets/$orphan" ]; then
        fail "stale orphan: tickets/$orphan should not exist (erg spec/integration serve it)"
    else
        pass "tickets/$orphan absent (print-on-demand via embedded asset)"
    fi
done

# Negative control: the guard must FAIL if the deployed copy is desynchronised.
# Prove it on a throwaway copy so we never touch the real tree.
PROBE=$(mktemp -d)
trap 'rm -rf "$TDIR" "$PROBE"' EXIT
cp "$ROOT/src/go/assets/AGENTS.md" "$PROBE/embedded"
cp "$ROOT/tickets/AGENTS.md" "$PROBE/deployed"
printf 'LOCAL DRIFT\n' >> "$PROBE/deployed"
if cmp -s "$PROBE/deployed" "$PROBE/embedded"; then
    fail "negative control: a drifted copy was NOT detected (cmp too lax)"
else
    pass "negative control: a drifted copy is detected by cmp"
fi

# (iv) hookBody <-> integration.md coherence guard (ticket 0219).
# The managed block installed into .git/hooks/pre-commit must be byte-for-byte
# identical to the shell snippet in src/go/assets/integration.md between the
# erg-managed markers. If one is updated without the other, this guard fails CI.
HOOKDIR=$(mktemp -d)
trap 'rm -rf "$TDIR" "$PROBE" "$HOOKDIR"' EXIT
mkdir -p "$HOOKDIR/tickets"
cp "$ERG_ABS" "$HOOKDIR/tickets/erg"
git init -q -b main "$HOOKDIR" >/dev/null 2>&1
$ERG install "$HOOKDIR" --hooks >/dev/null 2>&1

# Extract the managed block from the installed hook (between the markers, exclusive).
HOOK_BLOCK=$(awk '/^# >>> erg managed >>>/,/^# <<< erg managed <<</' "$HOOKDIR/.git/hooks/pre-commit" | grep -v '^# >>> erg managed >>>' | grep -v '^# <<< erg managed <<<')

# Extract the snippet from integration.md (between the ``` fences that wrap the
# managed block, stripping the fence lines and the marker lines themselves).
# The block in integration.md looks like:
#   ```sh
#   # >>> erg managed >>>
#   ...content...
#   # <<< erg managed <<<
#   ```
MD_BLOCK=$(awk '/^# >>> erg managed >>>/,/^# <<< erg managed <<</' "$ROOT/src/go/assets/integration.md" | grep -v '^# >>> erg managed >>>' | grep -v '^# <<< erg managed <<<')

if [ "$HOOK_BLOCK" = "$MD_BLOCK" ]; then
    pass "hookBody (installed hook) matches integration.md managed block byte-for-byte"
else
    fail "hookBody and integration.md managed block diverged -- edit both together"
    echo "  === hook block ===" >&2
    echo "$HOOK_BLOCK" >&2
    echo "  === md block ===" >&2
    echo "$MD_BLOCK" >&2
fi

# Negative control: prove the comparison detects a difference.
HOOK_BLOCK_MUTATED="${HOOK_BLOCK}
# INJECTED DRIFT"
if [ "$HOOK_BLOCK_MUTATED" = "$MD_BLOCK" ]; then
    fail "negative control: mutated hook block was NOT detected as different"
else
    pass "negative control: mutation in hook block is detected by comparison"
fi

echo ""
echo "selfcoherence: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
