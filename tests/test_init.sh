#!/bin/sh
# Integration tests for: erg init, erg uninstall
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg init/uninstall ==="

TDIR=$(mktemp -d)
trap 'rm -rf "$TDIR"' EXIT

REPO="$TDIR/repo"
mkdir -p "$REPO/.git/hooks"
cat > "$REPO/.git/hooks/pre-commit" <<'EOFHOOK'
#!/bin/sh
echo "custom pre-commit"
EOFHOOK
chmod +x "$REPO/.git/hooks/pre-commit"

OUT=$($ERG init "$REPO" 2>&1)

# Managed files materialized
if [ -f "$REPO/tickets/README.md" ] && [ -f "$REPO/tickets/spec-erg-v1.md" ] && [ -f "$REPO/tickets/integration/manifest.json" ]; then
    pass "init materializes managed files"
else
    fail "init materializes managed files"
fi

# Fragment appends
if grep -q '^git-erg local tickets: see tickets/README.md$' "$REPO/AGENTS.md"; then
    pass "init appends AGENTS pointer"
else
    fail "init appends AGENTS pointer"
fi

if grep -q '^tickets/erg$' "$REPO/.gitignore"; then
    pass "init appends .gitignore entry"
else
    fail "init appends .gitignore entry"
fi

if grep -q '^# --- git-erg: begin managed block ---$' "$REPO/.git/hooks/pre-commit" && grep -q '^# --- git-erg: end managed block ---$' "$REPO/.git/hooks/pre-commit"; then
    pass "init appends managed pre-commit block"
else
    fail "init appends managed pre-commit block"
fi

if [ -f "$REPO/tickets/.erg-bootstrap-manifest.json" ]; then
    pass "init writes manifest"
else
    fail "init writes manifest"
fi

# Re-init is idempotent (no duplicate fragments)
$ERG init "$REPO" >/dev/null

if [ "$(grep -c '^git-erg local tickets: see tickets/README.md$' "$REPO/AGENTS.md")" -eq 1 ]; then
    pass "re-init does not duplicate AGENTS pointer"
else
    fail "re-init does not duplicate AGENTS pointer"
fi

if [ "$(grep -c '^tickets/erg$' "$REPO/.gitignore")" -eq 1 ]; then
    pass "re-init does not duplicate .gitignore entry"
else
    fail "re-init does not duplicate .gitignore entry"
fi

if [ "$(grep -c '^# --- git-erg: begin managed block ---$' "$REPO/.git/hooks/pre-commit")" -eq 1 ]; then
    pass "re-init does not duplicate pre-commit block"
else
    fail "re-init does not duplicate pre-commit block"
fi

# Preserve user ticket on uninstall
cat > "$REPO/tickets/0999-user-ticket.erg" <<'EOFTICKET'
%erg v1
Title: User ticket
Created: 2026-05-04
Author: user

--- log ---
2026-05-04T12:00Z user created

--- body ---
User-managed ticket data.
EOFTICKET

UOUT=$($ERG uninstall "$REPO" 2>&1)

if [ -f "$REPO/tickets/0999-user-ticket.erg" ]; then
    pass "uninstall preserves user ticket files"
else
    fail "uninstall preserves user ticket files"
fi

if [ ! -f "$REPO/tickets/spec-erg-v1.md" ] && [ ! -f "$REPO/tickets/integration/manifest.json" ]; then
    pass "uninstall removes managed support files"
else
    fail "uninstall removes managed support files"
fi

if [ ! -f "$REPO/tickets/.erg-bootstrap-manifest.json" ]; then
    pass "uninstall removes manifest"
else
    fail "uninstall removes manifest"
fi

if ! grep -q '^git-erg local tickets: see tickets/README.md$' "$REPO/AGENTS.md"; then
    pass "uninstall removes AGENTS pointer"
else
    fail "uninstall removes AGENTS pointer"
fi

if ! grep -q '^tickets/erg$' "$REPO/.gitignore"; then
    pass "uninstall removes .gitignore entry"
else
    fail "uninstall removes .gitignore entry"
fi

if ! grep -q '^# --- git-erg: begin managed block ---$' "$REPO/.git/hooks/pre-commit" && grep -q 'custom pre-commit' "$REPO/.git/hooks/pre-commit"; then
    pass "uninstall removes managed hook block and keeps existing hook content"
else
    fail "uninstall removes managed hook block and keeps existing hook content"
fi

if echo "$OUT" | grep -q '^Plan:' && echo "$UOUT" | grep -q '^Plan:'; then
    pass "init and uninstall print plan"
else
    fail "init and uninstall print plan"
fi

# Output uses yes/no not true/false
if echo "$OUT" | grep -qE "true|false"; then
    fail "init output must not contain true/false"
else
    pass "init output: no true/false booleans"
fi
if echo "$OUT" | grep -qF "manifest written:"; then
    pass "init output: 'manifest written:'"
else
    fail "init output: 'manifest written:' (got: $OUT)"
fi
if echo "$OUT" | grep -qF "AGENTS.md entry"; then
    pass "init output: 'AGENTS.md entry'"
else
    fail "init output: 'AGENTS.md entry' (got: $OUT)"
fi
if echo "$OUT" | grep -qF "AGENTS.md pointer"; then
    fail "init output: must not say 'AGENTS.md pointer'"
else
    pass "init output: no 'AGENTS.md pointer' jargon"
fi

echo "init/uninstall: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
