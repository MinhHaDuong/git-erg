#!/bin/sh
# Integration tests for: erg init
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg init ==="

TDIR=$(mktemp -d)
trap 'rm -rf "$TDIR"' EXIT

REPO="$TDIR/repo"
mkdir -p "$REPO/tickets"

# --- init without binary exits 1 ---

if OUT=$($ERG init "$REPO" 2>&1); then
    fail "init without binary should exit 1"
else
    if echo "$OUT" | grep -q "binary not found"; then
        pass "init without binary: exits 1 with 'binary not found'"
    else
        fail "init without binary: expected 'binary not found' in output (got: $OUT)"
    fi
fi

# --- place a fake binary ---

touch "$REPO/tickets/erg"

# --- init unpacks exactly 2 files ---

OUT=$($ERG init "$REPO" 2>&1)

if [ -f "$REPO/tickets/AGENTS.md" ]; then
    pass "init creates AGENTS.md"
else
    fail "init creates AGENTS.md"
fi

if [ -f "$REPO/tickets/.ergrc" ]; then
    pass "init creates .ergrc"
else
    fail "init creates .ergrc"
fi

if [ -f "$REPO/tickets/spec-erg-v1.md" ]; then
    fail "init must not deposit spec-erg-v1.md (now: erg spec)"
else
    pass "init does not deposit spec-erg-v1.md"
fi

if [ -f "$REPO/tickets/integration.md" ]; then
    fail "init must not deposit integration.md (now: erg integration)"
else
    pass "init does not deposit integration.md"
fi

# --- no integration/ directory created ---

if [ -d "$REPO/tickets/integration" ]; then
    fail "init must not create tickets/integration/ directory"
else
    pass "init does not create tickets/integration/ directory"
fi

# --- no manifest, no AGENTS.md, no .gitignore, no hook ---

if [ -f "$REPO/tickets/.erg-bootstrap-manifest.json" ]; then
    fail "init must not write manifest"
else
    pass "init does not write manifest"
fi

if [ -f "$REPO/AGENTS.md" ]; then
    fail "init must not touch AGENTS.md"
else
    pass "init does not touch AGENTS.md"
fi

if [ -f "$REPO/.gitignore" ]; then
    fail "init must not touch .gitignore"
else
    pass "init does not touch .gitignore"
fi

# --- re-init is idempotent ---

OUT2=$($ERG init "$REPO" 2>&1)

if echo "$OUT2" | grep -q "0 created, 0 refreshed, 0 skipped (local edits), 2 unchanged"; then
    pass "re-init is idempotent (2 unchanged)"
else
    fail "re-init is idempotent (expected '0 created, 0 refreshed, 0 skipped (local edits), 2 unchanged', got: $OUT2)"
fi

# --- re-init refuses to overwrite user-edited files ---
printf "# user edit\n" >> "$REPO/tickets/.ergrc"
OUT3=$($ERG init "$REPO" 2>&1) && RC3=0 || RC3=$?
if [ "$RC3" -ne 0 ]; then
    pass "re-init with local edits: exits non-zero"
else
    fail "re-init with local edits: exits non-zero (rc=$RC3)"
fi
if [ "$RC3" -eq 2 ]; then
    pass "re-init with local edits: exit code is 2 (skipped, not hard error)"
else
    fail "re-init with local edits: expected exit 2, got $RC3"
fi
if grep -q "# user edit" "$REPO/tickets/.ergrc"; then
    pass "re-init with local edits: modified file preserved"
else
    fail "re-init with local edits: modified file was overwritten"
fi
if echo "$OUT3" | grep -q "local edits"; then
    pass "re-init with local edits: mentions 'local edits' in output"
else
    fail "re-init with local edits: expected 'local edits' in output (got: $OUT3)"
fi

# --- output mentions erg install ---

if echo "$OUT" | grep -q "erg install"; then
    pass "init output mentions erg install"
