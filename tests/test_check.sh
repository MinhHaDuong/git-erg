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

# --- exit-code coherence (ticket 0207): a violation is exit 1, never 2 ---
# 1 is reserved for hard failures (violations); 2 means "init skipped a local
# edit". check must never emit 2, so the shared table stays unambiguous.
dup_rc=0; $ERG check "$FIXTURES/dup" >/dev/null 2>&1 || dup_rc=$?
if [ "$dup_rc" -eq 1 ]; then
    pass "violation exits 1 (not 2 -- shared exit-code table is unambiguous)"
else
    fail "violation should exit 1, got $dup_rc"
fi

# --- Cross-directory duplicate IDs fail ---
mkdir -p "$FIXTURES/xdup/closed"
cat > "$FIXTURES/xdup/0001-one.erg" <<'EOF'
%erg 0.1
Title: One
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
cp "$FIXTURES/xdup/0001-one.erg" "$FIXTURES/xdup/closed/0001-one.erg"
if $ERG check "$FIXTURES/xdup" 2>&1 | grep -q 'duplicate'; then
    pass "cross-directory duplicate ID rejected"
else
    fail "cross-directory duplicate ID rejected"
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
cat > "$FIXTURES/cross/0002-refs-into-archive.erg" <<'EOF'
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

# --- Folder closure: open ticket in closed/ is an error (0241) ---
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
if echo "$out" | grep -q "VIOLATION.*open ticket in closed"; then
    pass "open ticket in closed/ is a violation"
else
    fail "open ticket in closed/ is a violation (got: $out)"
fi
if [ $rc -eq 1 ]; then
    pass "folder closure violation exits 1"
else
    fail "folder closure violation exits 1 (got rc=$rc)"
fi

# --- Folder closure: closed ticket at top level is an error (0241) ---
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
rc=0; out=$($ERG check "$FIXTURES/closure2" 2>&1) || rc=$?
if echo "$out" | grep -q "VIOLATION.*closed ticket not in closed"; then
    pass "closed-but-unarchived ticket is a violation"
else
    fail "closed-but-unarchived ticket is a violation (got: $out)"
fi
if [ $rc -eq 1 ]; then
    pass "closed-but-unarchived exits 1"
else
    fail "closed-but-unarchived exits 1 (got rc=$rc)"
fi

# --- Folder closure: basename-only closed ticket at top level is an error (0256) ---
# A slug truncation landing on "-closed" made ticket 0255 vanish from erg list
# and erg ready with no warning. check must catch the hand-created case too.
mkdir -p "$FIXTURES/closure3"
cat > "$FIXTURES/closure3/0001-work-closed.erg" <<'EOF'
%erg 0.1
Title: Open work with a closed-looking name
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
rc=0; out=$($ERG check "$FIXTURES/closure3" 2>&1) || rc=$?
if echo "$out" | grep -q "VIOLATION.*filename reads as closed"; then
    pass "basename-only closed ticket is a violation"
else
    fail "basename-only closed ticket is a violation (got: $out)"
fi
if [ $rc -eq 1 ]; then
    pass "basename-only closure violation exits 1 (error, not a warning)"
else
    fail "basename-only closure violation exits 1 (got rc=$rc)"
fi

# --- Negative control: disclosed/enclosed basenames are NOT closed (0256) ---
# Guards against a strings.Contains(name, "closed") reimplementation, which
# would pass the positive case above (spec: "Rules out disclosed, enclosed").
mkdir -p "$FIXTURES/closure4"
cat > "$FIXTURES/closure4/0001-not-disclosed.erg" <<'EOF'
%erg 0.1
Title: Not disclosed
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
cat > "$FIXTURES/closure4/0002-fully-enclosed.erg" <<'EOF'
%erg 0.1
Title: Fully enclosed
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
if $ERG check "$FIXTURES/closure4" >/dev/null 2>&1; then
    pass "disclosed/enclosed basenames are not flagged as closed"
