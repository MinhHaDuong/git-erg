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

# --- Valid ticket passes ---
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
if $ERG validate "$FIXTURES/0001-valid.erg" >/dev/null 2>&1; then
    pass "valid ticket passes"
else
    fail "valid ticket passes"
fi

# --- Missing magic line fails ---
cat > "$FIXTURES/0002-no-magic.erg" <<'EOF'
Title: No magic
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
if $ERG validate "$FIXTURES/0002-no-magic.erg" >/dev/null 2>&1; then
    fail "missing magic line detected"
else
    pass "missing magic line detected"
fi

# --- Unknown header fails ---
cat > "$FIXTURES/0003-bad-header.erg" <<'EOF'
%erg v1
Title: Bad header
Created: 2026-01-01
Author: a
X-Phase: dreaming

--- log ---
--- body ---
EOF
if $ERG validate "$FIXTURES/0003-bad-header.erg" >/dev/null 2>&1; then
    fail "unknown header rejected"
else
    pass "unknown header rejected"
fi

# --- Status: header rejected with migrate hint ---
cat > "$FIXTURES/0004-status-header.erg" <<'EOF'
%erg v1
Title: Has Status header
Status: open
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0004-status-header.erg" 2>&1 || true)
if echo "$out" | grep -q "no longer part of"; then
    pass "Status: header rejected with migrate hint"
else
    fail "Status: header rejected with migrate hint (got: $out)"
fi

# --- Closed: with non-empty value passes ---
cat > "$FIXTURES/0005-closed-ok.erg" <<'EOF'
%erg v1
Title: Closed ticket
Created: 2026-01-01
Author: a
Closed: completed in PR #42

--- log ---
2026-01-01T10:00Z a created
--- body ---
EOF
if $ERG validate "$FIXTURES/0005-closed-ok.erg" >/dev/null 2>&1; then
    pass "Closed: with reason accepted"
else
    fail "Closed: with reason accepted"
fi

# --- Closed: with empty value rejected ---
cat > "$FIXTURES/0006-closed-empty.erg" <<'EOF'
%erg v1
Title: Empty closed
Created: 2026-01-01
Author: a
Closed:

--- log ---
2026-01-01T10:00Z a created
--- body ---
EOF
if $ERG validate "$FIXTURES/0006-closed-empty.erg" >/dev/null 2>&1; then
    fail "empty Closed: rejected"
else
    pass "empty Closed: rejected"
fi

# --- Closed: in log section rejected ---
cat > "$FIXTURES/0007-closed-in-log.erg" <<'EOF'
%erg v1
Title: Misplaced closed in log
Created: 2026-01-01
Author: a

--- log ---
2026-01-01T10:00Z a created
Closed: this should not be here
--- body ---
EOF
if $ERG validate "$FIXTURES/0007-closed-in-log.erg" >/dev/null 2>&1; then
    fail "Closed: in log rejected"
else
    pass "Closed: in log rejected"
fi

# --- Closed: in body section rejected ---
cat > "$FIXTURES/0008-closed-in-body.erg" <<'EOF'
%erg v1
Title: Misplaced closed in body
Created: 2026-01-01
Author: a

--- log ---
2026-01-01T10:00Z a created
--- body ---
Closed: this should not be here
EOF
if $ERG validate "$FIXTURES/0008-closed-in-body.erg" >/dev/null 2>&1; then
    fail "Closed: in body rejected"
else
    pass "Closed: in body rejected"
fi

# --- Closed: appearing twice rejected (non-repeatable) ---
cat > "$FIXTURES/0009-closed-twice.erg" <<'EOF'
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
if $ERG validate "$FIXTURES/0009-closed-twice.erg" >/dev/null 2>&1; then
    fail "duplicate Closed: rejected"
else
    pass "duplicate Closed: rejected"
fi

# --- Bad filename pattern fails ---
cat > "$FIXTURES/abc-bad-name.erg" <<'EOF'
%erg v1
Title: Bad name
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
if $ERG validate "$FIXTURES/abc-bad-name.erg" >/dev/null 2>&1; then
    fail "bad filename pattern rejected"
else
    pass "bad filename pattern rejected"
fi

# --- Missing separators fail ---
cat > "$FIXTURES/0010-no-sep.erg" <<'EOF'
%erg v1
Title: No separators
Created: 2026-01-01
Author: a
EOF
if $ERG validate "$FIXTURES/0010-no-sep.erg" >/dev/null 2>&1; then
    fail "missing separators rejected"
