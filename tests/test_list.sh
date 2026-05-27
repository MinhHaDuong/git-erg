#!/bin/sh
# Integration tests for: erg list (and the `ls` alias)
set -eu

ERG="${ERG_BIN:-build/erg}"
FIXTURES=$(mktemp -d)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

cleanup() { rm -rf "$FIXTURES"; }
trap cleanup EXIT

mkdir -p "$FIXTURES/store/closed"

echo "=== erg list ==="

# Open ticket, no blockers.
cat > "$FIXTURES/store/0001-open.erg" <<'EOF'
%erg 0.1
Title: An open ticket
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF

# Open, tagged, and blocked by an open local ticket.
cat > "$FIXTURES/store/0002-blocked.erg" <<'EOF'
%erg 0.1
Title: A blocked ticket
Created: 2026-01-01
Author: a
Tag: needs-human
Blocked-by: 0001

--- log ---
--- body ---
EOF

# Closed via Closed: header.
cat > "$FIXTURES/store/0003-done.erg" <<'EOF'
%erg 0.1
Title: A closed ticket
Created: 2026-01-01
Author: a
Closed: shipped

--- log ---
--- body ---
EOF

# Closed via closed/ path.
cat > "$FIXTURES/store/closed/0004-archived.erg" <<'EOF'
%erg 0.1
Title: An archived ticket
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF

# --- Open-only is the default: open tickets shown, closed hidden ---
output=$($ERG list "$FIXTURES/store")
if echo "$output" | grep -q "0001" && echo "$output" | grep -q "0002"; then
    pass "list: open tickets shown by default"
else
    fail "list: open tickets shown by default (output: $output)"
fi
if echo "$output" | grep -q "0003" || echo "$output" | grep -q "0004"; then
    fail "list: closed tickets hidden by default (output: $output)"
else
    pass "list: closed tickets hidden by default"
fi

# --- Blocked ticket appears in list (unlike ready) ---
if echo "$output" | grep -q "blocked-by: 0001"; then
    pass "list: shows blocked-by ref for blocked ticket"
else
    fail "list: shows blocked-by ref for blocked ticket (output: $output)"
fi

# --- Tags rendered ---
if echo "$output" | grep -q "tags: needs-human"; then
    pass "list: shows tags"
else
    fail "list: shows tags (output: $output)"
fi

# --- --all includes closed tickets ---
output=$($ERG list --all "$FIXTURES/store")
if echo "$output" | grep -q "0003" && echo "$output" | grep -q "0004"; then
    pass "list --all: closed tickets included"
else
    fail "list --all: closed tickets included (output: $output)"
fi
if echo "$output" | grep -q "\[closed\]"; then
    pass "list --all: closed tickets marked [closed]"
else
    fail "list --all: closed tickets marked [closed] (output: $output)"
fi

# --- Sorted by ID ascending ---
order=$($ERG list --all "$FIXTURES/store" | grep -oE '^  [0-9]{4}' | tr -d ' ')
expected=$(printf '0001\n0002\n0003\n0004')
if [ "$order" = "$expected" ]; then
    pass "list --all: sorted by ID ascending"
else
    fail "list --all: sorted by ID ascending (got: $(echo "$order" | tr '\n' ' '))"
fi

# --- JSON: open-only by default, valid array, expected fields ---
output=$($ERG list --json "$FIXTURES/store")
if echo "$output" | jq -e 'type == "array"' >/dev/null 2>&1; then
    pass "list --json: emits a JSON array"
else
    fail "list --json: emits a JSON array (output: $output)"
fi
if echo "$output" | jq -e '[.[] | select(.id == "0002")] | .[0] | .title == "A blocked ticket" and .closed == false and (.tags | index("needs-human")) != null and .blocked_by[0].kind == "local" and .blocked_by[0].id == "0001"' >/dev/null 2>&1; then
    pass "list --json: entry has id, title, closed, tags, blocked_by"
else
    fail "list --json: entry has id, title, closed, tags, blocked_by (output: $output)"
fi
if echo "$output" | jq -e 'any(.[]; .closed == true)' >/dev/null 2>&1; then
    fail "list --json: closed tickets excluded by default (output: $output)"
else
    pass "list --json: closed tickets excluded by default"
fi

# --- JSON --all includes closed tickets ---
output=$($ERG list --json --all "$FIXTURES/store")
if echo "$output" | jq -e 'any(.[]; .id == "0004" and .closed == true)' >/dev/null 2>&1; then
    pass "list --json --all: closed ticket present with closed=true"
else
    fail "list --json --all: closed ticket present with closed=true (output: $output)"
fi

# --- tags is always an array (never null) for untagged tickets ---
if echo "$output" | jq -e '[.[] | select(.id == "0001")] | .[0].tags == []' >/dev/null 2>&1; then
    pass "list --json: untagged ticket has tags == []"
else
    fail "list --json: untagged ticket has tags == [] (output: $output)"
fi

# --- `ls` is an alias for `list` ---
if [ "$($ERG ls "$FIXTURES/store")" = "$($ERG list "$FIXTURES/store")" ]; then
    pass "ls: alias produces identical output to list"
else
    fail "ls: alias produces identical output to list"
fi

# --- `ls --help` resolves to list's help ---
if $ERG ls --help 2>/dev/null | grep -q "^## erg list"; then
    pass "ls --help: shows list help"
else
    fail "ls --help: shows list help"
