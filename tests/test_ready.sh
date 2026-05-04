#!/bin/sh
# Integration tests for: erg ready
set -e

ERG="${ERG_BIN:-tickets/tools/go/erg}"
FIXTURES="tests/fixtures"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

mkdir -p "$FIXTURES/ready"
trap 'rm -rf "$FIXTURES/ready"' EXIT

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
if echo "$output" | grep -q '"id"'; then
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
if echo "$output" | grep -q '"tags": \['; then
    pass "ready JSON includes tags array"
else
    fail "ready JSON includes tags array"
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

echo "ready: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