else
    pass "missing separators rejected"
fi

# --- Blocked-by unknown ID fails ---
cat > "$FIXTURES/0011-bad-ref.erg" <<'EOF'
%erg v1
Title: Bad ref
Created: 2026-01-01
Author: a
Blocked-by: 9999

--- log ---
--- body ---
EOF
if $ERG validate "$FIXTURES/0011-bad-ref.erg" >/dev/null 2>&1; then
    fail "unknown blocked-by rejected"
else
    pass "unknown blocked-by rejected"
fi

# --- Blocked-by ID found in closed/ subdir passes ---
TDIR=$(mktemp -d)
mkdir -p "$TDIR/closed"
cat > "$TDIR/closed/0012-closed-ref.erg" <<'EOF'
%erg v1
Title: Closed ref target
Created: 2026-01-01
Author: a
Closed: done

--- log ---
--- body ---
EOF
cat > "$TDIR/0013-closed-subdir-ref.erg" <<'EOF'
%erg v1
Title: Ref to closed subdir ticket
Created: 2026-01-01
Author: a
Blocked-by: 0012

--- log ---
--- body ---
EOF
if $ERG validate "$TDIR" >/dev/null 2>&1; then
    pass "blocked-by in closed subdir accepted"
else
    fail "blocked-by in closed subdir accepted"
fi
rm -rf "$TDIR"
TDIR=

# --- gh#N references (deprecated) rejected ---
cat > "$FIXTURES/0012-gh-ref.erg" <<'EOF'
%erg v1
Title: GitHub ref
Created: 2026-01-01
Author: a
Blocked-by: gh#435

--- log ---
--- body ---
EOF
if $ERG validate "$FIXTURES/0012-gh-ref.erg" >/dev/null 2>&1; then
    fail "gh#N reference (deprecated) rejected"
else
    pass "gh#N reference (deprecated) rejected"
fi

# --- gh:owner/repo#N cross-repo reference (deprecated) rejected ---
cat > "$FIXTURES/0016-gh-cross.erg" <<'EOF'
%erg v1
Title: Cross-repo GitHub ref
Created: 2026-01-01
Author: a
Blocked-by: gh:anthropics/claude-code#1234

--- log ---
--- body ---
EOF
if $ERG validate "$FIXTURES/0016-gh-cross.erg" >/dev/null 2>&1; then
    fail "gh:owner/repo#N reference (deprecated) rejected"
else
    pass "gh:owner/repo#N reference (deprecated) rejected"
fi

# --- Forge-agnostic host/owner/repo#N reference passes ---
cat > "$FIXTURES/0018-forge-ref.erg" <<'EOF'
%erg v1
Title: Forge-agnostic ref
Created: 2026-01-01
Author: a
Blocked-by: github.com/anthropics/claude-code#1234

--- log ---
--- body ---
EOF
if $ERG validate "$FIXTURES/0018-forge-ref.erg" >/dev/null 2>&1; then
    pass "host/owner/repo#N reference accepted"
else
    fail "host/owner/repo#N reference accepted"
fi

# --- GitLab forge ref passes ---
cat > "$FIXTURES/0019-gitlab-ref.erg" <<'EOF'
%erg v1
Title: GitLab ref
Created: 2026-01-01
Author: a
Blocked-by: gitlab.com/someorg/somerepo#42

--- log ---
--- body ---
EOF
if $ERG validate "$FIXTURES/0019-gitlab-ref.erg" >/dev/null 2>&1; then
    pass "gitlab.com ref accepted"
else
    fail "gitlab.com ref accepted"
fi

# --- gh: with no owner/repo#N rejected ---
cat > "$FIXTURES/0017-gh-bare-colon.erg" <<'EOF'
%erg v1
Title: Bare gh: colon
Created: 2026-01-01
Author: a
Blocked-by: gh:

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0017-gh-bare-colon.erg" 2>&1 || true)
if echo "$out" | grep -q "deprecated"; then
    pass "gh: without owner/repo#N rejected"
else
    fail "gh: without owner/repo#N rejected (got: $out)"
fi

# --- gh:owner/repo without #number rejected ---
cat > "$FIXTURES/0020-gh-no-number.erg" <<'EOF'
%erg v1
Title: gh: missing number
Created: 2026-01-01
Author: a
Blocked-by: gh:anthropics/claude-code

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0020-gh-no-number.erg" 2>&1 || true)
if echo "$out" | grep -q "deprecated"; then
    pass "gh:owner/repo without #N rejected"
