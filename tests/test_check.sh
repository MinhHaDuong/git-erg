#!/bin/sh
# Integration tests for: erg check
set -eu

ERG="${ERG_BIN:-build/erg}"
FIXTURES=$(mktemp -d)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

trap 'rm -rf "$FIXTURES"' EXIT

echo "=== erg check ==="

# --- Default dir (tickets/) passes ---
if $ERG check >/dev/null 2>&1; then
    pass "default dir passes"
else
    fail "default dir passes"
fi

# --- Explicit dir passes ---
mkdir -p "$FIXTURES/ok"
cat > "$FIXTURES/ok/0001-one.erg" <<'EOF'
%erg v1
Title: One
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
if $ERG check "$FIXTURES/ok" >/dev/null 2>&1; then
    pass "explicit dir passes"
else
    fail "explicit dir passes"
fi

# --- File arg rejected ---
out=$($ERG check "$FIXTURES/ok/0001-one.erg" 2>&1 || true)
if echo "$out" | grep -q "not a directory"; then
    pass "file arg rejected"
else
    fail "file arg rejected (got: $out)"
fi

# --- Duplicate IDs fail ---
mkdir -p "$FIXTURES/dup"
cat > "$FIXTURES/dup/0001-one.erg" <<'EOF'
%erg v1
Title: One
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
cat > "$FIXTURES/dup/0001-two.erg" <<'EOF'
%erg v1
Title: Two
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
if $ERG check "$FIXTURES/dup" >/dev/null 2>&1; then
    fail "duplicate IDs rejected"
else
    pass "duplicate IDs rejected"
fi

# --- Dependency cycle fails ---
mkdir -p "$FIXTURES/cycle"
cat > "$FIXTURES/cycle/0001-one.erg" <<'EOF'
%erg v1
Title: One
Created: 2026-01-01
Author: a
Blocked-by: 0002

--- log ---
--- body ---
EOF
cat > "$FIXTURES/cycle/0002-two.erg" <<'EOF'
%erg v1
Title: Two
Created: 2026-01-01
Author: a
Blocked-by: 0001

--- log ---
--- body ---
EOF
if $ERG check "$FIXTURES/cycle" >/dev/null 2>&1; then
    fail "dependency cycle rejected"
else
    pass "dependency cycle rejected"
fi

# --- Cross-dir ref resolution (closed subdir) passes ---
mkdir -p "$FIXTURES/cross/closed"
cat > "$FIXTURES/cross/closed/0001-closed-ref.erg" <<'EOF'
%erg v1
Title: Closed ref target
Created: 2026-01-01
Author: a
Closed: done

--- log ---
--- body ---
EOF
cat > "$FIXTURES/cross/0002-refs-closed.erg" <<'EOF'
%erg v1
Title: Ref to closed subdir ticket
Created: 2026-01-01
Author: a
Blocked-by: 0001

--- log ---
--- body ---
EOF
if $ERG check "$FIXTURES/cross" >/dev/null 2>&1; then
    pass "blocked-by in closed subdir accepted"
else
    fail "blocked-by in closed subdir accepted"
fi

# --- Forge ref does not cause local errors ---
mkdir -p "$FIXTURES/forge"
cat > "$FIXTURES/forge/0001-forge-ref.erg" <<'EOF'
%erg v1
Title: Forge ref
Created: 2026-01-01
Author: a
Blocked-by: github.com/other/repo#42

--- log ---
--- body ---
EOF
if $ERG check "$FIXTURES/forge" >/dev/null 2>&1; then
    pass "forge ref does not cause local errors"
else
    fail "forge ref does not cause local errors"
fi

# --- Folder closure: open ticket in closed/ warns ---
mkdir -p "$FIXTURES/closure/closed"
cat > "$FIXTURES/closure/closed/0001-open-in-closed.erg" <<'EOF'
%erg v1
Title: Open but in closed dir
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
rc=0; out=$($ERG check "$FIXTURES/closure" 2>&1) || rc=$?
if echo "$out" | grep -q "WARNING.*open ticket in closed"; then
    pass "open ticket in closed/ warns"
else
    fail "open ticket in closed/ warns (got: $out)"
fi
if [ $rc -eq 0 ]; then
    pass "folder closure warning exits 0"
else
    fail "folder closure warning exits 0"
fi

# --- Folder closure: closed ticket at top level warns ---
mkdir -p "$FIXTURES/closure2"
cat > "$FIXTURES/closure2/0001-closed-top.erg" <<'EOF'
%erg v1
Title: Closed at top level
Created: 2026-01-01
Author: a
Closed: done

--- log ---
--- body ---
EOF
out=$($ERG check "$FIXTURES/closure2" 2>&1)
if echo "$out" | grep -q "WARNING.*closed ticket not in closed"; then
    pass "closed ticket at top level warns"
else
    fail "closed ticket at top level warns (got: $out)"
fi

# --- Nonexistent dir fails ---
if $ERG check /no/such/dir >/dev/null 2>&1; then
    fail "nonexistent dir exits non-zero"
else
    pass "nonexistent dir exits non-zero"
fi

# --- Empty dir exits 0 ---
mkdir -p "$FIXTURES/empty"
if $ERG check "$FIXTURES/empty" >/dev/null 2>&1; then
    pass "empty dir exits 0"
else
    fail "empty dir exits 0"
fi

# live-corpus check moved to: make validate

echo "check: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
