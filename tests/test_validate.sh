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
%erg 0.1
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
%erg 0.1
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
%erg 0.1
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
%erg 0.1
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
%erg 0.1
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
%erg 0.1
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
%erg 0.1
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
%erg 0.1
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
%erg 0.1
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
%erg 0.1
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
%erg 0.1
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
%erg 0.1
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
%erg 0.1
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
%erg 0.1
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
%erg 0.1
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
%erg 0.1
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
%erg 0.1
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

# --- Tag: valid value accepted ---
cat > "$FIXTURES/0030-tags-valid.erg" <<'EOF'
%erg 0.1
Title: Tag valid
Created: 2026-01-01
Author: a
Tag: needs-human
Tag: deferred

--- log ---
2026-01-01T10:00Z a created

--- body ---
EOF
if $ERG validate "$FIXTURES/0030-tags-valid.erg" >/dev/null 2>&1; then
    pass "Tag: valid values accepted"
else
    fail "Tag: valid values accepted"
fi

# --- Legacy Tags: header rejected with migration hint ---
cat > "$FIXTURES/0031-tags-legacy.erg" <<'EOF'
%erg 0.1
Title: Tags legacy
Created: 2026-01-01
Author: a
Tags: needs-human

--- log ---
2026-01-01T10:00Z a created

