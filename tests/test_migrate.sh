#!/bin/sh
# Integration tests for: erg migrate
set -eu

ERG="${ERG_BIN:-build/erg}"
FIXTURES=$(mktemp -d)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

trap 'rm -rf "$FIXTURES"' EXIT

echo "=== erg migrate ==="

# --- Status: closed → Closed: header ---
cat > "$FIXTURES/0001-was-closed.erg" <<'EOF'
%erg v1
Title: Was closed
Status: closed
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created
2026-01-02T10:00Z claude status closed — done

--- body ---
Body.
EOF
$ERG migrate "$FIXTURES" >/dev/null
if grep -q "^Status:" "$FIXTURES/0001-was-closed.erg"; then
    fail "Status: closed → header removed"
else
    pass "Status: closed → header removed"
fi
if grep -q "^Closed: migrated from Status: closed$" "$FIXTURES/0001-was-closed.erg"; then
    pass "Status: closed → Closed: header added"
else
    fail "Status: closed → Closed: header added"
fi

# --- Status: open → Status: line removed, no Closed: added ---
cat > "$FIXTURES/0002-was-open.erg" <<'EOF'
%erg v1
Title: Was open
Status: open
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Body.
EOF
$ERG migrate "$FIXTURES" >/dev/null
if grep -q "^Status:" "$FIXTURES/0002-was-open.erg"; then
    fail "Status: open → header removed"
else
    pass "Status: open → header removed"
fi
if grep -q "^Closed:" "$FIXTURES/0002-was-open.erg"; then
    fail "Status: open → no Closed: added"
else
    pass "Status: open → no Closed: added"
fi

# --- Status: doing → header removed, no Closed: added ---
cat > "$FIXTURES/0003-was-doing.erg" <<'EOF'
%erg v1
Title: Was doing
Status: doing
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
$ERG migrate "$FIXTURES" >/dev/null
if grep -q "^Status:" "$FIXTURES/0003-was-doing.erg" || grep -q "^Closed:" "$FIXTURES/0003-was-doing.erg"; then
    fail "Status: doing → header removed, no Closed: added"
else
    pass "Status: doing → header removed, no Closed: added"
fi

# --- Status: pending → header removed, no Closed: added ---
cat > "$FIXTURES/0004-was-pending.erg" <<'EOF'
%erg v1
Title: Was pending
Status: pending
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
$ERG migrate "$FIXTURES" >/dev/null
if grep -q "^Status:" "$FIXTURES/0004-was-pending.erg" || grep -q "^Closed:" "$FIXTURES/0004-was-pending.erg"; then
    fail "Status: pending → header removed, no Closed: added"
else
    pass "Status: pending → header removed, no Closed: added"
fi

# --- Idempotent: running again is a no-op ---
cp "$FIXTURES/0001-was-closed.erg" "$FIXTURES/snapshot-0001"
cp "$FIXTURES/0002-was-open.erg" "$FIXTURES/snapshot-0002"
$ERG migrate "$FIXTURES" >/dev/null
if cmp -s "$FIXTURES/0001-was-closed.erg" "$FIXTURES/snapshot-0001" \
    && cmp -s "$FIXTURES/0002-was-open.erg" "$FIXTURES/snapshot-0002"; then
    pass "migrate is idempotent"
else
    fail "migrate is idempotent"
fi
rm -f "$FIXTURES/snapshot-0001" "$FIXTURES/snapshot-0002"

# --- File without Status: is unchanged (no-op, counted as already clean) ---
cat > "$FIXTURES/0005-already-clean.erg" <<'EOF'
%erg 0.1
Title: Already clean
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
cp "$FIXTURES/0005-already-clean.erg" "$FIXTURES/snapshot-0005"
out=$($ERG migrate "$FIXTURES")
if cmp -s "$FIXTURES/0005-already-clean.erg" "$FIXTURES/snapshot-0005"; then
    pass "no-Status file untouched"
else
    fail "no-Status file untouched"
fi
rm -f "$FIXTURES/snapshot-0005"
if echo "$out" | grep -qE "already clean: [1-9]"; then
    pass "summary reports already-clean count"
else
    fail "summary reports already-clean count (got: $out)"
fi