fi

# --- Forge-blocked ticket (offline-unknown ref counts as blocked) ---
cat > "$FIXTURES/store/0005-forge-blocked.erg" <<'EOF'
%erg 0.1
Title: Blocked by a forge issue
Created: 2026-01-01
Author: a
Blocked-by: github.com/anthropics/claude-code#1234

--- log ---
--- body ---
EOF

# IDs are matched anchored to the line-start column (^  NNNN) so a ticket's
# "(blocked-by: NNNN)" suffix never counts as a hit for ticket NNNN.

# --- Positive tag filter: only tickets carrying the tag ---
output=$($ERG list needs-human "$FIXTURES/store")
if echo "$output" | grep -qE '^  0002' && ! echo "$output" | grep -qE '^  0001'; then
    pass "list TAG: keeps only tagged tickets"
else
    fail "list TAG: keeps only tagged tickets (output: $output)"
fi

# --- Negative tag filter: drops tickets carrying the tag ---
output=$($ERG list not needs-human "$FIXTURES/store")
if echo "$output" | grep -qE '^  0001' && ! echo "$output" | grep -qE '^  0002'; then
    pass "list not TAG: drops tagged tickets"
else
    fail "list not TAG: drops tagged tickets (output: $output)"
fi

# --- blocked pseudo-tag: local open blocker and forge ref both count ---
output=$($ERG list blocked "$FIXTURES/store")
if echo "$output" | grep -qE '^  0002' && echo "$output" | grep -qE '^  0005' && ! echo "$output" | grep -qE '^  0001'; then
    pass "list blocked: local + forge blockers included, unblocked excluded"
else
    fail "list blocked: local + forge blockers included, unblocked excluded (output: $output)"
fi

# --- not blocked: drops blocked tickets ---
output=$($ERG list not blocked "$FIXTURES/store")
if echo "$output" | grep -qE '^  0001' && ! echo "$output" | grep -qE '^  0002' && ! echo "$output" | grep -qE '^  0005'; then
    pass "list not blocked: drops blocked tickets"
else
    fail "list not blocked: drops blocked tickets (output: $output)"
fi

# --- closed pseudo-tag overrides the implicit open default ---
output=$($ERG list closed "$FIXTURES/store")
if echo "$output" | grep -qE '^  0003' && echo "$output" | grep -qE '^  0004' && ! echo "$output" | grep -qE '^  0001'; then
    pass "list closed: shows closed tickets only"
else
    fail "list closed: shows closed tickets only (output: $output)"
fi

# --- contradictory open + closed yields nothing ---
output=$($ERG list open closed "$FIXTURES/store")
if echo "$output" | grep -qi "no tickets found"; then
    pass "list open closed: empty (mutually exclusive)"
else
    fail "list open closed: empty (output: $output)"
fi

# --- conjunction of positive and negative terms ---
output=$($ERG list open not blocked "$FIXTURES/store")
if echo "$output" | grep -qE '^  0001' && ! echo "$output" | grep -qE '^  0002' && ! echo "$output" | grep -qE '^  0003'; then
    pass "list open not blocked: conjunction filters correctly"
else
    fail "list open not blocked: conjunction filters correctly (output: $output)"
fi

# --- trailing 'not' with no tag is an error ---
if $ERG list "$FIXTURES/store" not >/dev/null 2>&1; then
    fail "list: dangling 'not' should error"
else
    pass "list: dangling 'not' errors"
fi

# --- JSON respects filters ---
output=$($ERG list --json blocked "$FIXTURES/store")
if echo "$output" | jq -e 'all(.[]; .blocked_by | length > 0)' >/dev/null 2>&1 \
    && echo "$output" | jq -e 'any(.[]; .id == "0005")' >/dev/null 2>&1; then
    pass "list --json blocked: every entry has a blocker"
else
    fail "list --json blocked: every entry has a blocker (output: $output)"
fi

rm -f "$FIXTURES/store/0005-forge-blocked.erg"

# An absolute path to the binary, so the cd-based cases below still find it.
ERG_ABS=$ERG
case "$ERG_ABS" in /*) ;; *) ERG_ABS="$(pwd)/$ERG_ABS" ;; esac

# --- Bare existing-directory name resolves as the store (no slash needed) ---
out_slash=$($ERG list "$FIXTURES/store")
out_bare=$(cd "$FIXTURES" && "$ERG_ABS" list store)
if [ "$out_slash" = "$out_bare" ]; then
    pass "list: bare existing-dir name resolves as store"
else
    fail "list: bare existing-dir name resolves as store (slash: $out_slash) (bare: $out_bare)"
fi

# --- 'closed' stays a filter even from inside a store with a closed/ subdir ---
out=$(cd "$FIXTURES/store" && "$ERG_ABS" list closed)
if echo "$out" | grep -qE '^  0003' && echo "$out" | grep -qE '^  0004' && ! echo "$out" | grep -qE '^  0001'; then
    pass "list: 'closed' is a filter, not the closed/ directory"
else
    fail "list: 'closed' is a filter, not the closed/ directory (output: $out)"
fi

# --- Empty store handled ---
mkdir -p "$FIXTURES/empty"
output=$($ERG list "$FIXTURES/empty")
if echo "$output" | grep -qi "no open tickets"; then
    pass "list: empty store handled"
else
    fail "list: empty store handled (output: $output)"
fi

echo "list: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