else
    fail "init output mentions erg install (got: $OUT)"
fi

# --- uninstall subcommand is removed ---

if $ERG uninstall "$REPO" >/dev/null 2>&1; then
    fail "uninstall subcommand should not exist"
else
    pass "uninstall subcommand removed"
fi

# --- unpacked AGENTS.md is pure ASCII (no U+FFFD, no stray Unicode) ---
# The original bug (0160) was a U+FFFD replacement character introduced by a
# Unicode round-trip. Asserting pure ASCII is strictly stronger than checking
# for U+FFFD alone and forecloses the whole corruption class.

if LC_ALL=C grep -nq '[^[:print:][:space:]]' "$REPO/tickets/AGENTS.md"; then
    fail "init-unpacked AGENTS.md contains non-ASCII or non-printable bytes"
else
    pass "init-unpacked AGENTS.md is pure ASCII"
fi

# unknown flag rejection (ticket 0178)
    out=$($ERG init --bogus 2>&1) && rc=0 || rc=$?
    if [ "$rc" -ne 0 ] && echo "$out" | grep -q "unknown flag"; then
        pass "unknown flag rejected with usage message"
    else
        fail "unknown flag not rejected (rc=$rc, got: $out)"
    fi

# --- orphan cleanup: matching files are removed ---

ORPHAN="$TDIR/orphan"
mkdir -p "$ORPHAN/tickets"
touch "$ORPHAN/tickets/erg"
# Pre-place spec-erg-v1.md with the exact embedded content
$ERG spec > "$ORPHAN/tickets/spec-erg-v1.md" 2>/dev/null
$ERG integration > "$ORPHAN/tickets/integration.md" 2>/dev/null
OUT_ORPHAN=$($ERG init "$ORPHAN" 2>&1)
if [ ! -f "$ORPHAN/tickets/spec-erg-v1.md" ]; then
    pass "orphan cleanup: matching spec-erg-v1.md removed"
else
    fail "orphan cleanup: matching spec-erg-v1.md still present"
fi
if [ ! -f "$ORPHAN/tickets/integration.md" ]; then
    pass "orphan cleanup: matching integration.md removed"
else
    fail "orphan cleanup: matching integration.md still present"
fi
if echo "$OUT_ORPHAN" | grep -q "removed orphaned asset"; then
    pass "orphan cleanup: logged removal message"
else
    fail "orphan cleanup: no removal message (got: $OUT_ORPHAN)"
fi

# --- orphan cleanup: divergent files are preserved ---

DIVERGE="$TDIR/diverge"
mkdir -p "$DIVERGE/tickets"
touch "$DIVERGE/tickets/erg"
printf "# my custom spec\n" > "$DIVERGE/tickets/spec-erg-v1.md"
printf "# my custom integration\n" > "$DIVERGE/tickets/integration.md"
$ERG init "$DIVERGE" > /dev/null 2>&1
if [ -f "$DIVERGE/tickets/spec-erg-v1.md" ]; then
    pass "orphan cleanup: divergent spec-erg-v1.md preserved"
else
    fail "orphan cleanup: divergent spec-erg-v1.md was removed"
fi
if [ -f "$DIVERGE/tickets/integration.md" ]; then
    pass "orphan cleanup: divergent integration.md preserved"
else
    fail "orphan cleanup: divergent integration.md was removed"
fi

# --- dry-run: no side effects (ticket 0207) ---

DRY="$TDIR/dry"
mkdir -p "$DRY/tickets"
touch "$DRY/tickets/erg"
# Pre-place a matching orphan that a real init would remove.
$ERG spec > "$DRY/tickets/spec-erg-v1.md" 2>/dev/null
OUT_DRY=$($ERG init -n "$DRY" 2>&1) && RC_DRY=0 || RC_DRY=$?
if [ "$RC_DRY" -eq 0 ]; then
    pass "dry-run: exits 0 on a clean preview"
