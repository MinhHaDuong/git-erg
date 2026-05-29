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

# --- init unpacks exactly 4 files ---

OUT=$($ERG init "$REPO" 2>&1)

if [ -f "$REPO/tickets/AGENTS.md" ]; then
    pass "init creates AGENTS.md"
else
    fail "init creates AGENTS.md"
fi

if [ -f "$REPO/tickets/spec-erg-v1.md" ]; then
    pass "init creates spec-erg-v1.md"
else
    fail "init creates spec-erg-v1.md"
fi

if [ -f "$REPO/tickets/integration.md" ]; then
    pass "init creates integration.md"
else
    fail "init creates integration.md"
fi

if [ -f "$REPO/tickets/.ergrc" ]; then
    pass "init creates .ergrc"
else
    fail "init creates .ergrc"
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

if echo "$OUT2" | grep -q "0 created, 0 refreshed, 4 unchanged"; then
    pass "re-init is idempotent (4 unchanged)"
else
    fail "re-init is idempotent (expected '0 created, 0 refreshed, 4 unchanged', got: $OUT2)"
fi

# --- output mentions integration.md ---

if echo "$OUT" | grep -q "integration.md"; then
    pass "init output mentions integration.md"
else
    fail "init output mentions integration.md (got: $OUT)"
fi

# --- uninstall subcommand is removed ---

if $ERG uninstall "$REPO" >/dev/null 2>&1; then
    fail "uninstall subcommand should not exist"
else
    pass "uninstall subcommand removed"
fi

# --- unpacked AGENTS.md is clean UTF-8 (no U+FFFD) ---

if grep -aq "$(printf '\357\277\275')" "$REPO/tickets/AGENTS.md"; then
    fail "init-unpacked AGENTS.md contains U+FFFD replacement character"
else
    pass "init-unpacked AGENTS.md is clean UTF-8"
fi

echo "init: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
