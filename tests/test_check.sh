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
%erg 0.1
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
%erg 0.1
Title: One
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
cat > "$FIXTURES/dup/0001-two.erg" <<'EOF'
%erg 0.1
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
%erg 0.1
Title: One
Created: 2026-01-01
Author: a
Blocked-by: 0002

--- log ---
--- body ---
EOF
cat > "$FIXTURES/cycle/0002-two.erg" <<'EOF'
%erg 0.1
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
%erg 0.1
Title: Closed ref target
Created: 2026-01-01
Author: a
Closed: done

--- log ---
--- body ---
EOF
cat > "$FIXTURES/cross/0002-refs-closed.erg" <<'EOF'
%erg 0.1
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
%erg 0.1
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
%erg 0.1
Title: Open but in closed dir
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
rc=0; out=$($ERG check "$FIXTURES/closure" 2>&1) || rc=$?
if echo "$out" | grep -q "WARN.*open ticket in closed"; then
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
%erg 0.1
Title: Closed at top level
Created: 2026-01-01
Author: a
Closed: done

--- log ---
--- body ---
EOF
out=$($ERG check "$FIXTURES/closure2" 2>&1)
if echo "$out" | grep -q "WARN.*closed ticket not in closed"; then
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

# --- Stray Go source warns ---
mkdir -p "$FIXTURES/stray/tools/go"
cat > "$FIXTURES/stray/0001-x.erg" <<'EOF'
%erg 0.1
Title: x
Created: 2026-01-01
Author: x

--- log ---

--- body ---
EOF
touch "$FIXTURES/stray/tools/go/fake.go"
rc=0; out=$($ERG check "$FIXTURES/stray" 2>&1) || rc=$?
if echo "$out" | grep -qF "WARN: Go source files found in"; then
    pass "stray Go source warns"
else
    fail "stray Go source warns (got: $out)"
fi
if [ $rc -eq 0 ]; then
    pass "stray Go source warning exits 0"
else
    fail "stray Go source warning exits 0"
fi

# --- Stray Go source at tickets root (top-level scan) warns ---
mkdir -p "$FIXTURES/stray-toplevel"
cat > "$FIXTURES/stray-toplevel/0001-x.erg" <<'EOF'
%erg 0.1
Title: x
Created: 2026-01-01
Author: x

--- log ---

--- body ---
EOF
touch "$FIXTURES/stray-toplevel/main.go"
rc=0; out=$($ERG check "$FIXTURES/stray-toplevel" 2>&1) || rc=$?
if echo "$out" | grep -qF "WARN: Go source files found in"; then
    pass "stray Go source at tickets root warns"
else
    fail "stray Go source at tickets root warns (got: $out)"
fi
if [ $rc -eq 0 ]; then
    pass "stray Go source at root warning exits 0"
else
    fail "stray Go source at root warning exits 0"
fi

# --- go.mod in tools/go/ warns regardless of module name (no exception) ---
mkdir -p "$FIXTURES/gomod/tools/go"
cat > "$FIXTURES/gomod/0001-x.erg" <<'EOF'
%erg 0.1
Title: x
Created: 2026-01-01
Author: x

--- log ---

--- body ---
EOF
cat > "$FIXTURES/gomod/tools/go/go.mod" <<'EOF'
module git-erg

go 1.21
EOF
rc=0; out=$($ERG check "$FIXTURES/gomod" 2>&1) || rc=$?
if echo "$out" | grep -qF "WARN: Go source files found in"; then
    pass "tools/go go.mod warns (no module-name exception)"
else
    fail "tools/go go.mod warns (no module-name exception) (got: $out)"
fi
if [ $rc -eq 0 ]; then
    pass "tools/go go.mod warning exits 0"
else
    fail "tools/go go.mod warning exits 0"
fi

