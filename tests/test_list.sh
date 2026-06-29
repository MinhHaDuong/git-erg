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

# Open, labeled, and blocked by an open local ticket.
cat > "$FIXTURES/store/0002-blocked.erg" <<'EOF'
%erg 0.1
Title: A blocked ticket
Created: 2026-01-01
Author: a
Label: needs-human
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

# --- Labels rendered ---
if echo "$output" | grep -q "labels: needs-human"; then
    pass "list: shows labels"
else
    fail "list: shows labels (output: $output)"
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
if echo "$output" | jq -e '[.[] | select(.id == "0002")] | .[0] | .title == "A blocked ticket" and .closed == false and (.labels | index("needs-human")) != null and .blocked_by[0].kind == "local" and .blocked_by[0].id == "0001"' >/dev/null 2>&1; then
    pass "list --json: entry has id, title, closed, labels, blocked_by"
else
    fail "list --json: entry has id, title, closed, labels, blocked_by (output: $output)"
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

# --- labels is always an array (never null) for unlabeled tickets ---
if echo "$output" | jq -e '[.[] | select(.id == "0001")] | .[0].labels == []' >/dev/null 2>&1; then
    pass "list --json: unlabeled ticket has labels == []"
else
    fail "list --json: unlabeled ticket has labels == [] (output: $output)"
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

# --- Positive label filter: only tickets carrying the label ---
output=$($ERG list needs-human "$FIXTURES/store")
if echo "$output" | grep -qE '^  0002' && ! echo "$output" | grep -qE '^  0001'; then
    pass "list LABEL: keeps only labeled tickets"
else
    fail "list LABEL: keeps only labeled tickets (output: $output)"
fi

# --- Negative label filter: drops tickets carrying the label ---
output=$($ERG list not needs-human "$FIXTURES/store")
if echo "$output" | grep -qE '^  0001' && ! echo "$output" | grep -qE '^  0002'; then
    pass "list not LABEL: drops labeled tickets"
else
    fail "list not LABEL: drops labeled tickets (output: $output)"
fi

# --- blocked pseudo-label: only a local OPEN blocker counts; an unresolved
# --- URI ref is optimistic, so 0005 is NOT blocked (ticket 0253) ---
output=$($ERG list blocked "$FIXTURES/store")
if echo "$output" | grep -qE '^  0002' && ! echo "$output" | grep -qE '^  0005' && ! echo "$output" | grep -qE '^  0001'; then
    pass "list blocked: only the local open blocker counts (unresolved URI ref does not)"
else
    fail "list blocked: only local open blocker counts (output: $output)"
fi

# --- not blocked: drops the local-blocked ticket, keeps the unresolved one ---
output=$($ERG list not blocked "$FIXTURES/store")
if echo "$output" | grep -qE '^  0001' && echo "$output" | grep -qE '^  0005' && ! echo "$output" | grep -qE '^  0002'; then
    pass "list not blocked: drops the local-blocked ticket, keeps the optimistic one"
else
    fail "list not blocked: drops the local-blocked ticket (output: $output)"
fi

# --- closed pseudo-label overrides the implicit open default ---
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

# --- trailing 'not' with no label is an error ---
if $ERG list "$FIXTURES/store" not >/dev/null 2>&1; then
    fail "list: dangling 'not' should error"
else
    pass "list: dangling 'not' errors"
fi

# --- JSON respects filters: only the local-blocked 0002 is "blocked" now ---
output=$($ERG list --json blocked "$FIXTURES/store")
if echo "$output" | jq -e 'all(.[]; .blocked_by | length > 0)' >/dev/null 2>&1 \
    && echo "$output" | jq -e 'any(.[]; .id == "0002")' >/dev/null 2>&1 \
    && echo "$output" | jq -e 'all(.[]; .id != "0005")' >/dev/null 2>&1; then
    pass "list --json blocked: only the local-blocked ticket, every entry has a blocker"
else
    fail "list --json blocked: only local-blocked entries (output: $output)"
fi

# --- An unresolved URI ref is optimistic: 0005 has no blocker entry (0253) ---
allout=$($ERG list --json "$FIXTURES/store")
if echo "$allout" | jq -e '[.[] | select(.id == "0005")] | .[0].blocked_by | length == 0' >/dev/null 2>&1; then
    pass "list --json: unresolved URI ref yields no blocker entry"
else
    fail "list --json: 0005 should have no blocker (output: $allout)"
