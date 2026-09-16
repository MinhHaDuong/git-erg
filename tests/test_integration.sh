#!/bin/sh
# Integration tests for: erg integration
set -eu

ERG="${ERG_BIN:-build/erg}"
# Repo root (this script lives in tests/), for the embedded asset sources.
ROOT=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg integration ==="

# --- integration prints the setup guide ---

out=$($ERG integration 2>/dev/null)
if echo "$out" | grep -q "pre-commit"; then
    pass "integration output mentions pre-commit"
else
    fail "integration output missing 'pre-commit' (got: $(echo "$out" | head -1))"
fi

# --- integration matches the embedded asset ---

first_line=$(echo "$out" | head -1)
if echo "$first_line" | grep -qi "integration\|setup\|hook"; then
    pass "integration first line looks like setup guide"
else
    fail "integration first line unexpected (got: $first_line)"
fi

# --- integration exits 0 ---

$ERG integration > /dev/null 2>&1 && rc=0 || rc=$?
if [ "$rc" -eq 0 ]; then
    pass "integration exits 0"
else
    fail "integration exits $rc"
fi

# --- integration help starts with ## erg integration ---

help_out=$($ERG integration --help 2>/dev/null)
if echo "$help_out" | grep -q "^## erg integration"; then
    pass "integration help starts with '## erg integration'"
else
    fail "integration help header (got: $(echo "$help_out" | head -1))"
fi

# --- ticket 0288: generic git-erg lore is served here, not resident ---
#
# The shipped tickets/AGENTS.md is resident context, re-read at every session
# start in every adopter repo. Long-form conventions belong in this on-demand
# channel instead. These assertions are on CONTENT, not exit code, and they run
# against the command's real output (cmdIntegration + bootstrapAsset), not
# against the asset file on disk.

if echo "$out" | grep -q "ID allocation is optimistic"; then
    pass "integration output carries the optimistic-ID allocation mechanism"
else
    fail "integration output missing the optimistic-ID allocation mechanism"
fi

if echo "$out" | grep -q "never to the next free ID"; then
    pass "integration output carries the renumber-clear-of-the-frontier policy"
else
    fail "integration output missing the renumber-clear-of-the-frontier policy"
fi

if echo "$out" | grep -qF 'gh pr list --json files'; then
    pass "integration output names the 'gh pr list --json files' trap"
else
    fail "integration output missing the 'gh pr list --json files' trap"
fi

if echo "$out" | grep -qF 'gh pr view' && echo "$out" | grep -qF '.files[].path'; then
    pass "integration output carries the per-PR 'gh pr view' enumeration recipe"
else
    fail "integration output missing the per-PR 'gh pr view' enumeration recipe"
fi

if echo "$out" | grep -qF 'erg check' && echo "$out" | grep -qF 'origin/main'; then
    pass "integration output carries the post-merge 'erg check origin/main' rule"
else
    fail "integration output missing the post-merge 'erg check origin/main' rule"
fi

if echo "$out" | grep -q "^## Handoff-document sections"; then
    pass "integration output carries the handoff-document section template"
else
    fail "integration output missing the handoff-document section template"
fi

# --- the shipped resident asset stays lean (ticket 0288, Action 3) ---
#
# A concrete ceiling, not an order of magnitude: 2141 bytes before this ticket,
# plus room for the pointer it adds. A later edit that creeps the resident file
# back up fails here.

AGENTS_CEILING=2400
agents_bytes=$(wc -c < "$ROOT/src/go/assets/AGENTS.md" | tr -d ' ')
if [ "$agents_bytes" -le "$AGENTS_CEILING" ]; then
    pass "src/go/assets/AGENTS.md is $agents_bytes bytes (ceiling $AGENTS_CEILING)"
else
    fail "src/go/assets/AGENTS.md is $agents_bytes bytes, over the $AGENTS_CEILING-byte ceiling"
fi

# Negative control: prove the comparison above can fail. A ceiling check that
# passes whatever the file holds is not a check.
if [ "$((AGENTS_CEILING + 1))" -le "$AGENTS_CEILING" ]; then
    fail "negative control: an over-ceiling size was NOT rejected"
else
    pass "negative control: an over-ceiling size is rejected"
fi

# --- bucket boundary: no adopter-specific lore in the shipped assets ---
#
# Without this, a wholesale copy of one adopter's edited AGENTS.md -- the exact
# antipattern 0288 exists to avoid -- would satisfy every positive assertion
# above. These names are one adopter's own CI wiring and incident history; they
# have no place in an asset shipped to every repo.