else
    fail "disclosed/enclosed basenames are not flagged as closed"
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

# --- Ticketless store: the dir-based scans still run ---
# corpusWarnings used to return nil on an empty corpus, which silenced four
# scans that need no ticket to be meaningful (stray Go source, encoding,
# interior header blanks, asset drift). The population that silenced was the
# fresh adopter who has run `erg init` and not filed a ticket yet -- exactly
# the one those scans are for. Two arms, because a warning that never fires
# and a store that is genuinely clean print the same thing.
mkdir -p "$FIXTURES/ticketless-stray"
touch "$FIXTURES/ticketless-stray/fake.go"
rc=0; out=$($ERG check "$FIXTURES/ticketless-stray" 2>&1) || rc=$?
if echo "$out" | grep -qF "WARN: Go source files found in"; then
    pass "ticketless store: stray Go source still warns"
else
    fail "ticketless store: stray Go source drew no warning (got: $out)"
fi
if [ $rc -eq 0 ]; then
    pass "ticketless store: a warning is not a verdict, exit stays 0"
else
    fail "ticketless store: warning changed the exit code (rc=$rc)"
fi

# Positive control for the arm above: a ticketless store with nothing wrong
# must stay silent. Without it, an implementation that warns unconditionally
# would pass the first arm and nothing would notice.
mkdir -p "$FIXTURES/ticketless-clean"
rc=0; out=$($ERG check "$FIXTURES/ticketless-clean" 2>&1) || rc=$?
if [ $rc -eq 0 ] && ! echo "$out" | grep -q "WARN"; then
    pass "ticketless store: a clean one warns about nothing"
else
    fail "ticketless store: clean store warned anyway (rc=$rc, got: $out)"
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
# Use a stale Blocked-by (open ticket referencing a closed one) as the warning
# fixture now that folderClosure produces errors rather than warnings (0241).
mkdir -p "$FIXTURES/warn1/closed"
cat > "$FIXTURES/warn1/closed/0001-closed-ref.erg" <<'EOF'
%erg 0.1
Title: Closed reference target
Created: 2026-01-01
Author: a
Closed: done

--- log ---
--- body ---
EOF
cat > "$FIXTURES/warn1/0002-stale-blocker.erg" <<'EOF'
%erg 0.1
Title: Stale blocker ref
Created: 2026-01-01
Author: a
Blocked-by: 0001

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
# Two stale Blocked-by warnings: two open tickets each referencing a closed one.
mkdir -p "$FIXTURES/warn2/closed"
cat > "$FIXTURES/warn2/closed/0001-closed-ref-pl.erg" <<'EOF'
%erg 0.1
Title: Closed reference target
Created: 2026-01-01
Author: a
Closed: done

--- log ---
--- body ---
EOF
cat > "$FIXTURES/warn2/0002-stale-a.erg" <<'EOF'
%erg 0.1
Title: Stale ref A
Created: 2026-01-01
Author: a
Blocked-by: 0001

--- log ---
--- body ---
EOF
cat > "$FIXTURES/warn2/0003-stale-b.erg" <<'EOF'
%erg 0.1
Title: Stale ref B
Created: 2026-01-01
Author: a
Blocked-by: 0001

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
Label: bogus-label

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

Label: needs-human

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
# Separate fixture dir to avoid duplicate ID with the open fixture above.
# Put the closed ticket in closed/ to satisfy folder closure (0241).
mkdir -p "$FIXTURES/title-gf/closed"
cat > "$FIXTURES/title-gf/closed/0001-bad.erg" <<'EOF'
%erg 0.1
Title: open the config reader to subdir overrides
Created: 2026-01-01
Author: a
Closed: superseded

--- log ---
2026-01-01T10:00Z a closed — superseded
--- body ---
EOF
out=$($ERG check "$FIXTURES/title-gf" 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 0 ]; then
    pass "rule 14: check grandfathers closed ticket"
else
    fail "rule 14: check grandfathers closed ticket (rc=$rc, got: $out)"