# --- A formerly-Status ticket passes check cleanly after migration ---
# Run check on an isolated dir holding exactly one freshly-migrated ticket, so
# the result is load-bearing: a leftover Status: header would make the
# validator emit "'Status:' header is no longer part of %erg 0.1" and exit 1.
# (Running against $FIXTURES would mask this behind duplicate-ID errors.)
CLEANDIR=$(mktemp -d)
cat > "$CLEANDIR/0001-was-status.erg" <<'EOF'
%erg v1
Title: Legacy ticket carrying a status header
Created: 2026-01-01
Author: claude
Status: open

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
$ERG migrate "$CLEANDIR" >/dev/null 2>&1
out=$($ERG check "$CLEANDIR" 2>&1); rc=$?
if [ "$rc" -eq 0 ] && ! echo "$out" | grep -qi "Status:"; then
    pass "formerly-Status ticket passes check after migration"
else
    fail "formerly-Status ticket passes check after migration (rc=$rc, out: $out)"
fi
rm -rf "$CLEANDIR"

# --- Tags: → Tag: rewrite (preamble-bounded, value preserved) ---
cat > "$FIXTURES/0010-tags-legacy.erg" <<'EOF'
%erg v1
Title: Legacy Tags
Created: 2026-01-01
Author: claude
Tags: needs-human
Tags: deferred

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Body code block referencing Tags: should stay literal.
EOF
out=$($ERG migrate "$FIXTURES" 2>&1)
if grep -q "^Tags:" "$FIXTURES/0010-tags-legacy.erg"; then
    fail "Tags: legacy → preamble rewritten"
else
    pass "Tags: legacy → preamble rewritten"
fi
if grep -q "^Tag: needs-human$" "$FIXTURES/0010-tags-legacy.erg" \
    && grep -q "^Tag: deferred$" "$FIXTURES/0010-tags-legacy.erg"; then
    pass "Tags: → Tag: values preserved (round-trip)"
else
    fail "Tags: → Tag: values preserved (round-trip)"
fi
# Body code block must NOT be rewritten — the literal "Tags:" stays.
if grep -q "Body code block referencing Tags: should stay literal." "$FIXTURES/0010-tags-legacy.erg"; then
    pass "Tags: in body code block preserved verbatim"
else
    fail "Tags: in body code block preserved verbatim"
fi
if echo "$out" | grep -qE "Tags: → Tag: rewrite: [1-9]"; then
    pass "summary reports Tags→Tag rewrite count"
else
    fail "summary reports Tags→Tag rewrite count (got: $out)"
fi

# --- Tags: → Tag: rewrite is idempotent ---
cp "$FIXTURES/0010-tags-legacy.erg" "$FIXTURES/snapshot-0010"
$ERG migrate "$FIXTURES" >/dev/null
if cmp -s "$FIXTURES/0010-tags-legacy.erg" "$FIXTURES/snapshot-0010"; then
    pass "Tags→Tag rewrite is idempotent"
else
    fail "Tags→Tag rewrite is idempotent"
fi
rm -f "$FIXTURES/snapshot-0010"

# --- erg validate rejects legacy Tags: lines with migration hint ---
cat > "$FIXTURES/0011-still-tags.erg" <<'EOF'
%erg v1
Title: Still has Tags
Created: 2026-01-01
Author: claude
Tags: needs-human

--- log ---
2026-01-01T10:00Z claude created
--- body ---
EOF
out=$($ERG validate "$FIXTURES/0011-still-tags.erg" 2>&1 || true)
if echo "$out" | grep -q "renamed to 'Tag:'"; then
    pass "validate rejects legacy Tags: with migration hint"
else
    fail "validate rejects legacy Tags: with migration hint (got: $out)"
fi

# --- erg validate rejects Status: lines (migrate is the only tolerant cmd) ---
cat > "$FIXTURES/0009-still-status.erg" <<'EOF'
%erg v1
Title: Still has Status
Status: open
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created
--- body ---
EOF
if $ERG validate "$FIXTURES/0009-still-status.erg" >/dev/null 2>&1; then
    fail "validate rejects unmigrated Status:"
else
    pass "validate rejects unmigrated Status:"
fi

# --- Interior header blank: swept across the corpus, reported, idempotent ---
printf '%%erg 0.1\nTitle: Interior blank\nCreated: 2026-01-01\nAuthor: claude\n\nTag: needs-human\nBlocked-by: 0001\n\n--- log ---\n2026-01-01T10:00Z claude created\n\n--- body ---\nBody.\n' > "$FIXTURES/0020-interior-blank.erg"
out=$($ERG migrate "$FIXTURES" 2>&1)
# The blank between Author: and Tag: must be gone; the terminating blank
# before --- log --- must survive (Tag/Blocked-by now sit in the header block).
if awk '/^Author: claude$/{getline n; if(n==""){found=1}} END{exit !found}' "$FIXTURES/0020-interior-blank.erg"; then
    fail "migrate sweep: interior header blank removed"
else
    pass "migrate sweep: interior header blank removed"
