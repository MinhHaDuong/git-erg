#!/bin/sh
# Integration tests for: erg ready
#
# ready is a saved filter over `erg list`: open, not blocked, and free of the
# configured skip tags (default: needs-human, deferred). It shares list's
# output format.
set -eu

ERG="${ERG_BIN:-build/erg}"
FIXTURES=$(mktemp -d)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

mkdir -p "$FIXTURES/ready"
tmpdir=
cleanup() {
    rm -rf "$FIXTURES"
    [ -n "$tmpdir" ] && rm -rf "$tmpdir"
}
trap cleanup EXIT

echo "=== erg ready ==="

# --- Open ticket with no blockers is ready ---
cat > "$FIXTURES/ready/0001-open.erg" <<'EOF'
%erg 0.1
Title: Open
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
output=$($ERG ready "$FIXTURES/ready")
if echo "$output" | grep -q "0001"; then
    pass "open ticket is ready"
else
    fail "open ticket is ready"
fi

# --- Heading is 'Ready tickets' ---
if echo "$output" | grep -q "^Ready tickets ("; then
    pass "heading is 'Ready tickets'"
else
    fail "heading is 'Ready tickets' (output: $output)"
fi

# --- Closed ticket (via Closed: header) not ready ---
cat > "$FIXTURES/ready/0001-open.erg" <<'EOF'
%erg 0.1
Title: Closed
Created: 2026-01-01
Author: a
Closed: done

--- log ---
--- body ---
EOF
output=$($ERG ready "$FIXTURES/ready")
if echo "$output" | grep -q "0001"; then
    fail "closed ticket excluded"
else
    pass "closed ticket excluded"
fi

# --- Closed via path component (closed/ subdirectory) excluded ---
mkdir -p "$FIXTURES/ready/closed"
cat > "$FIXTURES/ready/closed/0099-archived.erg" <<'EOF'
%erg 0.1
Title: Closed by path
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
output=$($ERG ready "$FIXTURES/ready")
if echo "$output" | grep -q "0099"; then
    fail "path-closed ticket excluded"
else
    pass "path-closed ticket excluded"
fi
rm -rf "$FIXTURES/ready/closed"

# --- Blocked by open ticket: not ready; the blocker is ready ---
rm -f "$FIXTURES/ready/"*.erg
cat > "$FIXTURES/ready/0001-blocker.erg" <<'EOF'
%erg 0.1
Title: Blocker
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
cat > "$FIXTURES/ready/0002-blocked.erg" <<'EOF'
%erg 0.1
Title: Blocked
Created: 2026-01-01
Author: a
Blocked-by: 0001

--- log ---
--- body ---
EOF
output=$($ERG ready "$FIXTURES/ready")
if echo "$output" | grep -q "0002"; then
    fail "blocked ticket excluded from ready"
else
    pass "blocked ticket excluded from ready"
fi
if echo "$output" | grep -q "0001"; then
    pass "unblocked ticket is ready"
else
    fail "unblocked ticket is ready"
fi

# --- Blocked by closed ticket: ready ---
cat > "$FIXTURES/ready/0001-blocker.erg" <<'EOF'
%erg 0.1
Title: Blocker
Created: 2026-01-01
Author: a
Closed: done

--- log ---
--- body ---
EOF
output=$($ERG ready "$FIXTURES/ready")
if echo "$output" | grep -q "0002"; then
    pass "unblocked after close is ready"
else
    fail "unblocked after close is ready"
fi

# --- Forge-ref blocker is blocking ---
rm -f "$FIXTURES/ready/"*.erg
cat > "$FIXTURES/ready/0030-forge-blocked.erg" <<'EOF'
%erg 0.1
Title: Blocked by forge
Created: 2026-01-01
Author: a
Blocked-by: github.com/anthropics/claude-code#1234

--- log ---
2026-01-01T10:00Z a created

--- body ---
EOF
output=$($ERG ready "$FIXTURES/ready")
if echo "$output" | grep -q "0030"; then
    fail "forge-ref blocker excluded from ready"