else
    fail "gh:owner/repo without #N rejected (got: $out)"
fi

# --- Malformed forge ref (missing host/owner/repo) rejected ---
cat > "$FIXTURES/0021-bad-forge.erg" <<'EOF'
%erg v1
Title: Bad forge
Created: 2026-01-01
Author: a
Blocked-by: host/repo#1

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0021-bad-forge.erg" 2>&1 || true)
if echo "$out" | grep -q "malformed ref"; then
    pass "forge ref missing owner rejected"
else
    fail "forge ref missing owner rejected (got: $out)"
fi

# --- Forge ref with zero issue number rejected ---
cat > "$FIXTURES/0022-forge-zero-num.erg" <<'EOF'
%erg v1
Title: Forge zero number
Created: 2026-01-01
Author: a
Blocked-by: github.com/foo/bar#0

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0022-forge-zero-num.erg" 2>&1 || true)
if echo "$out" | grep -q "leading zero"; then
    pass "forge ref with zero issue number rejected"
else
    fail "forge ref with zero issue number rejected (got: $out)"
fi

# --- gh: with invalid owner (leading dash) rejected (deprecated) ---
cat > "$FIXTURES/0023-gh-bad-owner.erg" <<'EOF'
%erg v1
Title: Bad owner
Created: 2026-01-01
Author: a
Blocked-by: gh:-bad/repo#1

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0023-gh-bad-owner.erg" 2>&1 || true)
if echo "$out" | grep -q "deprecated"; then
    pass "gh: with invalid owner rejected (deprecated)"
else
    fail "gh: with invalid owner rejected (deprecated) (got: $out)"
fi

# --- gh: with invalid repo (..) rejected (deprecated) ---
cat > "$FIXTURES/0024-gh-bad-repo.erg" <<'EOF'
%erg v1
Title: Bad repo
Created: 2026-01-01
Author: a
Blocked-by: gh:owner/foo..bar#1

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0024-gh-bad-repo.erg" 2>&1 || true)
if echo "$out" | grep -q "deprecated"; then
    pass "gh: with invalid repo rejected (deprecated)"
else
    fail "gh: with invalid repo rejected (deprecated) (got: $out)"
fi

# --- Mixed-case scheme (GH#) rejected (case-sensitive) ---
cat > "$FIXTURES/0025-gh-case.erg" <<'EOF'
%erg v1
Title: Wrong case
Created: 2026-01-01
Author: a
Blocked-by: GH#42

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0025-gh-case.erg" 2>&1 || true)
if echo "$out" | grep -q "case-sensitive"; then
    pass "GH# (wrong case) rejected"
else
    fail "GH# (wrong case) rejected (got: $out)"
fi

# --- Mixed-case gh: variant with extra path rejected (case-sensitive) ---
cat > "$FIXTURES/0028-gh-case-colon-extra.erg" <<'EOF'
%erg v1
Title: Wrong case with colon
Created: 2026-01-01
Author: a
Blocked-by: GH:owner/repo/extra#1

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0028-gh-case-colon-extra.erg" 2>&1 || true)
if echo "$out" | grep -q "case-sensitive"; then
    pass "GH:owner/repo/extra#1 rejected (case-sensitive)"
else
    fail "GH:owner/repo/extra#1 rejected (got: $out)"
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
out=$($ERG validate "$FIXTURES/0026-forge-zero.erg" 2>&1 || true)
if echo "$out" | grep -q "leading zero"; then
    pass "forge ref with leading zero rejected"
else
    fail "forge ref with leading zero rejected (got: $out)"
fi
# --- Forge ref is parsed correctly (no local cycle error) ---
mkdir -p "$FIXTURES/cross"
cat > "$FIXTURES/cross/0001-one.erg" <<'EOF'
%erg v1
Title: One
Created: 2026-01-01
Author: a
Blocked-by: github.com/other/repo#42

--- log ---
--- body ---
EOF
if $ERG validate "$FIXTURES/cross" >/dev/null 2>&1; then
    pass "forge ref does not cause local cycle/unknown-id error"
else
    fail "forge ref does not cause local cycle/unknown-id error"
fi
rm -rf "$FIXTURES/cross"