fi
if grep -q "^Tag: needs-human$" "$FIXTURES/0020-interior-blank.erg" \
    && grep -q "^Blocked-by: 0001$" "$FIXTURES/0020-interior-blank.erg"; then
    pass "migrate sweep: headers below the blank preserved"
else
    fail "migrate sweep: headers below the blank preserved"
fi
if echo "$out" | grep -qE "interior header blank sweep: [1-9]"; then
    pass "summary reports interior header blank sweep count"
else
    fail "summary reports interior header blank sweep count (got: $out)"
fi
cp "$FIXTURES/0020-interior-blank.erg" "$FIXTURES/snapshot-0020"
$ERG migrate "$FIXTURES" >/dev/null
if cmp -s "$FIXTURES/0020-interior-blank.erg" "$FIXTURES/snapshot-0020"; then
    pass "migrate sweep: idempotent (second run no-op)"
else
    fail "migrate sweep: idempotent (second run no-op)"
fi
rm -f "$FIXTURES/snapshot-0020"

# --- Layout migration: tools/, FORMAT.md removed; archive/ → closed/; init refreshed ---
LDIR=$(mktemp -d)
trap 'rm -rf "$LDIR"' EXIT
mkdir -p "$LDIR/tickets/tools"
touch "$LDIR/tickets/FORMAT.md"
mkdir -p "$LDIR/archive"
# Place erg binary so cmdInit can find it
cp "$ERG" "$LDIR/tickets/erg"

$ERG migrate "$LDIR/tickets" >/dev/null 2>&1

if [ -d "$LDIR/tickets/tools" ]; then
    fail "layout migration: tickets/tools/ removed"
else
    pass "layout migration: tickets/tools/ removed"
fi
if [ -f "$LDIR/tickets/FORMAT.md" ]; then
    fail "layout migration: tickets/FORMAT.md removed"
else
    pass "layout migration: tickets/FORMAT.md removed"
fi
if [ -d "$LDIR/archive" ]; then
    fail "layout migration: archive/ gone after rename"
else
    pass "layout migration: archive/ gone after rename"
fi
if [ -d "$LDIR/closed" ]; then
    pass "layout migration: closed/ exists after rename"
else
    fail "layout migration: closed/ exists after rename"
fi

# --- Layout migration idempotency ---
if $ERG migrate "$LDIR/tickets" >/dev/null 2>&1; then
    pass "layout migration: idempotent (second run succeeds)"
else
    fail "layout migration: idempotent (second run must not error)"
fi

# --- Layout migration: self-copy binary when tickets/erg absent ---
BDIR=$(mktemp -d)
mkdir -p "$BDIR/tickets"
$ERG migrate "$BDIR/tickets" >/dev/null 2>&1
if [ -x "$BDIR/tickets/erg" ]; then
    pass "layout migration: self-copied tickets/erg when absent"
else
    fail "layout migration: self-copied tickets/erg when absent"
fi
mtime1=$(stat -c %Y "$BDIR/tickets/erg")
$ERG migrate "$BDIR/tickets" >/dev/null 2>&1
mtime2=$(stat -c %Y "$BDIR/tickets/erg")
if [ "$mtime1" = "$mtime2" ]; then
    pass "layout migration: self-copy idempotent (no overwrite)"
else
    fail "layout migration: self-copy idempotent (no overwrite)"
fi
rm -rf "$BDIR"

# --- Layout migration: merge archive/ into existing closed/ (no conflict) ---
MDIR2=$(mktemp -d)
mkdir -p "$MDIR2/tickets"
mkdir -p "$MDIR2/archive"
printf '%%erg 0.1\nTitle: Old\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n' > "$MDIR2/archive/0001-alpha.erg"
mkdir -p "$MDIR2/closed"
printf '%%erg 0.1\nTitle: Existing\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n' > "$MDIR2/closed/0002-beta.erg"
cp "$ERG" "$MDIR2/tickets/erg"
"$ERG" migrate "$MDIR2/tickets" >/dev/null 2>&1
if [ ! -d "$MDIR2/archive" ] && [ -f "$MDIR2/closed/0001-alpha.erg" ]; then
    pass "layout migration: merged archive/ into closed/ (no conflict)"
else
    fail "layout migration: merged archive/ into closed/ (no conflict)"
fi
rm -rf "$MDIR2"

