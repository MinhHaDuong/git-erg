#!/bin/sh
# Integration tests for: erg ready
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
    git branch -D test/0098-claim 2>/dev/null || true
    [ -n "$tmpdir" ] && rm -rf "$tmpdir"
}
trap cleanup EXIT

echo "=== erg ready ==="

# --- Open (not-closed) ticket with no blockers is ready ---
cat > "$FIXTURES/ready/0001-open.erg" <<'EOF'
%erg v1
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

# --- Closed ticket (via Closed: header) not in ready list ---
cat > "$FIXTURES/ready/0001-open.erg" <<'EOF'
%erg v1
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
%erg v1
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

# --- Closed via -closed.erg suffix excluded ---
cat > "$FIXTURES/ready/0001-foo-closed.erg" <<'EOF'
%erg v1
Title: Suffix closed
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
output=$($ERG ready "$FIXTURES/ready")
if echo "$output" | grep -q "0001"; then
    fail "suffix-closed ticket excluded"
else
    pass "suffix-closed ticket excluded"
fi
rm -f "$FIXTURES/ready/0001-foo-closed.erg"

# --- 'disclosed' in basename does NOT trigger close ---
cat > "$FIXTURES/ready/0001-disclosed-bug.erg" <<'EOF'
%erg v1
Title: Disclosed (false-positive bait)
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
output=$($ERG ready "$FIXTURES/ready")
if echo "$output" | grep -q "0001"; then
    pass "disclosed in name not treated as closed"
else
    fail "disclosed in name not treated as closed"
fi
rm -f "$FIXTURES/ready/0001-disclosed-bug.erg"

# --- Blocked by open ticket: not ready ---
rm -f "$FIXTURES/ready/"*.erg
cat > "$FIXTURES/ready/0001-blocker.erg" <<'EOF'
%erg v1
Title: Blocker
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
cat > "$FIXTURES/ready/0002-blocked.erg" <<'EOF'
%erg v1
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
%erg v1
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

# --- Blocked by closed ticket in closed/ subdir: still ready ---
rm -f "$FIXTURES/ready/"*.erg
mkdir -p "$FIXTURES/ready/closed"
cat > "$FIXTURES/ready/closed/0001-blocker.erg" <<'EOF'
%erg v1
Title: Archived blocker
Created: 2026-01-01
Author: a
Closed: done

--- log ---
--- body ---
EOF
cat > "$FIXTURES/ready/0002-blocked.erg" <<'EOF'
%erg v1
Title: Blocked by archived ticket
Created: 2026-01-01
Author: a
Blocked-by: 0001

--- log ---
--- body ---
EOF
output=$($ERG ready "$FIXTURES/ready")
if echo "$output" | grep -q "0002"; then
    pass "blocked-by closed subdir ticket is ready"
else
    fail "blocked-by closed subdir ticket is ready"
fi
rm -rf "$FIXTURES/ready/closed"

# --- JSON output ---
output=$($ERG ready --json "$FIXTURES/ready")
if echo "$output" | grep -q '"id": "0002"'; then
    pass "JSON output works"
else
    fail "JSON output works"
fi

# --- Ready excludes tickets carrying skip tags ---
rm -f "$FIXTURES/ready/"*.erg
cat > "$FIXTURES/ready/0040-tagged.erg" <<'EOF'
%erg v1
Title: Needs human triage
Created: 2026-01-01
Author: a
Tags: needs-human

--- log ---
2026-01-01T10:00Z a created

--- body ---
EOF
output=$($ERG ready "$FIXTURES/ready")
if echo "$output" | grep -q "0040"; then
    fail "skip-tagged ticket excluded from ready"
else
    pass "skip-tagged ticket excluded from ready"
fi

# --- JSON output includes tags array for ready entries ---
rm -f "$FIXTURES/ready/"*.erg
cat > "$FIXTURES/ready/0041-untagged.erg" <<'EOF'
%erg v1
Title: Untagged ready ticket
Created: 2026-01-01
Author: a

--- log ---
2026-01-01T10:00Z a created

--- body ---
EOF
output=$($ERG ready --json "$FIXTURES/ready")
if echo "$output" | grep -q '"tags": \[\]'; then
    pass "ready JSON includes tags array"
else
    fail "ready JSON includes tags array"
fi

# --- JSON output includes blocked_by for forge blockers ---
cat > "$FIXTURES/ready/0042-forge-blocked-json.erg" <<'EOF'
%erg v1
Title: Forge blocked for JSON
Created: 2026-01-01
Author: a
Blocked-by: github.com/anthropics/claude-code#1234

--- log ---
2026-01-01T10:00Z a created