else
    pass "forge-ref blocker excluded from ready"
fi

# --- Skip-tagged tickets (needs-human, deferred) excluded ---
rm -f "$FIXTURES/ready/"*.erg
cat > "$FIXTURES/ready/0040-needs-human.erg" <<'EOF'
%erg 0.1
Title: Needs human triage
Created: 2026-01-01
Author: a
Tag: needs-human

--- log ---
2026-01-01T10:00Z a created

--- body ---
EOF
cat > "$FIXTURES/ready/0041-deferred.erg" <<'EOF'
%erg 0.1
Title: Deferred
Created: 2026-01-01
Author: a
Tag: deferred

--- log ---
2026-01-01T10:00Z a created

--- body ---
EOF
output=$($ERG ready "$FIXTURES/ready")
if echo "$output" | grep -q "0040"; then
    fail "needs-human ticket excluded from ready"
else
    pass "needs-human ticket excluded from ready"
fi
if echo "$output" | grep -q "0041"; then
    fail "deferred ticket excluded from ready"
else
    pass "deferred ticket excluded from ready"
fi

# --- JSON output: list schema, no ready/claimed fields ---
rm -f "$FIXTURES/ready/"*.erg
cat > "$FIXTURES/ready/0050-ready.erg" <<'EOF'
%erg 0.1
Title: Ready for JSON
Created: 2026-01-01
Author: a

--- log ---
2026-01-01T10:00Z a created

--- body ---
EOF
output=$($ERG ready --json "$FIXTURES/ready")
if echo "$output" | jq -e '.[0] | .id == "0050" and .closed == false and (.tags == []) and (.blocked_by == [])' >/dev/null 2>&1; then
    pass "ready --json: list schema (id, closed, tags, blocked_by)"
else
    fail "ready --json: list schema (output: $output)"
fi
if echo "$output" | grep -q '"claimed"\|"ready"'; then
    fail "ready --json: dropped fields (claimed/ready) absent"
else
    pass "ready --json: dropped fields (claimed/ready) absent"
fi

# --- Empty dir handled ---
rm -rf "$FIXTURES/ready"
mkdir -p "$FIXTURES/ready"
output=$($ERG ready "$FIXTURES/ready")
if echo "$output" | grep -qi "no ready tickets"; then
    pass "empty dir handled"
else
    fail "empty dir handled (output: $output)"
fi

# --- Unknown blocker ID: ticket is ready and WARNING on stderr ---
cat > "$FIXTURES/ready/0060-unknown-blocker.erg" <<'EOF'
%erg 0.1
Title: Blocked by unknown ticket
Created: 2026-01-01
Author: a
Blocked-by: 9999

--- log ---
2026-01-01T10:00Z a created

--- body ---
EOF
tmpout=$(mktemp); tmperr=$(mktemp)
$ERG ready "$FIXTURES/ready" >"$tmpout" 2>"$tmperr" || true
stdout=$(cat "$tmpout"); stderr=$(cat "$tmperr")
rm -f "$tmpout" "$tmperr"
if echo "$stdout" | grep -q "0060" && echo "$stderr" | grep -q "WARNING" && echo "$stderr" | grep -q "9999"; then
    pass "unknown blocker: ticket is ready and WARNING on stderr with ID"
else
    fail "unknown blocker: ready + WARNING (stdout: $stdout) (stderr: $stderr)"
fi

# --- Offline-safe: no-repo dir doesn't crash ---
tmpdir=$(mktemp -d)
(
    cd "$tmpdir" && mkdir tickets && \
    cat > tickets/0001-foo.erg <<'EOF'
%erg 0.1
Title: Offline test
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
    $ERG ready --json tickets/
)
rc=$?
rm -rf "$tmpdir"
if [ "$rc" -eq 0 ]; then
    pass "offline-safe: no-repo dir doesn't crash"
else
    fail "offline-safe: no-repo dir doesn't crash (exit code: $rc)"
fi

echo "ready: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