fi

# live-corpus check moved to: make validate

# unknown flag rejection (ticket 0178)
    out=$($ERG check --bogus 2>&1) && rc=0 || rc=$?
    if [ "$rc" -ne 0 ] && echo "$out" | grep -q "unknown flag"; then
        pass "unknown flag rejected with usage message"
    else
        fail "unknown flag not rejected (rc=$rc, got: $out)"
    fi

# --- asset drift warning (ticket 0212) ---
# Requires a usable stamp for the asset; a stamp != embedded means the binary
# was upgraded since the last init. Non-fatal (exit 0).
#
# The fixture stamps are 64 hex digits, and the width is load-bearing since
# ticket 0292: a stamp that is not a SHA-256 is read as no stamp at all, so the
# six-digit placeholders these fixtures used to carry would route the asset to
# the stampless compare and this arm would assert a warning that cannot fire.
# What the arm is about is a WELL-FORMED stamp that disagrees with the embedded
# hash, which is what a hash of nothing gives it.
DRIFTDIR="$FIXTURES/drift"
mkdir -p "$DRIFTDIR"
cat > "$DRIFTDIR/9001-x.erg" <<'EOF'
%erg 0.1
Title: X
Created: 2026-01-01
Author: t

--- log ---
--- body ---
EOF
printf '# erg provenance manifest -- do not edit\nrev: x\ndate: y\nassets:\n  .ergrc sha256:0000000000000000000000000000000000000000000000000000000000000000\n  AGENTS.md sha256:1111111111111111111111111111111111111111111111111111111111111111\n' > "$DRIFTDIR/.erg-assets"
rc=0; out=$($ERG check "$DRIFTDIR" 2>&1) || rc=$?
if [ "$rc" -eq 0 ] && echo "$out" | grep -q "differs from the .erg-assets stamp"; then
    pass "drift: stamp != embedded emits a non-fatal warning"
else
    fail "drift: expected a non-fatal drift warning (rc=$rc, got: $out)"
fi
# NOTE the 'date: y' above: it is an UNPARSEABLE stamp, and the assertion just
# made is that it still takes the upgrade-direction message. "y" sorts above
# every digit, so a direction check that compared it lexically without a shape
# guard would call this repo a rollback (ticket 0279).

# --- asset drift DIRECTION (ticket 0279) ---
# Same drift condition, opposite direction: a stamp this binary predates. The
# warning must not claim an upgrade, and must name 'erg update' -- running
# 'erg init' here would REVERT the deployed assets, not refresh them.
ROLLDIR="$FIXTURES/rollback"
mkdir -p "$ROLLDIR"
cp "$DRIFTDIR/9001-x.erg" "$ROLLDIR/"
printf '# erg provenance manifest -- do not edit\nrev: x\ndate: 2099-01-01T00:00:00Z\nassets:\n  .ergrc sha256:0000000000000000000000000000000000000000000000000000000000000000\n  AGENTS.md sha256:1111111111111111111111111111111111111111111111111111111111111111\n' > "$ROLLDIR/.erg-assets"
rc=0; out=$($ERG check "$ROLLDIR" 2>&1) || rc=$?
if [ "$rc" -eq 0 ] && echo "$out" | grep -q "run 'erg update' first"; then
    pass "drift: stamp newer than the binary names 'erg update'"
else
    fail "drift: expected the rollback remedy (rc=$rc, got: $out)"
fi
if echo "$out" | grep -q "upgraded"; then
    fail "drift: the binary is the older side, yet the warning claims an upgrade (got: $out)"
else
    pass "drift: stamp newer than the binary never claims 'upgraded'"
fi