# --- Plural: 1 warning singular form ---
mkdir -p "$FIXTURES/warn1/closed"
cat > "$FIXTURES/warn1/closed/0001-open-in-closed-sing.erg" <<'EOF'
%erg 0.1
Title: Open but in closed dir
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
out=$($ERG check "$FIXTURES/warn1" 2>&1) || true
if echo "$out" | grep -qF ", 1 warning)"; then
    pass "check: 1 warning uses singular"
else
    fail "check: 1 warning uses singular (got: $out)"
fi
if echo "$out" | grep -qF "warning(s)"; then
    fail "check: no (s) fake plural for 1 warning"
else
    pass "check: no (s) fake plural for 1 warning"
fi

# --- Plural: 2 warnings plural form ---
mkdir -p "$FIXTURES/warn2/closed"
cat > "$FIXTURES/warn2/closed/0001-open-in-closed-pl.erg" <<'EOF'
%erg 0.1
Title: Open but in closed dir A
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
cat > "$FIXTURES/warn2/0002-closed-top-pl.erg" <<'EOF'
%erg 0.1
Title: Closed at top level B
Created: 2026-01-01
Author: a
Closed: done

--- log ---
--- body ---
EOF
out=$($ERG check "$FIXTURES/warn2" 2>&1) || true
if echo "$out" | grep -qF ", 2 warnings)"; then
    pass "check: 2 warnings uses plural"
else
    fail "check: 2 warnings uses plural (got: $out)"
fi

# --- Plural: 1 error singular form ---
mkdir -p "$FIXTURES/err1"
cat > "$FIXTURES/err1/0001-bad-date.erg" <<'EOF'
%erg 0.1
Title: Bad date
Created: not-a-date
Author: a

--- log ---
--- body ---
EOF
out=$($ERG check "$FIXTURES/err1" 2>&1) || true
if echo "$out" | grep -qF "FAILED (1 error)"; then
    pass "check: 1 error uses singular"
else
    fail "check: 1 error uses singular (got: $out)"
fi
if echo "$out" | grep -qF "error(s)"; then
    fail "check: no (s) fake plural for 1 error"
else
    pass "check: no (s) fake plural for 1 error"
fi

# --- Plural: 2 errors plural form ---
mkdir -p "$FIXTURES/err2"
cat > "$FIXTURES/err2/0001-bad-date2.erg" <<'EOF'
%erg 0.1
Title: Bad date
Created: not-a-date
Author: a
Tag: bogus-tag

--- log ---
--- body ---
EOF
out=$($ERG check "$FIXTURES/err2" 2>&1) || true
if echo "$out" | grep -qF "FAILED (2 errors)"; then
    pass "check: 2 errors uses plural"
else
    fail "check: 2 errors uses plural (got: $out)"
fi

# --- Self-reference cycle (length 1) detected ---
mkdir -p "$FIXTURES/self-cycle"
cat > "$FIXTURES/self-cycle/0001-self.erg" <<'EOF'
%erg 0.1
Title: Self reference
Created: 2026-01-01
Author: a
Blocked-by: 0001