ADOPTER_STRINGS="climate-finance-het search-works-for-zotero scripts/check-cross-pr-ticket-collision.sh harness-extension-point erg-pr-merge voice-alignment-vision cross-pr-ticket-collision validate-tickets"
for f in "$ROOT/src/go/assets/integration.md" "$ROOT/src/go/assets/AGENTS.md"; do
    leaked=""
    for s in $ADOPTER_STRINGS; do
        if grep -qF "$s" "$f"; then
            leaked="$leaked $s"
        fi
    done
    if [ -z "$leaked" ]; then
        pass "$(basename "$f") carries no adopter-specific lore"
    else
        fail "$(basename "$f") leaks adopter-specific lore:$leaked"
    fi
done

# --- the declared home for adopter lore is genuinely erg's to ignore ---
#
# The asset above makes two factual claims about tickets/LOCAL.md: erg check
# ignores it like any other non-.erg file, and erg never rewrites or deletes it.
# Both are claims about behaviour, so pin them here rather than trust the prose.

ERG_ABS=$(CDPATH= cd "$(dirname "$ERG")" && pwd)/$(basename "$ERG")
LOREDIR=$(mktemp -d)
mkdir -p "$LOREDIR/tickets"
cp "$ERG_ABS" "$LOREDIR/tickets/erg"
$ERG init "$LOREDIR" >/dev/null 2>&1
printf '# local lore\nCI job: check-cross-pr\n' > "$LOREDIR/tickets/LOCAL.md"
if $ERG check "$LOREDIR/tickets" >/dev/null 2>&1; then
    pass "erg check ignores tickets/LOCAL.md"
else
    fail "erg check rejects a store holding tickets/LOCAL.md"
fi
$ERG init "$LOREDIR" >/dev/null 2>&1
if [ "$(cat "$LOREDIR/tickets/LOCAL.md")" = "$(printf '# local lore\nCI job: check-cross-pr')" ]; then
    pass "erg init leaves tickets/LOCAL.md byte-identical"
else
    fail "erg init rewrote tickets/LOCAL.md"
fi
# Negative control: the same comparison must notice a rewrite.
printf 'clobbered\n' > "$LOREDIR/tickets/LOCAL.md"
if [ "$(cat "$LOREDIR/tickets/LOCAL.md")" = "$(printf '# local lore\nCI job: check-cross-pr')" ]; then
    fail "negative control: a rewritten LOCAL.md was NOT detected"
else
    pass "negative control: a rewritten LOCAL.md is detected"
fi
rm -r "$LOREDIR"

# Negative control: prove the scan above detects a leak.
LEAKPROBE=$(mktemp)
trap 'rm -f "$LEAKPROBE"' EXIT
cp "$ROOT/src/go/assets/integration.md" "$LEAKPROBE"
printf 'see scripts/check-cross-pr-ticket-collision.sh\n' >> "$LEAKPROBE"
probe_leaked=""
for s in $ADOPTER_STRINGS; do
    if grep -qF "$s" "$LEAKPROBE"; then
        probe_leaked="$probe_leaked $s"
    fi
done
if [ -n "$probe_leaked" ]; then
    pass "negative control: a planted adopter-specific string is detected"
else
    fail "negative control: a planted adopter-specific string was NOT detected"
fi

# A literal blocklist only stops copy-paste. The realistic recurrence is one
# adopter's war story paraphrased into the shared guide, and those carry two
# shapes a literal list cannot enumerate: the date it happened and the ticket
# it happened in. Both are banned structurally in the on-demand guide. (Not in
# AGENTS.md: its worked example is a dated ticket, by design.)

if grep -qE '[0-9]{4}-[0-9]{2}-[0-9]{2}' "$ROOT/src/go/assets/integration.md"; then
    fail "integration.md carries a calendar date -- generic guidance has no incident dates"
else
    pass "integration.md carries no calendar date"
fi

if grep -qiE 'ticket +[0-9]{3,4}' "$ROOT/src/go/assets/integration.md"; then
    fail "integration.md cites a ticket number -- generic guidance names no adopter's tickets"
else
    pass "integration.md cites no ticket number"
fi

# Negative controls for both structural guards.
DATEPROBE=$(mktemp)
trap 'rm -f "$LEAKPROBE" "$DATEPROBE"' EXIT
cp "$ROOT/src/go/assets/integration.md" "$DATEPROBE"
printf 'seen in one repo on 2026-07-22, filed as ticket 0243\n' >> "$DATEPROBE"
if grep -qE '[0-9]{4}-[0-9]{2}-[0-9]{2}' "$DATEPROBE"; then
    pass "negative control: a planted incident date is detected"
else
    fail "negative control: a planted incident date was NOT detected"
fi
if grep -qiE 'ticket +[0-9]{3,4}' "$DATEPROBE"; then
    pass "negative control: a planted ticket citation is detected"
else
    fail "negative control: a planted ticket citation was NOT detected"
fi

echo ""
echo "integration: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