--- body ---
EOF
output=$($ERG ready --json "$FIXTURES/ready")
if echo "$output" | grep -q '"blocked_by": \[{"kind": "forge", "ref": "github.com/anthropics/claude-code#1234"}'; then
    pass "ready JSON includes forge blocked_by"
else
    fail "ready JSON includes forge blocked_by"
fi

# --- JSON output includes blocked_by for local blockers ---
cat > "$FIXTURES/ready/0043-local-blocker.erg" <<'EOF'
%erg v1
Title: Local blocker
Created: 2026-01-01
Author: a

--- log ---
2026-01-01T10:00Z a created

--- body ---
EOF
cat > "$FIXTURES/ready/0044-local-blocked.erg" <<'EOF'
%erg v1
Title: Local blocked for JSON
Created: 2026-01-01
Author: a
Blocked-by: 0043

--- log ---
2026-01-01T10:00Z a created

--- body ---
EOF
output=$($ERG ready --json "$FIXTURES/ready")
if echo "$output" | grep -q '"blocked_by": \[{"kind": "local", "id": "0043"}'; then
    pass "ready JSON includes local blocked_by"
else
    fail "ready JSON includes local blocked_by"
fi

# --- Empty dir ---
rm -f "$FIXTURES/ready/"*.erg
# --- Forge-ref blocker is blocking ---
cat > "$FIXTURES/ready/0030-forge-blocked.erg" <<'EOF'
%erg v1
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

# --- Empty dir handled ---
rm -rf "$FIXTURES/ready"
mkdir -p "$FIXTURES/ready"
output=$($ERG ready "$FIXTURES/ready")
if echo "$output" | grep -qi "no tickets"; then
    pass "empty dir handled"
else
    fail "empty dir handled"
fi

# --- Unknown blocker ID: ticket appears in ready list and WARNING on stderr ---
rm -f "$FIXTURES/ready/"*.erg
cat > "$FIXTURES/ready/0050-unknown-blocker.erg" <<'EOF'
%erg v1
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
if echo "$stdout" | grep -q "0050" && echo "$stderr" | grep -q "WARNING" && echo "$stderr" | grep -q "9999"; then
    pass "unknown blocker: ticket is ready and WARNING on stderr with ID"
else
    fail "unknown blocker: ticket is ready and WARNING on stderr with ID (stdout: $stdout) (stderr: $stderr)"
fi

# --- Unclaimed ticket has claimed=false in JSON ---
rm -f "$FIXTURES/ready/"*.erg
cat > "$FIXTURES/ready/0091-unclaimed.erg" <<'EOF'
%erg v1
Title: Unclaimed ticket
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
output=$($ERG ready --json "$FIXTURES/ready")
if echo "$output" | grep -q '"claimed": false'; then
    pass "unclaimed ticket has claimed=false in JSON"
else
    fail "unclaimed ticket has claimed=false in JSON (output: $output)"
fi

# --- Claimed ticket (local branch exists) has claimed=true, ready=false ---
rm -f "$FIXTURES/ready/"*.erg
cat > "$FIXTURES/ready/0098-claimable.erg" <<'EOF'
%erg v1
Title: Claimable ticket
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
git branch test/0098-claim 2>/dev/null || true
output=$($ERG ready --json "$FIXTURES/ready")
claimed_ok=false
ready_ok=false
# Parse the 0098 entry: find ready field on the same JSON object
entry=$(echo "$output" | tr '\n' ' ' | grep -o '{[^}]*"id": "0098"[^}]*}')
if echo "$entry" | grep -q '"claimed": true'; then
    claimed_ok=true
fi
if echo "$entry" | grep -q '"ready": false'; then
    ready_ok=true
fi
if [ "$claimed_ok" = "true" ] && [ "$ready_ok" = "true" ]; then
    pass "claimed ticket has claimed=true and ready=false"
else
    fail "claimed ticket has claimed=true and ready=false (output: $output)"
fi

# --- Human-readable output shows "Claimed" section ---
output=$($ERG ready "$FIXTURES/ready")
if echo "$output" | grep -q "Claimed"; then
    pass "human-readable output shows Claimed section"
else
    fail "human-readable output shows Claimed section (output: $output)"
fi
git branch -D test/0098-claim 2>/dev/null || true

# --- Offline-safe: no-remote repo doesn't crash ---
tmpdir=$(mktemp -d)
(
    cd "$tmpdir" && git init -q && mkdir tickets && \
    cat > tickets/0001-foo.erg <<'EOF'
%erg v1
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
    pass "offline-safe: no-remote repo doesn't crash"
else
    fail "offline-safe: no-remote repo doesn't crash (exit code: $rc)"
fi

echo "ready: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