# No manifest -> no drift warning (derisque: no stamp-relative claim without a
# stamp). This fixture writes no .ergrc and no AGENTS.md at all, so it is
# ALSO the "absent, not diverged" control for the stampless report below:
# nothing on disk to compare, so neither channel may speak.
NODRIFT="$FIXTURES/nodrift"
mkdir -p "$NODRIFT"
cp "$DRIFTDIR/9001-x.erg" "$NODRIFT/"
out=$($ERG check "$NODRIFT" 2>&1 || true)
if echo "$out" | grep -q "differs from the .erg-assets stamp"; then
    fail "drift: warned without a manifest (should not)"
else
    pass "drift: no manifest -> no drift warning"
fi
if echo "$out" | grep -qF "no .erg-assets stamp"; then
    fail "stampless: reported a store with no assets on disk at all (should not)"
else
    pass "stampless: no manifest and no assets -> silent"
fi

# --- stampless store (ticket 0283) ---
# No manifest AND an on-disk asset that differs from the embedded copy. The
# difference is real but unattributable, and before 0283 every channel went
# silent about it. erg check must now say so and name the remedy.
STAMPLESS="$FIXTURES/stampless"
mkdir -p "$STAMPLESS"
cp "$DRIFTDIR/9001-x.erg" "$STAMPLESS/"
printf '# an .ergrc that is not what this binary embeds\nlabels = whatever\n' > "$STAMPLESS/.ergrc"
rc=0; out=$($ERG check "$STAMPLESS" 2>&1) || rc=$?
if [ "$rc" -eq 0 ] && echo "$out" | grep -qF "no .erg-assets stamp"; then
    pass "stampless: diverged asset with no manifest is reported (non-fatal)"
else
    fail "stampless: expected a non-fatal stampless report (rc=$rc, got: $out)"
fi
# A bare "erg init" grep would also be satisfied by assetRollbackSignal, which
# ends "...then 'erg init'". Assert the stampless advice specifically: it must
# name a route the reader can actually take. Two of them are now asserted
# together because the constant is a cross-version literal and ticket 0292's
# correction could only be APPENDED to it -- the historical prefix still says
# "its git history can" and "stamps it as if shipped", and the tail after it
# says where each of those falls down. A test greping only the prefix would go
# on passing if the correction were dropped.
if echo "$out" | grep -qF "its git history can" && echo "$out" | grep -qF "stamps it as if shipped"; then
    pass "stampless: the historical prefix of the cross-version literal is intact"
else
    fail "stampless: the shipped prefix was reworded, which breaks erg update's grep (got: $out)"
fi
if echo "$out" | grep -qF "erg init --show NAME" && echo "$out" | grep -qF "no longer stamps a file it preserved"; then
    pass "stampless: the report names the erg command that shows the shipped copy"
else
    fail "stampless: the report must name an erg-side comparison, not only git (got: $out)"
fi
# Guard: no stamp exists here, so no stamp-relative claim may be made.
if echo "$out" | grep -qF "differs from the .erg-assets stamp"; then
    fail "stampless: claimed a stamp comparison with no stamp on disk (got: $out)"
else
    pass "stampless: makes no stamp-relative claim"
fi

# Positive control for the gate: same absence of a manifest, but the on-disk
# assets match the embedded copy exactly -> silent. Without this arm, an
# implementation that reports on "no manifest" alone passes the case above and
# proves nothing about gating on real divergence. Built by letting `erg init`
# lay down this binary's own assets, then deleting only the stamp it wrote.
STAMPMATCH="$FIXTURES/stampmatch"
mkdir -p "$STAMPMATCH/tickets"
touch "$STAMPMATCH/tickets/erg"
cp "$DRIFTDIR/9001-x.erg" "$STAMPMATCH/tickets/"
$ERG init "$STAMPMATCH" >/dev/null 2>&1
rm -f "$STAMPMATCH/tickets/.erg-assets"
# Guard: the control is only meaningful if init actually laid the assets down.
# Without them this would silently degenerate into the absent-asset case above.
if [ ! -f "$STAMPMATCH/tickets/.ergrc" ] || [ -f "$STAMPMATCH/tickets/.erg-assets" ]; then
    fail "stampless control: fixture is not a stampless store with assets present (test would be vacuous)"