else
    fail "dry-run: expected exit 0, got $RC_DRY"
fi
if [ ! -f "$DRY/tickets/.ergrc" ] && [ ! -f "$DRY/tickets/AGENTS.md" ]; then
    pass "dry-run: did not create any asset"
else
    fail "dry-run: created assets (.ergrc or AGENTS.md present)"
fi
if [ -f "$DRY/tickets/spec-erg-v1.md" ]; then
    pass "dry-run: did not remove the matching orphan"
else
    fail "dry-run: removed the orphan (should only preview)"
fi
if echo "$OUT_DRY" | grep -q "dry-run"; then
    pass "dry-run: output labels itself as dry-run"
else
    fail "dry-run: output missing 'dry-run' label (got: $OUT_DRY)"
fi
# --dry-run long form is also accepted
$ERG init --dry-run "$DRY" >/dev/null 2>&1 && pass "long --dry-run accepted" || fail "long --dry-run rejected"

# --- dry-run reports exit 2 when it would skip a local edit ---

DRY2="$TDIR/dry2"
mkdir -p "$DRY2/tickets"
touch "$DRY2/tickets/erg"
$ERG init "$DRY2" >/dev/null 2>&1
printf "# edit\n" >> "$DRY2/tickets/.ergrc"
$ERG init -n "$DRY2" >/dev/null 2>&1 && RC_DRY2=0 || RC_DRY2=$?
if [ "$RC_DRY2" -eq 2 ]; then
    pass "dry-run: exit 2 when a local edit would be skipped"
else
    fail "dry-run: expected exit 2 for would-skip, got $RC_DRY2"
fi
# The local edit must still be present (dry-run never writes).
if grep -q "# edit" "$DRY2/tickets/.ergrc"; then
    pass "dry-run: local edit untouched"
else
    fail "dry-run: local edit was modified"
fi

# --- --force overwrites divergent files (exit 0) ---

FORCE="$TDIR/force"
mkdir -p "$FORCE/tickets"
touch "$FORCE/tickets/erg"
$ERG init "$FORCE" >/dev/null 2>&1
printf "# user edit to clobber\n" >> "$FORCE/tickets/.ergrc"
$ERG init --force "$FORCE" >/dev/null 2>&1 && RC_FORCE=0 || RC_FORCE=$?
if [ "$RC_FORCE" -eq 0 ]; then
    pass "--force: exits 0"
else
    fail "--force: expected exit 0, got $RC_FORCE"
fi
if grep -q "# user edit to clobber" "$FORCE/tickets/.ergrc"; then
    fail "--force: local edit survived (should be overwritten)"
else
    pass "--force: local edit overwritten"
fi

# --- chained read-only check surfaces a corpus warning, init still exits 0 ---

CHAIN="$TDIR/chain"
mkdir -p "$CHAIN/tickets"
touch "$CHAIN/tickets/erg"
# A closed ticket placed outside closed/ triggers folderClosure WARN.
cat > "$CHAIN/tickets/9001-chain.erg" <<'ERGEOF'
%erg 0.1
Title: Chain test closed ticket
Created: 2026-06-02
Author: claude
Closed: 2026-06-02

--- log ---
2026-06-02T00:00Z claude created

--- body ---
Closed ticket outside closed/ to trigger a folderClosure warning.
ERGEOF
OUT_CHAIN=$($ERG init "$CHAIN" 2>&1) && RC_CHAIN=0 || RC_CHAIN=$?
if echo "$OUT_CHAIN" | grep -q "closed ticket not in closed/ directory"; then
    pass "chaining: init surfaces the corpus warning"
else
    fail "chaining: warning not surfaced (got: $OUT_CHAIN)"
fi
if [ "$RC_CHAIN" -eq 0 ]; then
    pass "chaining: init exit code reflects init (0), not the warning"
else
    fail "chaining: init exit should be 0 despite warning, got $RC_CHAIN"
fi


echo "init: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