fi

# --- JSON schema type pinning (ticket 0171) ---
# The contract: array of objects; each has id (string), title (string),
# closed (boolean), labels (array), blocked_by (array), refs (array).
# Additive fields (e.g. "file") are tolerated — only the 6 above are pinned.
output=$($ERG list --json --all "$FIXTURES/store")

# Non-vacuity guard: all(.[]; …) is true on an empty array.
if echo "$output" | jq -e 'length > 0' >/dev/null 2>&1; then
    pass "list --json type pin: output is non-empty"
else
    fail "list --json type pin: output is non-empty (output: $output)"
fi
if echo "$output" | jq -e 'type == "array"' >/dev/null 2>&1; then
    pass "list --json type pin: top-level is array"
else
    fail "list --json type pin: top-level is array (output: $output)"
fi

# Positive type assertions.
if echo "$output" | jq -e 'all(.[]; .id | type == "string")' >/dev/null 2>&1; then
    pass "list --json type pin: id is string"
else
    fail "list --json type pin: id is string (output: $output)"
fi
if echo "$output" | jq -e 'all(.[]; .title | type == "string")' >/dev/null 2>&1; then
    pass "list --json type pin: title is string"
else
    fail "list --json type pin: title is string (output: $output)"
fi
if echo "$output" | jq -e 'all(.[]; .closed | type == "boolean")' >/dev/null 2>&1; then
    pass "list --json type pin: closed is boolean"
else
    fail "list --json type pin: closed is boolean (output: $output)"
fi
if echo "$output" | jq -e 'all(.[]; .labels | type == "array")' >/dev/null 2>&1; then
    pass "list --json type pin: labels is array"
else
    fail "list --json type pin: labels is array (output: $output)"
fi
if echo "$output" | jq -e 'all(.[]; .blocked_by | type == "array")' >/dev/null 2>&1; then
    pass "list --json type pin: blocked_by is array"
else
    fail "list --json type pin: blocked_by is array (output: $output)"
fi
if echo "$output" | jq -e 'all(.[]; .refs | type == "array")' >/dev/null 2>&1; then
    pass "list --json type pin: refs is array"
else
    fail "list --json type pin: refs is array (output: $output)"
fi

# Negative controls: wrong types must fail on the same non-empty output.
if echo "$output" | jq -e 'all(.[]; .id | type == "number")' >/dev/null 2>&1; then
    fail "list --json neg ctrl: id is not number"
else
    pass "list --json neg ctrl: id is not number"
fi
if echo "$output" | jq -e 'all(.[]; .title | type == "number")' >/dev/null 2>&1; then
    fail "list --json neg ctrl: title is not number"
else
    pass "list --json neg ctrl: title is not number"
fi
if echo "$output" | jq -e 'all(.[]; .closed | type == "string")' >/dev/null 2>&1; then
    fail "list --json neg ctrl: closed is not string"
else
    pass "list --json neg ctrl: closed is not string"
fi
if echo "$output" | jq -e 'all(.[]; .labels | type == "null")' >/dev/null 2>&1; then
    fail "list --json neg ctrl: labels is not null"
else
    pass "list --json neg ctrl: labels is not null"
fi
if echo "$output" | jq -e 'all(.[]; .blocked_by | type == "null")' >/dev/null 2>&1; then
    fail "list --json neg ctrl: blocked_by is not null"
else
    pass "list --json neg ctrl: blocked_by is not null"
fi
if echo "$output" | jq -e 'all(.[]; .refs | type == "null")' >/dev/null 2>&1; then
    fail "list --json neg ctrl: refs is not null"
else
    pass "list --json neg ctrl: refs is not null"
fi

# --- JSON refs is [] (not null/missing) for tickets with no matching ref ---
output=$($ERG list --json "$FIXTURES/store")
if echo "$output" | jq -e 'all(.[]; .refs == [])' >/dev/null 2>&1; then
    pass "list --json: refs is [] when no refs match"
else
    fail "list --json: refs is [] when no refs match (output: $output)"
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

# unknown flag rejection (ticket 0180)
    out=$($ERG list --bogus 2>&1) && rc=0 || rc=$?
    if [ "$rc" -ne 0 ] && echo "$out" | grep -q "unknown flag"; then
        pass "unknown flag rejected with usage message"
    else
        fail "unknown flag not rejected (rc=$rc, got: $out)"
    fi

echo "list: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