# --- Layout migration: collision-abort when archive/ and closed/ share a filename ---
MDIR3=$(mktemp -d)
mkdir -p "$MDIR3/tickets"
mkdir -p "$MDIR3/archive"
echo "a" > "$MDIR3/archive/0001-conflict.erg"
mkdir -p "$MDIR3/closed"
echo "b" > "$MDIR3/closed/0001-conflict.erg"
cp "$ERG" "$MDIR3/tickets/erg"
EXIT_CODE=0
"$ERG" migrate "$MDIR3/tickets" >/dev/null 2>&1 || EXIT_CODE=$?
if [ "$EXIT_CODE" -ne 0 ] && [ -d "$MDIR3/archive" ] && [ -f "$MDIR3/archive/0001-conflict.erg" ]; then
    pass "layout migration: collision-abort exits non-zero and leaves archive/ untouched"
else
    fail "layout migration: collision-abort exits non-zero and leaves archive/ untouched"
fi
rm -rf "$MDIR3"

# --- Hook rewrite: legacy erg_bin path and validate→check ---
HDIR=$(mktemp -d)
mkdir -p "$HDIR/tickets" "$HDIR/.git/hooks"
cp "$ERG" "$HDIR/tickets/erg"
cat > "$HDIR/.git/hooks/pre-commit" <<'HOOK'
#!/bin/sh
erg_files=$(git diff --cached --name-only | grep '\.erg$' || true)
if [ -n "$erg_files" ]; then
    erg_bin="tickets/tools/go/erg"
    if [ -x "$erg_bin" ]; then
        if ! "$erg_bin" validate $erg_files; then exit 1; fi
        if ! "$erg_bin" validate tickets/; then exit 1; fi
    fi
fi
HOOK
chmod +x "$HDIR/.git/hooks/pre-commit"
"$ERG" migrate "$HDIR/tickets" >/dev/null 2>&1
if grep -q 'erg_bin="tickets/erg"' "$HDIR/.git/hooks/pre-commit"; then
    pass "hook rewrite: erg_bin path updated to tickets/erg"
else
    fail "hook rewrite: erg_bin path updated to tickets/erg"
fi
if grep -q '"$erg_bin" check tickets/' "$HDIR/.git/hooks/pre-commit"; then
    pass "hook rewrite: corpus validate → check on directory"
else
    fail "hook rewrite: corpus validate → check on directory"
fi
if grep -q 'tickets/tools/go/erg' "$HDIR/.git/hooks/pre-commit"; then
    fail "hook rewrite: legacy path fully removed"
else
    pass "hook rewrite: legacy path fully removed"
fi
if [ -x "$HDIR/.git/hooks/pre-commit" ]; then
    pass "hook rewrite: executable bit preserved"
else
    fail "hook rewrite: executable bit preserved"
fi
mtime1=$(stat -c %Y "$HDIR/.git/hooks/pre-commit")
"$ERG" migrate "$HDIR/tickets" >/dev/null 2>&1
mtime2=$(stat -c %Y "$HDIR/.git/hooks/pre-commit")
if [ "$mtime1" = "$mtime2" ]; then
    pass "hook rewrite: idempotent (second run does not touch the file)"
else
    fail "hook rewrite: idempotent (second run does not touch the file)"
fi
rm -rf "$HDIR"

# --- Hook rewrite: unmanaged hook (no legacy pattern) is left untouched ---
UHDIR=$(mktemp -d)
mkdir -p "$UHDIR/tickets" "$UHDIR/.git/hooks"
cp "$ERG" "$UHDIR/tickets/erg"
echo '#!/bin/sh' > "$UHDIR/.git/hooks/pre-commit"
echo 'echo "user hook"' >> "$UHDIR/.git/hooks/pre-commit"
chmod +x "$UHDIR/.git/hooks/pre-commit"
before=$(cat "$UHDIR/.git/hooks/pre-commit")
"$ERG" migrate "$UHDIR/tickets" >/dev/null 2>&1
after=$(cat "$UHDIR/.git/hooks/pre-commit")
if [ "$before" = "$after" ]; then
    pass "hook rewrite: unmanaged hook left untouched"
else
    fail "hook rewrite: unmanaged hook left untouched"
fi
rm -rf "$UHDIR"

# --- Hook rewrite: no hook file → no error ---
NHDIR=$(mktemp -d)
mkdir -p "$NHDIR/tickets" "$NHDIR/.git/hooks"
cp "$ERG" "$NHDIR/tickets/erg"
if "$ERG" migrate "$NHDIR/tickets" >/dev/null 2>&1; then
    pass "hook rewrite: missing hook is a silent no-op"
else
    fail "hook rewrite: missing hook is a silent no-op"
fi
if [ -e "$NHDIR/.git/hooks/pre-commit" ]; then
    fail "hook rewrite: missing hook stays missing"
else
    pass "hook rewrite: missing hook stays missing"
fi
rm -rf "$NHDIR"

echo "migrate: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