else
    rc=0; out=$($ERG check "$STAMPMATCH/tickets" 2>&1) || rc=$?
    if [ "$rc" -eq 0 ] && ! echo "$out" | grep -qF "no .erg-assets stamp"; then
        pass "stampless: assets matching the embedded copy exactly stay silent"
    else
        fail "stampless: an exact match must not be nagged (rc=$rc, got: $out)"
    fi
fi

# --- partial manifest (ticket 0292, defect 3) ---
# 0283's silence, one level down. The stamped branch skipped any asset with no
# entry and did not fall back to the stampless compare, while parseManifest
# returns non-nil as soon as ONE line parses -- so a manifest stamping .ergrc
# but not AGENTS.md silenced AGENTS.md divergence on every channel: not
# drift-warned (no stamp for it), not stampless-warned (the manifest exists).
# The gate had to become per ASSET, not per store.
#
# The shape is not contrived: it is exactly what `erg init` itself writes once
# it stops stamping a file it preserved.
PARTIAL="$FIXTURES/partial"
mkdir -p "$PARTIAL/tickets"
touch "$PARTIAL/tickets/erg"
cp "$DRIFTDIR/9001-x.erg" "$PARTIAL/tickets/"
$ERG init "$PARTIAL" >/dev/null 2>&1
# Drop AGENTS.md's line from the manifest init just wrote, then diverge it.
grep -v "^  AGENTS\.md sha256:" "$PARTIAL/tickets/.erg-assets" > "$PARTIAL/manifest.tmp"
mv "$PARTIAL/manifest.tmp" "$PARTIAL/tickets/.erg-assets"
printf '# an AGENTS.md that is not what this binary embeds\n' > "$PARTIAL/tickets/AGENTS.md"
# Guard: the arm is about the STAMPED branch, so the manifest must still parse
# and must still stamp .ergrc. Without this the fixture degenerates into the
# stampless case above and the assertion proves nothing about the per-asset gate.
if ! grep -q "^  \.ergrc sha256:" "$PARTIAL/tickets/.erg-assets" ||
    grep -q "^  AGENTS\.md sha256:" "$PARTIAL/tickets/.erg-assets"; then
    fail "partial: fixture is not a partially-stamped manifest (test would be vacuous)"
else
    rc=0; out=$($ERG check "$PARTIAL/tickets" 2>&1) || rc=$?
    if [ "$rc" -eq 0 ] && echo "$out" | grep -qF "AGENTS.md: no .erg-assets stamp"; then
        pass "partial: an asset missing from an otherwise-valid manifest is reported"
    else
        fail "partial: the stamped branch swallowed the unstamped asset (rc=$rc, got: $out)"
    fi
fi

# Sibling control, same fixture shape: restore the matching content and the
# report goes quiet. Without it, an implementation reporting on "unstamped"
# rather than on "unstamped AND diverged" passes the arm above -- and would nag
# every store whose manifest predates an asset being added to the list.
PARTIALOK="$FIXTURES/partial-match"
mkdir -p "$PARTIALOK/tickets"
touch "$PARTIALOK/tickets/erg"
cp "$DRIFTDIR/9001-x.erg" "$PARTIALOK/tickets/"
$ERG init "$PARTIALOK" >/dev/null 2>&1
grep -v "^  AGENTS\.md sha256:" "$PARTIALOK/tickets/.erg-assets" > "$PARTIALOK/manifest.tmp"
mv "$PARTIALOK/manifest.tmp" "$PARTIALOK/tickets/.erg-assets"
if ! grep -q "^  \.ergrc sha256:" "$PARTIALOK/tickets/.erg-assets" ||
    [ ! -f "$PARTIALOK/tickets/AGENTS.md" ]; then
    fail "partial control: fixture is not a partially-stamped store with assets present (test would be vacuous)"