# --- Malformed log line fails ---
cat > "$FIXTURES/0027-bad-log.erg" <<'EOF'
%erg v1
Title: Bad log
Created: 2026-01-01
Author: a

--- log ---
this is not valid

--- body ---
EOF
if $ERG validate "$FIXTURES/0027-bad-log.erg" >/dev/null 2>&1; then
    fail "malformed log line rejected"
else
    pass "malformed log line rejected"
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
if $ERG validate "$FIXTURES/dup" >/dev/null 2>&1; then
    fail "duplicate IDs rejected"
else
    pass "duplicate IDs rejected"
fi

# --- Dependency cycle fails ---
mkdir -p "$FIXTURES/dup"
cat > "$FIXTURES/dup/0001-one.erg" <<'EOF'
%erg v1
Title: One
Created: 2026-01-01
Author: a
Blocked-by: 0002

--- log ---
--- body ---
EOF
cat > "$FIXTURES/dup/0002-two.erg" <<'EOF'
%erg v1
Title: Two
Created: 2026-01-01
Author: a
Blocked-by: 0001

--- log ---
--- body ---
EOF
if $ERG validate "$FIXTURES/dup" >/dev/null 2>&1; then
    fail "dependency cycle rejected"
else
    pass "dependency cycle rejected"
fi

# --- Duplicate '--- body ---' separator fails ---
cat > "$FIXTURES/0014-dup-body.erg" <<'EOF'
%erg v1
Title: Duplicate body separator
Created: 2026-01-01
Author: a

--- log ---
2026-01-01T10:00Z a created
--- body ---
first body line
--- body ---
content past a duplicate separator
EOF
if $ERG validate "$FIXTURES/0014-dup-body.erg" >/dev/null 2>&1; then
    fail "duplicate body separator rejected"
else
    pass "duplicate body separator rejected"
fi

# --- Duplicate '--- log ---' separator fails ---
cat > "$FIXTURES/0015-dup-log.erg" <<'EOF'
%erg v1
Title: Duplicate log separator
Created: 2026-01-01
Author: a

--- log ---
2026-01-01T10:00Z a created
--- log ---
2026-01-01T10:01Z a note duplicate
--- body ---
body
EOF
if $ERG validate "$FIXTURES/0015-dup-log.erg" >/dev/null 2>&1; then
    fail "duplicate log separator rejected"
else
    pass "duplicate log separator rejected"
fi

# --- Real tickets pass ---
if $ERG validate tickets/ >/dev/null 2>&1; then
    pass "real tickets pass"
else
    fail "real tickets pass"
fi

# --- Tags: valid value accepted ---
cat > "$FIXTURES/0099-tags-valid.erg" <<'EOF'
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
if $ERG validate "$FIXTURES/0099-tags-valid.erg" >/dev/null 2>&1; then
    pass "Tags: valid values accepted"
else
    fail "Tags: valid values accepted"
fi

# --- Tags: unknown value rejected ---
cat > "$FIXTURES/0099-tags-invalid.erg" <<'EOF'
%erg v1
Title: Tags invalid
Created: 2026-01-01
Author: a
Tags: unknown-label

--- log ---
2026-01-01T10:00Z a created

--- body ---
EOF
if $ERG validate "$FIXTURES/0099-tags-invalid.erg" >/dev/null 2>&1; then
    fail "Tags: unknown value rejected"
else
    pass "Tags: unknown value rejected"
fi

# --- Nonexistent path emits WARNING and exits 0 ---
out=$($ERG validate /no/such/path 2>&1 || true)
if echo "$out" | grep -q "WARNING" && $ERG validate /no/such/path >/dev/null 2>&1; then
    pass "nonexistent path: exit 0 with WARNING"
else
    fail "nonexistent path: exit 0 with WARNING (got: $out)"
fi

# --- Non-.erg file emits WARNING and exits 0 ---
touch "$FIXTURES/junk.txt"
out=$($ERG validate "$FIXTURES/junk.txt" 2>&1 || true)
if $ERG validate "$FIXTURES/junk.txt" >/dev/null 2>&1; then
    if echo "$out" | grep -q "WARNING" && echo "$out" | grep -q "not a .erg file"; then
        pass "non-.erg file: exit 0 with WARNING"
    else
        fail "non-.erg file: exit 0 with WARNING (got: $out)"
    fi
else
    fail "non-.erg file: expected exit 0"
fi

echo "validate: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
