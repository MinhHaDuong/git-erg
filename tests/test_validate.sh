#!/bin/sh
# Integration tests for: erg validate
set -eu

ERG="${ERG_BIN:-build/erg}"
FIXTURES=$(mktemp -d)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

trap 'rm -rf "$FIXTURES"' EXIT

echo "=== erg validate ==="

# --- No args prints usage and exits non-zero ---
if $ERG validate >/dev/null 2>&1; then
    fail "no args exits non-zero"
else
    pass "no args exits non-zero"
fi

# --- Directory arg rejected ---
out=$($ERG validate "$FIXTURES" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "is a directory"; then
    pass "directory arg rejected"
else
    fail "directory arg rejected (rc=$rc, got: $out)"
fi

# Fixture reused by "multiple file args pass" and "blocked-by sibling" cases
cat > "$FIXTURES/0001-valid.erg" <<'EOF'
%erg v1
Title: Valid ticket
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
EOF

# --- Multiple file args ---
cat > "$FIXTURES/0002-second.erg" <<'EOF'
%erg v1
Title: Second ticket
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
if $ERG validate "$FIXTURES/0001-valid.erg" "$FIXTURES/0002-second.erg" >/dev/null 2>&1; then
    pass "multiple file args pass"
else
    fail "multiple file args pass"
fi

# --- Status: header rejected with migrate hint ---
cat > "$FIXTURES/0005-status-header.erg" <<'EOF'
%erg v1
Title: Has Status header
Status: open
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0005-status-header.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "no longer part of"; then
    pass "Status: header rejected with migrate hint"
else
    fail "Status: header rejected with migrate hint (rc=$rc, got: $out)"
fi

# --- Closed: with non-empty value passes ---
cat > "$FIXTURES/0006-closed-ok.erg" <<'EOF'
%erg v1
Title: Closed ticket
Created: 2026-01-01
Author: a
Closed: completed in PR #42

--- log ---
2026-01-01T10:00Z a created
--- body ---
EOF
if $ERG validate "$FIXTURES/0006-closed-ok.erg" >/dev/null 2>&1; then
    pass "Closed: with reason accepted"
else
    fail "Closed: with reason accepted"
fi

# --- Closed: appearing twice rejected (non-repeatable) ---
cat > "$FIXTURES/0010-closed-twice.erg" <<'EOF'
%erg v1
Title: Two closed headers
Created: 2026-01-01
Author: a
Closed: first reason
Closed: second reason

--- log ---
2026-01-01T10:00Z a created
--- body ---
EOF
if $ERG validate "$FIXTURES/0010-closed-twice.erg" >/dev/null 2>&1; then
    fail "duplicate Closed: rejected"
else
    pass "duplicate Closed: rejected"
fi

# --- Blocked-by ref to sibling file in same dir passes ---
cat > "$FIXTURES/0013-ref-sibling.erg" <<'EOF'
%erg v1
Title: Refs sibling
Created: 2026-01-01
Author: a
Blocked-by: 0001

--- log ---
--- body ---
EOF
if $ERG validate "$FIXTURES/0013-ref-sibling.erg" >/dev/null 2>&1; then
    pass "blocked-by sibling in same dir accepted"
else
    fail "blocked-by sibling in same dir accepted"
fi

# --- Forge-agnostic host/owner/repo#N reference passes ---
cat > "$FIXTURES/0016-forge-ref.erg" <<'EOF'
%erg v1
Title: Forge-agnostic ref
Created: 2026-01-01
Author: a
Blocked-by: github.com/anthropics/claude-code#1234

--- log ---
--- body ---
EOF
if $ERG validate "$FIXTURES/0016-forge-ref.erg" >/dev/null 2>&1; then
    pass "host/owner/repo#N reference accepted"
else
    fail "host/owner/repo#N reference accepted"
fi

# --- GitLab forge ref passes ---
cat > "$FIXTURES/0017-gitlab-ref.erg" <<'EOF'
%erg v1
Title: GitLab ref
Created: 2026-01-01
Author: a
Blocked-by: gitlab.com/someorg/somerepo#42

--- log ---
--- body ---
EOF
if $ERG validate "$FIXTURES/0017-gitlab-ref.erg" >/dev/null 2>&1; then
    pass "gitlab.com ref accepted"
else
    fail "gitlab.com ref accepted"
fi

# --- gh: with no owner/repo#N rejected ---
cat > "$FIXTURES/0018-gh-bare-colon.erg" <<'EOF'
%erg v1
Title: Bare gh: colon
Created: 2026-01-01
Author: a
Blocked-by: gh:

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0018-gh-bare-colon.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "deprecated"; then
    pass "gh: without owner/repo#N rejected"
else
    fail "gh: without owner/repo#N rejected (rc=$rc, got: $out)"
fi

# --- gh:owner/repo without #number rejected ---
cat > "$FIXTURES/0019-gh-no-number.erg" <<'EOF'
%erg v1
Title: gh: missing number
Created: 2026-01-01
Author: a
Blocked-by: gh:anthropics/claude-code

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0019-gh-no-number.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "deprecated"; then
    pass "gh:owner/repo without #N rejected"
else
    fail "gh:owner/repo without #N rejected (rc=$rc, got: $out)"
fi

# --- Malformed forge ref (missing host/owner/repo) rejected ---
cat > "$FIXTURES/0020-bad-forge.erg" <<'EOF'
%erg v1
Title: Bad forge
Created: 2026-01-01
Author: a
Blocked-by: host/repo#1

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0020-bad-forge.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "malformed ref"; then
    pass "forge ref missing owner rejected"
else
    fail "forge ref missing owner rejected (rc=$rc, got: $out)"
fi

# --- Forge ref with zero issue number rejected ---
cat > "$FIXTURES/0021-forge-zero-num.erg" <<'EOF'
%erg v1
Title: Forge zero number
Created: 2026-01-01
Author: a
Blocked-by: github.com/foo/bar#0

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0021-forge-zero-num.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "leading zero"; then
    pass "forge ref with zero issue number rejected"
else
    fail "forge ref with zero issue number rejected (rc=$rc, got: $out)"
fi

# --- gh: with invalid owner (leading dash) rejected (deprecated) ---
cat > "$FIXTURES/0022-gh-bad-owner.erg" <<'EOF'
%erg v1
Title: Bad owner
Created: 2026-01-01
Author: a
Blocked-by: gh:-bad/repo#1

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0022-gh-bad-owner.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "deprecated"; then
    pass "gh: with invalid owner rejected (deprecated)"
else
    fail "gh: with invalid owner rejected (deprecated) (rc=$rc, got: $out)"
fi

# --- gh: with invalid repo (..) rejected (deprecated) ---
cat > "$FIXTURES/0023-gh-bad-repo.erg" <<'EOF'
%erg v1
Title: Bad repo
Created: 2026-01-01
Author: a
Blocked-by: gh:owner/foo..bar#1

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0023-gh-bad-repo.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "deprecated"; then
    pass "gh: with invalid repo rejected (deprecated)"
else
    fail "gh: with invalid repo rejected (deprecated) (rc=$rc, got: $out)"
fi

# --- Mixed-case scheme (GH#) rejected (case-sensitive) ---
cat > "$FIXTURES/0024-gh-case.erg" <<'EOF'
%erg v1
Title: Wrong case
Created: 2026-01-01
Author: a
Blocked-by: GH#42

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0024-gh-case.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "case-sensitive"; then
    pass "GH# (wrong case) rejected"
else
    fail "GH# (wrong case) rejected (rc=$rc, got: $out)"
fi

# --- Mixed-case gh: variant with extra path rejected (case-sensitive) ---
cat > "$FIXTURES/0025-gh-case-colon-extra.erg" <<'EOF'
%erg v1
Title: Wrong case with colon
Created: 2026-01-01
Author: a
Blocked-by: GH:owner/repo/extra#1

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0025-gh-case-colon-extra.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "case-sensitive"; then
    pass "GH:owner/repo/extra#1 rejected (case-sensitive)"
else
    fail "GH:owner/repo/extra#1 rejected (rc=$rc, got: $out)"
fi

# --- Leading-zero issue number in forge ref rejected ---
cat > "$FIXTURES/0026-forge-zero.erg" <<'EOF'
%erg v1
Title: Leading-zero number
Created: 2026-01-01
Author: a
Blocked-by: github.com/foo/bar#042

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0026-forge-zero.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "leading zero"; then
    pass "forge ref with leading zero rejected"
else
    fail "forge ref with leading zero rejected (rc=$rc, got: $out)"
fi

# --- Tags: valid value accepted ---
cat > "$FIXTURES/0030-tags-valid.erg" <<'EOF'
%erg v1
Title: Tags valid
Created: 2026-01-01
Author: a
Tags: needs-human
Tags: deferred

--- log ---
2026-01-01T10:00Z a created

--- body ---
EOF
if $ERG validate "$FIXTURES/0030-tags-valid.erg" >/dev/null 2>&1; then
    pass "Tags: valid values accepted"
else
    fail "Tags: valid values accepted"
fi

# --- Nonexistent path emits WARNING and exits 0 ---
out=$($ERG validate /no/such/path 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 0 ] && echo "$out" | grep -qi "warning\|skipping"; then
    pass "nonexistent path: exit 0 with WARNING"
else
    fail "nonexistent path: exit 0 with WARNING (rc=$rc, got: $out)"
fi

# --- Non-.erg file emits WARNING and exits 0 ---
touch "$FIXTURES/junk.txt"
out=$($ERG validate "$FIXTURES/junk.txt" 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 0 ] && echo "$out" | grep -qi "warning\|not a .erg file"; then
    pass "non-.erg file: exit 0 with WARNING"
else
    fail "non-.erg file: exit 0 with WARNING (rc=$rc, got: $out)"
fi

echo "validate: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