else
    rc=0; out=$($ERG check "$PARTIALOK/tickets" 2>&1) || rc=$?
    if [ "$rc" -eq 0 ] && ! echo "$out" | grep -qF "no .erg-assets stamp"; then
        pass "partial: an unstamped asset matching the embedded copy stays silent"
    else
        fail "partial: an exact match must not be nagged for lacking a stamp (rc=$rc, got: $out)"
    fi
fi

# --- vendored erg-github drift (ticket 0282) ---
# tickets/erg-github is vendored: a committed POSIX-sh helper that travels with
# the clone, in none of erg's asset lists, and therefore -- until 0282 -- on no
# staleness channel at all. An adopter carrying a year-old copy was told
# nothing, ever, which is how 0255's cmd_verify() fix failed to reach the repo
# where that bug actually fired.
#
# Asserted on message CONTENT, never on the exit code: these are warnings, so
# the exit code is 0 in both the fixed and the unfixed world and reading it
# would prove nothing. The noisy arm comes FIRST; the two silent arms only mean
# something once the report has been seen to fire.
VENDORED="$FIXTURES/vendored"
mkdir -p "$VENDORED"
cp "$DRIFTDIR/9001-x.erg" "$VENDORED/"
printf '#!/bin/sh\n# an old vendored erg-github, predating the 0255 fix\nexit 0\n' > "$VENDORED/erg-github"
rc=0; out=$($ERG check "$VENDORED" 2>&1) || rc=$?
if [ "$rc" -eq 0 ] && echo "$out" | grep -qF "it is vendored, so erg never writes it"; then
    pass "vendored: a stale erg-github is reported (non-fatal)"
else
    fail "vendored: expected a non-fatal vendored-drift report (rc=$rc, got: $out)"
fi
if echo "$out" | grep -qF "erg-github" && echo "$out" | grep -qF "re-vendor it by hand"; then
    pass "vendored: the report names the file and a route erg cannot walk for you"
else
    fail "vendored: the report must name the file and the manual remedy (got: $out)"
fi
# erg never wrote this file, so no stamp-relative claim may be made about it,
# in either direction -- and 'erg init' must not be prescribed: it does not
# touch a vendored path, so it would report success and change nothing.
if echo "$out" | grep -qF "differs from the .erg-assets stamp" || echo "$out" | grep -qF "no .erg-assets stamp"; then
    fail "vendored: claimed a stamp comparison for a file erg never wrote (got: $out)"
else
    pass "vendored: makes no stamp-relative claim"
fi

# Same stale copy, but in a STAMPED store. The managed-asset branch returns
# early only when there is no manifest, so an implementation that folded the
# vendored compare into that branch would report here and go silent above (or
# the reverse). The Go test covers both; this is the CLI layer's parity arm.
VENDOREDSTAMP="$FIXTURES/vendored-stamped"
mkdir -p "$VENDOREDSTAMP/tickets"
touch "$VENDOREDSTAMP/tickets/erg"
cp "$DRIFTDIR/9001-x.erg" "$VENDOREDSTAMP/tickets/"
$ERG init "$VENDOREDSTAMP" >/dev/null 2>&1
printf '#!/bin/sh\n# an old vendored erg-github, predating the 0255 fix\nexit 0\n' > "$VENDOREDSTAMP/tickets/erg-github"
# Guard: the arm is only about a STAMPED store if init actually stamped one.
if ! grep -qE "sha256:[0-9a-f]{64}" "$VENDOREDSTAMP/tickets/.erg-assets" 2>/dev/null; then
    fail "vendored: stamped fixture has no manifest (test would duplicate the stampless arm)"
else
    rc=0; out=$($ERG check "$VENDOREDSTAMP/tickets" 2>&1) || rc=$?
    if [ "$rc" -eq 0 ] && echo "$out" | grep -qF "it is vendored, so erg never writes it"; then
        pass "vendored: a stale erg-github is reported in a stamped store too"
    else
        fail "vendored: the stamped branch swallowed the vendored report (rc=$rc, got: $out)"
    fi
