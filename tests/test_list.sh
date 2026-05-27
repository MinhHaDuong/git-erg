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
