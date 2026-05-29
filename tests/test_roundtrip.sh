#!/bin/sh
# Integration tests for: install/uninstall round-trip (ticket 0150)
set -eu

ERG="$(cd "$(dirname "${ERG_BIN:-build/erg}")" && pwd)/$(basename "${ERG_BIN:-build/erg}")"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== install/uninstall round-trip ==="

TDIR=$(mktemp -d)
trap 'rm -rf "$TDIR"' EXIT

# ── Cell 1: project-tree round-trip ──────────────────────────────
# init → real usage → manual uninstall → tree is byte-clean
# (only user-created ticket data remains)

REPO="$TDIR/repo"
git init -q "$REPO"
mkdir -p "$REPO/tickets"

# Snapshot before install (exclude .git internals)
(cd "$REPO" && find . -not -path './.git/*' -not -name '.git' | sort > "$TDIR/before.txt")

# Install: place binary + run erg init
cp "$ERG" "$REPO/tickets/erg"
chmod +x "$REPO/tickets/erg"
$ERG init "$REPO" >/dev/null 2>&1

# Install hook per integration.md step 1
mkdir -p "$REPO/.git/hooks"
cat > "$REPO/.git/hooks/pre-commit" << 'HOOK'
#!/bin/sh
if git diff --cached --name-only | grep -q '^tickets/erg$'; then
    branch=$(git branch --show-current)
    if [ "$branch" != "main" ]; then
        echo "pre-commit: do not commit tickets/erg in feature branches." >&2
        exit 1
    fi
fi
erg_files=$(git diff --cached --name-only | grep '\.erg$' || true)
if [ -n "$erg_files" ]; then
    erg_bin="tickets/erg"
    if [ -x "$erg_bin" ]; then
        $erg_bin validate $erg_files || { echo "ERROR: validation failed" >&2; exit 1; }
        $erg_bin check tickets/ || { echo "ERROR: check failed" >&2; exit 1; }
    fi
fi
HOOK
chmod +x "$REPO/.git/hooks/pre-commit"

# Verify all install artifacts exist
for f in tickets/.ergrc tickets/AGENTS.md tickets/spec-erg-v1.md tickets/integration.md tickets/erg; do
    if [ -f "$REPO/$f" ]; then
        pass "install creates $f"
    else
        fail "install creates $f"
    fi
done

if [ -f "$REPO/.git/hooks/pre-commit" ]; then
    pass "install creates pre-commit hook"
else
    fail "install creates pre-commit hook"
fi

# ── Real usage in between ────────────────────────────────────────
# Exercise the tool so the store is non-trivial before removal.

(cd "$REPO" && $ERG new "Roundtrip test ticket" tickets/) >/dev/null 2>&1
(cd "$REPO" && $ERG list tickets/) >/dev/null 2>&1
(cd "$REPO" && $ERG log 0001 "testing round-trip" tickets/) >/dev/null 2>&1
(cd "$REPO" && $ERG close 0001 "done" tickets/) >/dev/null 2>&1
(cd "$REPO" && $ERG archive tickets/) >/dev/null 2>&1
(cd "$REPO" && $ERG check tickets/) >/dev/null 2>&1

if [ -f "$REPO/tickets/closed/0001-roundtrip-test-ticket.erg" ]; then
    pass "usage: ticket created, closed, archived"
else
    fail "usage: ticket created, closed, archived"
fi

# ── Manual uninstall ─────────────────────────────────────────────
# Follow the documented steps from integration.md exactly.

rm -f "$REPO/tickets/.ergrc" \
      "$REPO/tickets/AGENTS.md" \
      "$REPO/tickets/spec-erg-v1.md" \
      "$REPO/tickets/integration.md" \
      "$REPO/tickets/erg"
rm -f "$REPO/.git/hooks/pre-commit"

# ── Verify: tree is clean except for user data ───────────────────

(cd "$REPO" && find . -not -path './.git/*' -not -name '.git' | sort > "$TDIR/after.txt")

# The only difference should be user-created ticket data
DIFF=$(diff "$TDIR/before.txt" "$TDIR/after.txt" || true)

# after.txt should have MORE entries (the user's archived ticket + dirs)
ADDED=$(echo "$DIFF" | grep '^>' | sed 's/^> //' || true)
REMOVED=$(echo "$DIFF" | grep '^<' || true)

if [ -z "$REMOVED" ]; then
    pass "uninstall leaves no orphan files"
else
    fail "uninstall leaves orphan files: $REMOVED"
fi

# The only additions should be under tickets/ (user data)
BAD_ADDITIONS=""
for line in $ADDED; do
    case "$line" in
        ./tickets|./tickets/closed|./tickets/closed/*) ;;
        *) BAD_ADDITIONS="$BAD_ADDITIONS $line" ;;
    esac
done

if [ -z "$BAD_ADDITIONS" ]; then
    pass "only user ticket data remains after uninstall"
else
    fail "unexpected files after uninstall:$BAD_ADDITIONS"
fi

# User data is explicitly preserved
if [ -f "$REPO/tickets/closed/0001-roundtrip-test-ticket.erg" ]; then
    pass "user tickets preserved after uninstall"
else
    fail "user tickets preserved after uninstall"
fi

# ── Cell 2: ~/.local/bin round-trip ──────────────────────────────
# install-erg-binary → rm → no orphans under fake $HOME

FAKE_HOME="$TDIR/fakehome"
mkdir -p "$FAKE_HOME"

# Run the real Makefile target with a fake HOME so it installs there.
# BOOTSTRAP_BIN points to the build output (the binary under test).
PROJ_ROOT="$(cd "$(dirname "$ERG")/.." && pwd)"
HOME="$FAKE_HOME" make -C "$PROJ_ROOT" install-erg-binary \
    BOOTSTRAP_BIN="$ERG" >/dev/null 2>&1

if [ -f "$FAKE_HOME/.local/bin/erg" ]; then
    pass "install-erg-binary places binary in ~/.local/bin"
else
    fail "install-erg-binary places binary in ~/.local/bin"
fi

rm -f "$FAKE_HOME/.local/bin/erg"

ORPHANS=$(find "$FAKE_HOME" -type f 2>/dev/null || true)
if [ -z "$ORPHANS" ]; then
    pass "no orphan files under \$HOME after uninstall"
else
    fail "orphan files under \$HOME: $ORPHANS"
fi

# ── Summary ──────────────────────────────────────────────────────

echo "roundtrip: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