fi

# Silent arm 1: the copy this binary ships. src/go/assets/erg-github IS the
# embedded reference (the self-coherence guard pins it to tickets/erg-github),
# so a store holding it byte-for-byte has nothing to report.
VENDOREDOK="$FIXTURES/vendored-current"
mkdir -p "$VENDOREDOK"
cp "$DRIFTDIR/9001-x.erg" "$VENDOREDOK/"
cp src/go/assets/erg-github "$VENDOREDOK/erg-github"
# Guard: without a real, non-empty copy this arm degenerates into the absent
# case below and would pass while proving nothing about the equality gate.
if [ ! -s "$VENDOREDOK/erg-github" ]; then
    fail "vendored control: fixture has no erg-github copy (test would be vacuous)"
else
    rc=0; out=$($ERG check "$VENDOREDOK" 2>&1) || rc=$?
    if [ "$rc" -eq 0 ] && ! echo "$out" | grep -qF "it is vendored, so erg never writes it"; then
        pass "vendored: a copy identical to the shipped one stays silent"
    else
        fail "vendored: an exact match must not be nagged (rc=$rc, got: $out)"
    fi
fi

# Silent arm 2: no erg-github at all. This is the invariant that keeps the
# forge layer OPTIONAL -- erg core is offline and forge-blind, and a repo that
# declined the forge helper must never be nagged into adopting it. $NODRIFT is
# a store with no assets on disk whatsoever, so it is exactly that case.
out=$($ERG check "$NODRIFT" 2>&1 || true)
if echo "$out" | grep -qF "it is vendored, so erg never writes it"; then
    fail "vendored: reported a store that carries no erg-github at all (should not)"
else
    pass "vendored: no erg-github on disk -> silent (forge layer stays optional)"
fi

# Matching manifest -> no drift warning (exit criterion: no warn when all
# assets match the embedded version). `erg init` stamps THIS binary's own
# embedded assets, so the manifest matches by construction. init needs a
# tickets/erg present (it stamps relative to the binary location), hence the
# touch -- without it init fails and writes no manifest, which would make this
# test silently degenerate into the no-manifest case above.
MATCHDIR="$FIXTURES/match"
mkdir -p "$MATCHDIR/tickets"
touch "$MATCHDIR/tickets/erg"
cp "$DRIFTDIR/9001-x.erg" "$MATCHDIR/tickets/"
$ERG init "$MATCHDIR" >/dev/null 2>&1
# Guard: the test is only meaningful if init actually wrote a stamped manifest.
if ! grep -q "sha256:[0-9a-f]" "$MATCHDIR/tickets/.erg-assets" 2>/dev/null; then
    fail "drift: matching fixture: init wrote no stamped manifest (test would be vacuous)"
else
    rc=0; out=$($ERG check "$MATCHDIR/tickets" 2>&1) || rc=$?
    if [ "$rc" -eq 0 ] && ! echo "$out" | grep -q "differs from the .erg-assets stamp"; then
        pass "drift: matching manifest -> no drift warning"
    else
        fail "drift: matching stamps should not warn (rc=$rc, got: $out)"
    fi
fi


# --- Help text: the asset-drift bullet names BOTH directions ---
# cmdCheck calls assetDriftWarnings, which is direction-aware (assetDriftSignal
# vs assetRollbackSignal). Help text that names only the upgrade direction
# prescribes 'erg init' for a rollback -- the command that PERFORMS the revert,
# which is exactly the false-direction claim ticket 0279 removes from the code.
# Static help is a place that claim can be reintroduced; this test closes it.
HELPOUT=$($ERG check --help 2>&1) || true
if echo "$HELPOUT" | grep -q "run 'erg init'" && echo "$HELPOUT" | grep -q "run 'erg update' first"; then
    pass "help: asset-drift bullet names both directions"
else
    fail "help: asset-drift bullet documents only one direction (got: $HELPOUT)"
fi

echo "check: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
