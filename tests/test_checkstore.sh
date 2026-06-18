#!/bin/sh
# Integration tests for the local store-shape gate (ticket 0246).
#
# The gate is the Makefile `check-store` target: it runs `erg check` over the
# ticket store and exits non-zero on any corpus violation. PR #305 shipped a
# closed-but-unarchived ticket that `erg check` would have caught instantly,
# but nothing ran it locally before push -- it failed only in CI. These tests
# exercise the target (not just `erg check`) against planted violations via the
# STORE override, so a regression in the target wiring (wrong variable, missing
# build dependency, dropped from `make test`) is caught locally.
set -eu

ERG="${ERG_BIN:-build/erg}"
# The target must be exercised from the repo root so `make` resolves the
# Makefile, and the gate must use the binary under test. ERG_BIN is absolute in
# the Make-driven harness; fall back to an absolute build/erg otherwise.
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
case "$ERG" in
    /*) ERG_ABS="$ERG" ;;
    *)  ERG_ABS="$ROOT/$ERG" ;;
esac

FIXTURES=$(mktemp -d)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

cleanup() { rm -rf -- "$FIXTURES"; }
trap cleanup EXIT

echo "=== check-store (local store-shape gate) ==="

# gate runs `make check-store STORE=<dir>` from the repo root, reusing the
# already-built binary under test (ERG_BIN), and returns the target's exit code.
gate() {
    ( cd "$ROOT" && make --no-print-directory check-store \
        ERG_BIN="$ERG_ABS" STORE="$1" >/dev/null 2>&1 )
}

# --- Clean store passes (gate is not a false-positive machine) ---
mkdir -p "$FIXTURES/clean"
cat > "$FIXTURES/clean/0001-one.erg" <<'EOF'
%erg 0.1
Title: One
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
if gate "$FIXTURES/clean"; then
    pass "clean store passes the gate"
else
    fail "clean store passes the gate"
fi

# --- Closed-but-unarchived ticket fails (the exact PR #305 violation) ---
mkdir -p "$FIXTURES/unarchived"
cat > "$FIXTURES/unarchived/0001-done.erg" <<'EOF'
%erg 0.1
Title: Done but left in tickets/
Created: 2026-01-01
Closed: 2026-01-02
Author: a

--- log ---
--- body ---
EOF
if gate "$FIXTURES/unarchived"; then
    fail "closed-but-unarchived ticket fails the gate"
else
    pass "closed-but-unarchived ticket fails the gate"
fi

# --- Duplicate ID fails ---
mkdir -p "$FIXTURES/dup"
cat > "$FIXTURES/dup/0001-a.erg" <<'EOF'
%erg 0.1
Title: A
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
cat > "$FIXTURES/dup/0001-b.erg" <<'EOF'
%erg 0.1
Title: B
Created: 2026-01-01
Author: a

--- log ---
--- body ---
EOF
if gate "$FIXTURES/dup"; then
    fail "duplicate ID fails the gate"
else
    pass "duplicate ID fails the gate"
fi

echo "check-store: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