--- log ---
--- body ---
EOF
out=$($ERG check "$FIXTURES/self-cycle" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "dependency cycle"; then
    pass "self-reference cycle detected"
else
    fail "self-reference cycle detected (rc=$rc, got: $out)"
fi

# --- Length-3 cycle (A->B->C->A) detected ---
mkdir -p "$FIXTURES/cycle3"
cat > "$FIXTURES/cycle3/0001-a.erg" <<'EOF'
%erg 0.1
Title: A
Created: 2026-01-01
Author: a
Blocked-by: 0002

--- log ---
--- body ---
EOF
cat > "$FIXTURES/cycle3/0002-b.erg" <<'EOF'
%erg 0.1
Title: B
Created: 2026-01-01
Author: a
Blocked-by: 0003

--- log ---
--- body ---
EOF
cat > "$FIXTURES/cycle3/0003-c.erg" <<'EOF'
%erg 0.1
Title: C
Created: 2026-01-01
Author: a
Blocked-by: 0001

--- log ---
--- body ---
EOF
out=$($ERG check "$FIXTURES/cycle3" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "dependency cycle"; then
    pass "length-3 cycle detected"
else
    fail "length-3 cycle detected (rc=$rc, got: $out)"
fi

# --- Stale Blocked-by: open ticket refs closed ticket warns (Case A) ---
mkdir -p "$FIXTURES/stale-blocked/closed"
cat > "$FIXTURES/stale-blocked/closed/0001-blocker.erg" <<'EOF'
%erg 0.1
Title: Blocker closed
Created: 2026-01-01
Author: a
Closed: done

--- log ---
--- body ---
EOF
cat > "$FIXTURES/stale-blocked/0002-stale.erg" <<'EOF'
%erg 0.1
Title: Stale blocked-by
Created: 2026-01-01
Author: a
Blocked-by: 0001

--- log ---
--- body ---
EOF
rc=0; out=$($ERG check "$FIXTURES/stale-blocked" 2>&1) || rc=$?
if echo "$out" | grep -q "Blocked-by 0001 is already closed"; then
    pass "stale Blocked-by warns"
else
    fail "stale Blocked-by warns (got: $out)"
fi
if [ $rc -eq 0 ]; then
    pass "stale Blocked-by warning exits 0"
else
    fail "stale Blocked-by warning exits 0"
fi

# --- Stale Blocked-by: both open — no stale warn (Case B) ---
mkdir -p "$FIXTURES/no-stale"
cat > "$FIXTURES/no-stale/0001-open.erg" <<'EOF'
%erg 0.1
Title: A blocker ticket
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
cat > "$FIXTURES/no-stale/0002-blocked.erg" <<'EOF'
%erg 0.1
Title: Depends on the blocker
Created: 2026-01-01
Author: a
Blocked-by: 0001

--- log ---
--- body ---
EOF
rc=0; out=$($ERG check "$FIXTURES/no-stale" 2>&1) || rc=$?
if echo "$out" | grep -q "is already closed"; then
    fail "no stale warn when blocker is open (got: $out)"
else
    pass "no stale warn when blocker is open"
fi
if [ $rc -eq 0 ]; then
    pass "no stale warn exits 0"
else
    fail "no stale warn exits 0"
fi

# --- Stale Blocked-by: forge ref skipped — no stale warn (Case C) ---
mkdir -p "$FIXTURES/stale-forge"
cat > "$FIXTURES/stale-forge/0001-forge-only.erg" <<'EOF'
%erg 0.1
Title: Forge blocked only
Created: 2026-01-01
Author: a
Blocked-by: github.com/other/repo#1

--- log ---
--- body ---
EOF
rc=0; out=$($ERG check "$FIXTURES/stale-forge" 2>&1) || rc=$?
if echo "$out" | grep -q "is already closed"; then
    fail "forge ref skipped for stale check (got: $out)"
else
    pass "forge ref skipped for stale check"
fi
if [ $rc -eq 0 ]; then
    pass "forge ref stale check exits 0"
else
    fail "forge ref stale check exits 0"
fi

# --- Encoding warning: CRLF file warns ---
mkdir -p "$FIXTURES/enc-crlf"
printf '%%erg 0.1\r\nTitle: x\r\nCreated: 2026-01-01\r\nAuthor: x\r\n\r\n--- log ---\r\n--- body ---\r\n' > "$FIXTURES/enc-crlf/0001-crlf.erg"
rc=0; out=$($ERG check "$FIXTURES/enc-crlf" 2>&1) || rc=$?
if echo "$out" | grep -q "WARNING.*CRLF"; then
    pass "CRLF encoding warning emitted"
else
    fail "CRLF encoding warning emitted (got: $out)"
fi
if [ $rc -eq 0 ]; then
    pass "CRLF encoding warning exits 0"
else
    fail "CRLF encoding warning exits 0"
fi

# --- Encoding warning: BOM file warns ---
mkdir -p "$FIXTURES/enc-bom"
printf '\357\273\277%%erg 0.1\nTitle: x\nCreated: 2026-01-01\nAuthor: x\n\n--- log ---\n--- body ---\n' > "$FIXTURES/enc-bom/0001-bom.erg"
rc=0; out=$($ERG check "$FIXTURES/enc-bom" 2>&1) || rc=$?
if echo "$out" | grep -q "WARNING.*BOM"; then
    pass "BOM encoding warning emitted"
else
    fail "BOM encoding warning emitted (got: $out)"
fi
if [ $rc -eq 0 ]; then
    pass "BOM encoding warning exits 0"
else
    fail "BOM encoding warning exits 0"
fi

# --- Encoding warning: clean file no warning ---
mkdir -p "$FIXTURES/enc-clean"
cat > "$FIXTURES/enc-clean/0001-clean.erg" <<'EOF'
%erg 0.1
Title: Clean
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
out=$($ERG check "$FIXTURES/enc-clean" 2>&1)
if echo "$out" | grep -q "WARNING.*BOM\|WARNING.*CRLF"; then
    fail "clean file has no encoding warning (got: $out)"
else
    pass "clean file has no encoding warning"
fi

# --- Interior header blank: check warns (non-fatal) and exits 0 ---
mkdir -p "$FIXTURES/hdr-blank"
cat > "$FIXTURES/hdr-blank/0001-interior.erg" <<'EOF'
%erg 0.1
Title: Interior blank
Created: 2026-01-01
Author: a

Tag: needs-human

--- log ---
--- body ---
EOF
rc=0
out=$($ERG check "$FIXTURES/hdr-blank" 2>&1) || rc=$?
if echo "$out" | grep -q "WARN .*: blank line inside header block"; then
    pass "check warns on interior header blank"
else
    fail "check warns on interior header blank (got: $out)"
fi
if [ "$rc" -eq 0 ]; then
    pass "check exits 0 on interior header blank"
else
    fail "check exits 0 on interior header blank (rc=$rc)"
fi

# --- Clean file: no interior-blank warning ---
mkdir -p "$FIXTURES/hdr-clean"
cat > "$FIXTURES/hdr-clean/0001-clean.erg" <<'EOF'
%erg 0.1
Title: Clean
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
out=$($ERG check "$FIXTURES/hdr-clean" 2>&1)
if echo "$out" | grep -q "blank line inside header block"; then
    fail "clean file must not warn on interior header blank (got: $out)"
else
    pass "clean file has no interior-header-blank warning"
fi

# --- Rule 14 is enforced corpus-wide by check ---
mkdir -p "$FIXTURES/title-rule"
cat > "$FIXTURES/title-rule/0001-bad.erg" <<'EOF'
%erg 0.1
Title: open the config reader to subdir overrides
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
out=$($ERG check "$FIXTURES/title-rule" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "status word 'open'"; then
    pass "rule 14: check surfaces title status word corpus-wide"
else
    fail "rule 14: check surfaces title status word corpus-wide (rc=$rc, got: $out)"
fi

# --- Rule 14: closed ticket grandfathered under check too ---
cat > "$FIXTURES/title-rule/0001-bad.erg" <<'EOF'
%erg 0.1
Title: open the config reader to subdir overrides
Created: 2026-01-01
Author: a
Closed: superseded

--- log ---
2026-01-01T10:00Z a closed — superseded
--- body ---
EOF
out=$($ERG check "$FIXTURES/title-rule" 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 0 ]; then
    pass "rule 14: check grandfathers closed ticket"
else
    fail "rule 14: check grandfathers closed ticket (rc=$rc, got: $out)"
fi

# live-corpus check moved to: make validate

echo "check: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