--- body ---
EOF
out=$($ERG validate "$FIXTURES/0031-tags-legacy.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "renamed to 'Tag:'"; then
    pass "Tags: legacy header rejected with migration hint"
else
    fail "Tags: legacy header rejected with migration hint (rc=$rc, got: $out)"
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

# --- Plural: 1 file singular ---
out=$($ERG validate "$FIXTURES/0001-valid.erg" 2>&1)
if echo "$out" | grep -qF "PASS (1 file)"; then
    pass "validate: 1 file uses singular"
else
    fail "validate: 1 file uses singular (got: $out)"
fi
# --- Plural: 2 files plural ---
out=$($ERG validate "$FIXTURES/0001-valid.erg" "$FIXTURES/0002-second.erg" 2>&1)
if echo "$out" | grep -qF "PASS (2 files)"; then
    pass "validate: 2 files uses plural"
else
    fail "validate: 2 files uses plural (got: $out)"
fi

# --- Plural: 1 error singular ---
cat > "$FIXTURES/0090-bad-date.erg" <<'EOF'
%erg 0.1
Title: Bad date
Created: not-a-date
Author: a

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0090-bad-date.erg" 2>&1) || true
if echo "$out" | grep -qF "FAILED (1 error)"; then
    pass "validate: 1 error uses singular"
else
    fail "validate: 1 error uses singular (got: $out)"
fi
if echo "$out" | grep -qF "error(s)"; then
    fail "validate: no (s) fake plural for 1 error"
else
    pass "validate: no (s) fake plural for 1 error"
fi

# --- Plural: 2 errors plural ---
cat > "$FIXTURES/0091-bad-date2.erg" <<'EOF'
%erg 0.1
Title: Bad date two
Created: also-not-a-date
Author: a

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0090-bad-date.erg" "$FIXTURES/0091-bad-date2.erg" 2>&1) || true
if echo "$out" | grep -qF "FAILED (2 errors)"; then
    pass "validate: 2 errors uses plural"
else
    fail "validate: 2 errors uses plural (got: $out)"
fi

# === Rule-coverage gaps (audit additions) ===

# --- Magic line: missing rejected ---
cat > "$FIXTURES/0100-no-magic.erg" <<'EOF'
Title: No magic line
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0100-no-magic.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "missing magic first line"; then
    pass "missing magic line rejected"
else
    fail "missing magic line rejected (rc=$rc, got: $out)"
fi

# --- Magic line: wrong version rejected ---
cat > "$FIXTURES/0101-wrong-version.erg" <<'EOF'
%erg v2
Title: Wrong version
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0101-wrong-version.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "missing magic first line"; then
    pass "%erg v2 magic rejected"
else
    fail "%erg v2 magic rejected (rc=$rc, got: $out)"
fi

# --- Required header: missing Title rejected ---
cat > "$FIXTURES/0102-no-title.erg" <<'EOF'
%erg 0.1
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0102-no-title.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "missing or empty required header 'Title'"; then
    pass "missing Title rejected"
else
    fail "missing Title rejected (rc=$rc, got: $out)"
fi

# --- Required header: missing Created rejected ---
cat > "$FIXTURES/0103-no-created.erg" <<'EOF'
%erg 0.1
Title: No created
Author: a

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0103-no-created.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "missing or empty required header 'Created'"; then
    pass "missing Created rejected"
else
    fail "missing Created rejected (rc=$rc, got: $out)"
fi

# --- Required header: missing Author rejected ---
cat > "$FIXTURES/0104-no-author.erg" <<'EOF'
%erg 0.1
Title: No author
Created: 2026-01-01

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0104-no-author.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "missing or empty required header 'Author'"; then
    pass "missing Author rejected"
else
    fail "missing Author rejected (rc=$rc, got: $out)"
fi

# --- Singleton: Title repeated rejected ---
cat > "$FIXTURES/0105-title-twice.erg" <<'EOF'
%erg 0.1
Title: First
Title: Second
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0105-title-twice.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "'Title' is non-repeatable"; then
    pass "duplicate Title rejected"
else
    fail "duplicate Title rejected (rc=$rc, got: $out)"
fi

# --- Singleton: Created repeated rejected ---
cat > "$FIXTURES/0106-created-twice.erg" <<'EOF'
%erg 0.1
Title: Two creates
Created: 2026-01-01
Created: 2026-01-02
Author: a

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0106-created-twice.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "'Created' is non-repeatable"; then
    pass "duplicate Created rejected"
else
    fail "duplicate Created rejected (rc=$rc, got: $out)"
fi

# --- Singleton: Author repeated rejected ---
cat > "$FIXTURES/0107-author-twice.erg" <<'EOF'
%erg 0.1
Title: Two authors
Created: 2026-01-01
Author: a
Author: b

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0107-author-twice.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "'Author' is non-repeatable"; then
    pass "duplicate Author rejected"
else
    fail "duplicate Author rejected (rc=$rc, got: $out)"
fi

# --- Closed: empty value rejected ---
cat > "$FIXTURES/0108-closed-empty.erg" <<'EOF'
%erg 0.1
Title: Empty closed
Created: 2026-01-01
Author: a
Closed:

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0108-closed-empty.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "non-empty value"; then
    pass "Closed: with empty value rejected"
else
    fail "Closed: with empty value rejected (rc=$rc, got: $out)"
fi

# --- Closed: line in log section rejected ---
cat > "$FIXTURES/0109-closed-in-log.erg" <<'EOF'
%erg 0.1
Title: Closed in log
Created: 2026-01-01
Author: a

--- log ---
2026-01-01T10:00Z a created
Closed: stray reason

--- body ---
EOF
out=$($ERG validate "$FIXTURES/0109-closed-in-log.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "found in log section"; then
    pass "Closed: in log section rejected"
else
    fail "Closed: in log section rejected (rc=$rc, got: $out)"
fi

# --- Closed: line in body section rejected ---
cat > "$FIXTURES/0110-closed-in-body.erg" <<'EOF'
%erg 0.1
Title: Closed in body
Created: 2026-01-01
Author: a

--- log ---

--- body ---
Closed: stray reason
EOF
out=$($ERG validate "$FIXTURES/0110-closed-in-body.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "found in body section"; then
    pass "Closed: in body section rejected"
else
    fail "Closed: in body section rejected (rc=$rc, got: $out)"
fi

# --- Closed: substring in body prose accepted (line-start match required) ---
cat > "$FIXTURES/0111-closed-in-prose.erg" <<'EOF'
%erg 0.1
Title: Mentions a status key in prose
Created: 2026-01-01
Author: a

--- log ---

--- body ---
This issue was Closed: by user X earlier.
  Closed: indented does not count.
EOF
if $ERG validate "$FIXTURES/0111-closed-in-prose.erg" >/dev/null 2>&1; then
    pass "Closed: in body prose (not at column 0) accepted"
else
    fail "Closed: in body prose (not at column 0) accepted"
fi

# --- Filename: 3-digit ID rejected ---
cat > "$FIXTURES/001-three-digit.erg" <<'EOF'
%erg 0.1
Title: Three digit ID
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/001-three-digit.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "filename does not match"; then
    pass "3-digit filename ID rejected"
else
    fail "3-digit filename ID rejected (rc=$rc, got: $out)"
fi

# --- Filename: 5-digit ID rejected ---
cat > "$FIXTURES/12345-five-digit.erg" <<'EOF'
%erg 0.1
Title: Five digit ID
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/12345-five-digit.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "filename does not match"; then
    pass "5-digit filename ID rejected"
else
    fail "5-digit filename ID rejected (rc=$rc, got: $out)"
fi

# --- Filename: uppercase slug rejected ---
cat > "$FIXTURES/0112-Foo.erg" <<'EOF'
%erg 0.1
Title: Uppercase slug
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0112-Foo.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "filename does not match"; then
    pass "uppercase slug rejected"
else
    fail "uppercase slug rejected (rc=$rc, got: $out)"
fi

# --- Filename: underscore in slug rejected ---
cat > "$FIXTURES/0113-foo_bar.erg" <<'EOF'
%erg 0.1
Title: Underscore slug
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0113-foo_bar.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "filename does not match"; then
    pass "underscore slug rejected"
else
    fail "underscore slug rejected (rc=$rc, got: $out)"
fi

# --- Separator: missing --- log --- rejected ---
cat > "$FIXTURES/0114-no-log-sep.erg" <<'EOF'
%erg 0.1
Title: No log separator
Created: 2026-01-01
Author: a

--- body ---
EOF
out=$($ERG validate "$FIXTURES/0114-no-log-sep.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "missing '--- log ---'"; then
    pass "missing log separator rejected"
else
    fail "missing log separator rejected (rc=$rc, got: $out)"
fi

# --- Separator: missing --- body --- rejected ---
cat > "$FIXTURES/0115-no-body-sep.erg" <<'EOF'
%erg 0.1
Title: No body separator
Created: 2026-01-01
Author: a

--- log ---
2026-01-01T10:00Z a created
EOF
out=$($ERG validate "$FIXTURES/0115-no-body-sep.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "missing '--- body ---'"; then
    pass "missing body separator rejected"
else
    fail "missing body separator rejected (rc=$rc, got: $out)"
fi

# --- Separator: duplicate --- log --- accepted (rule 12 relaxation, ticket 0116) ---
cat > "$FIXTURES/0116-dup-log-sep.erg" <<'EOF'
%erg 0.1
Title: Duplicate log separator
Created: 2026-01-01
Author: a

--- log ---
2026-01-01T10:00Z a created
--- log ---

--- body ---
EOF
out=$($ERG validate "$FIXTURES/0116-dup-log-sep.erg" 2>&1) && rc=0 || rc=$?
# Ticket 0116: rule 12 relaxed — duplicate separators are no longer
# errors. The first occurrence transitions sections; subsequent ones
# are body text (legitimate bodies quote the format literals).
if [ "$rc" -eq 0 ] && echo "$out" | grep -q "PASS"; then
    pass "duplicate log separator accepted (rule 12 relaxation)"
else
    fail "duplicate log separator accepted (rc=$rc, got: $out)"
fi

# --- Separator: duplicate --- body --- accepted (rule 12 relaxation, ticket 0116) ---
cat > "$FIXTURES/0117-dup-body-sep.erg" <<'EOF'
%erg 0.1
Title: Duplicate body separator
Created: 2026-01-01
Author: a

--- log ---

--- body ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0117-dup-body-sep.erg" 2>&1) && rc=0 || rc=$?
# Ticket 0116: rule 12 relaxed — duplicate separators are no longer
# errors. See above.
if [ "$rc" -eq 0 ] && echo "$out" | grep -q "PASS"; then
    pass "duplicate body separator accepted (rule 12 relaxation)"
else
    fail "duplicate body separator accepted (rc=$rc, got: $out)"
fi

# --- Log line: missing verb rejected ---
cat > "$FIXTURES/0118-log-missing-verb.erg" <<'EOF'
%erg 0.1
Title: Log missing verb
Created: 2026-01-01
Author: a

--- log ---
2026-01-01T10:00Z onlyactor

--- body ---
EOF
out=$($ERG validate "$FIXTURES/0118-log-missing-verb.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "malformed log line"; then
    pass "log line missing verb rejected"
else
    fail "log line missing verb rejected (rc=$rc, got: $out)"
fi

# --- Log line: timestamp with seconds rejected ---
cat > "$FIXTURES/0119-log-with-seconds.erg" <<'EOF'
%erg 0.1
Title: Log with seconds
Created: 2026-01-01
Author: a

--- log ---
2026-01-01T10:00:00Z a created

--- body ---
EOF
out=$($ERG validate "$FIXTURES/0119-log-with-seconds.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "malformed log line"; then
    pass "log line with seconds rejected"
else
    fail "log line with seconds rejected (rc=$rc, got: $out)"
fi

# --- Log line: bad timestamp format rejected ---
cat > "$FIXTURES/0120-log-bad-ts.erg" <<'EOF'
%erg 0.1
Title: Bad timestamp
Created: 2026-01-01
Author: a

--- log ---
2026-01-01 10:00 a created

--- body ---
EOF
out=$($ERG validate "$FIXTURES/0120-log-bad-ts.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "malformed log line"; then
    pass "log line with bad timestamp rejected"
else
    fail "log line with bad timestamp rejected (rc=$rc, got: $out)"
fi

# --- Unknown header: Priority rejected ---
cat > "$FIXTURES/0121-priority.erg" <<'EOF'
%erg 0.1
Title: Unknown header
Created: 2026-01-01
Author: a
Priority: high

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0121-priority.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "unknown header 'Priority'"; then
    pass "unknown header (Priority) rejected"
else
    fail "unknown header (Priority) rejected (rc=$rc, got: $out)"
fi

# --- Unknown header: X-Foo rejected ---
cat > "$FIXTURES/0122-x-foo.erg" <<'EOF'
%erg 0.1
Title: X-Foo header
Created: 2026-01-01
Author: a
X-Foo: bar

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0122-x-foo.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "unknown header 'X-Foo'"; then
    pass "unknown header (X-Foo) rejected"
else
    fail "unknown header (X-Foo) rejected (rc=$rc, got: $out)"
fi

# --- Tag: invalid value rejected (per-file validate) ---
cat > "$FIXTURES/0123-bad-tag.erg" <<'EOF'
%erg 0.1
Title: Bad tag
Created: 2026-01-01
Author: a
Tag: bogus

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0123-bad-tag.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "unknown Tag value 'bogus'"; then
    pass "Tag: bogus value rejected"
else
    fail "Tag: bogus value rejected (rc=$rc, got: $out)"
fi

# --- Blocked-by: unknown local ID rejected (isolated dir — no 9999 fixture) ---
mkdir -p "$FIXTURES/blocked-unknown"
cat > "$FIXTURES/blocked-unknown/0124-blocked-unknown.erg" <<'EOF'
%erg 0.1
Title: Blocked by unknown
Created: 2026-01-01
Author: a
Blocked-by: 9999

--- log ---
--- body ---
EOF
out=$($ERG validate "$FIXTURES/blocked-unknown/0124-blocked-unknown.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "references unknown ticket ID"; then
    pass "Blocked-by unknown local ID rejected"
else
    fail "Blocked-by unknown local ID rejected (rc=$rc, got: $out)"
fi

# --- Maximally complete ticket (kitchen sink) accepted ---
cat > "$FIXTURES/0125-kitchen-sink.erg" <<'EOF'
%erg 0.1
Title: Kitchen sink
Created: 2026-01-01
Author: claude
Closed: completed in PR #99
Blocked-by: 0001
Blocked-by: github.com/foo/bar#42
Tag: needs-human
Tag: deferred

--- log ---
2026-01-01T09:00Z claude created
2026-01-02T10:30Z user note added context
2026-01-03T11:15Z claude closed PR-99-merged

--- body ---
## Context
Free-form markdown.

## Exit criteria
- [ ] Done
EOF
if $ERG validate "$FIXTURES/0125-kitchen-sink.erg" >/dev/null 2>&1; then
    pass "kitchen-sink ticket (all optional headers) accepted"
else
    fail "kitchen-sink ticket (all optional headers) accepted"
fi

# --- Legacy %erg v1 magic line rejected with migrate hint ---
cat > "$FIXTURES/0200-legacy-v1.erg" <<'EOF'
%erg v1
Title: Legacy V1 ticket
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T09:00Z claude created

--- body ---
EOF
rc=0
out=$($ERG validate "$FIXTURES/0200-legacy-v1.erg" 2>&1) || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "erg migrate"; then
    pass "legacy %%erg v1 rejected with migrate hint"
else
    fail "legacy %%erg v1 rejected with migrate hint (rc=$rc, got: $out)"
fi

# --- Interior header blank: validate shouts on stderr but still exits 0 ---
cat > "$FIXTURES/0300-interior-blank.erg" <<'EOF'
%erg 0.1
Title: Interior blank
Created: 2026-01-01
Author: claude

Tag: needs-human

--- log ---
2026-01-01T09:00Z claude created
--- body ---
EOF
rc=0
stderr_out=$($ERG validate "$FIXTURES/0300-interior-blank.erg" 2>&1 >/dev/null) || rc=$?
if echo "$stderr_out" | grep -q "WARNING:.*blank line inside header block"; then
    pass "validate shouts WARNING on interior header blank"
else
    fail "validate shouts WARNING on interior header blank (got: $stderr_out)"
fi
if [ "$rc" -eq 0 ]; then
    pass "validate still exits 0 on interior header blank"
else
    fail "validate still exits 0 on interior header blank (rc=$rc)"
fi

# --- Interior header blank does not mask a bad Blocked-by ref (read tolerance) ---
cat > "$FIXTURES/0301-blank-masks-ref.erg" <<'EOF'
%erg 0.1
Title: Blank masks ref
Created: 2026-01-01
Author: claude

Blocked-by: 9999
--- log ---
2026-01-01T09:00Z claude created
--- body ---
EOF
rc=0
out=$($ERG validate "$FIXTURES/0301-blank-masks-ref.erg" 2>&1) || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "references unknown ticket ID"; then
    pass "Blocked-by below interior blank is now seen (rule 10 fires)"
else
    fail "Blocked-by below interior blank is now seen (rc=$rc, got: $out)"
fi

# --- Clean file: no interior-blank warning ---
cat > "$FIXTURES/0302-clean.erg" <<'EOF'
%erg 0.1
Title: Clean
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T09:00Z claude created
--- body ---
EOF
stderr_out=$($ERG validate "$FIXTURES/0302-clean.erg" 2>&1 >/dev/null)
if echo "$stderr_out" | grep -q "blank line inside header block"; then
    fail "clean file must not emit interior-blank warning (got: $stderr_out)"
else
    pass "clean file emits no interior-blank warning"
fi

# --- Rule 14: Title begins with a status word is rejected ---
cat > "$FIXTURES/0401-title-start.erg" <<'EOF'
%erg 0.1
Title: ready: demote claimed signal from blocker to marker
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T09:00Z claude created
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0401-title-start.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "status word 'ready'" && echo "$out" | grep -q "0401-title-start.erg:2:"; then
    pass "rule 14: Title starting with status word rejected (names word + line)"
else
    fail "rule 14: Title starting with status word rejected (rc=$rc, got: $out)"
fi

# --- Rule 14: Title ends with a status word is rejected ---
cat > "$FIXTURES/0402-title-end.erg" <<'EOF'
%erg 0.1
Title: make the work queue ready
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T09:00Z claude created
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0402-title-end.erg" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "status word 'ready'"; then
    pass "rule 14: Title ending with status word rejected"
else
    fail "rule 14: Title ending with status word rejected (rc=$rc, got: $out)"
fi

# --- Rule 14: status word mid-title is accepted ---
cat > "$FIXTURES/0403-title-mid.erg" <<'EOF'
%erg 0.1
Title: respect the open flag in the parser
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T09:00Z claude created
--- body ---
EOF
if $ERG validate "$FIXTURES/0403-title-mid.erg" >/dev/null 2>&1; then
    pass "rule 14: status word mid-title accepted"
else
    fail "rule 14: status word mid-title accepted"
fi

# --- Rule 14: closed ticket grandfathered (status word at edge tolerated) ---
cat > "$FIXTURES/0404-title-closed.erg" <<'EOF'
%erg 0.1
Title: ready: demote claimed signal from blocker to marker
Created: 2026-01-01
Author: claude
Closed: superseded

--- log ---
2026-01-01T09:00Z claude created
2026-01-01T10:00Z claude closed — superseded
--- body ---
EOF
if $ERG validate "$FIXTURES/0404-title-closed.erg" >/dev/null 2>&1; then
    pass "rule 14: closed ticket grandfathered"
else
    fail "rule 14: closed ticket grandfathered"
fi

echo "validate: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
