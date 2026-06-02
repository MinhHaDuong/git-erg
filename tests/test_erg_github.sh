#!/bin/sh
# Integration tests for: tickets/erg-github (forge layer, ticket 0209)
set -eu

ERG="${ERG_BIN:-build/erg}"
ERG_ABS=$(CDPATH= cd "$(dirname "$ERG")" && pwd)/$(basename "$ERG")
# erg-github lives in the repo's tickets/ (committed helper). Resolve from CWD.
SCRIPT=$(CDPATH= cd "$(dirname "$0")/.." && pwd)/tickets/erg-github
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg-github ==="

TDIR=$(mktemp -d)
trap 'rm -rf "$TDIR"' EXIT

# --- executable bit set (git records 100755 for an executable committed file) ---
if [ -x "$SCRIPT" ]; then
    pass "erg-github is executable"
else
    fail "erg-github is not executable"
fi
# If it is already tracked, also assert the recorded git mode is 100755.
gmode=$(cd "$(dirname "$SCRIPT")/.." && git ls-files -s tickets/erg-github 2>/dev/null | awk '{print $1}')
if [ -z "$gmode" ] || [ "$gmode" = "100755" ]; then
    pass "erg-github tracked mode is 100755 (or not yet tracked)"
else
    fail "erg-github tracked mode is '$gmode' (expected 100755)"
fi

# --- pure ASCII (it ships and runs on every clone) ---
if LC_ALL=C grep -nq '[^[:print:][:space:]]' "$SCRIPT"; then
    fail "erg-github contains non-ASCII bytes"
else
    pass "erg-github is pure ASCII"
fi

# --- POSIX sh portability ratchet (no bashisms) ---
# NOTE '\[\[[^:]' matches bash's [[ test but NOT the POSIX class [[:space:]].
bashism=""
grep -nE '\[\[[^:]|^[[:space:]]*local |declare -a|pipefail|\$'"'" "$SCRIPT" >/dev/null 2>&1 && bashism="yes"
if [ -z "$bashism" ]; then
    pass "erg-github has no obvious bashisms"
else
    fail "erg-github contains a bashism ([[ / local / declare -a / pipefail / \$'...')"
fi
# shebang is /bin/sh
if head -1 "$SCRIPT" | grep -q '^#!/bin/sh'; then
    pass "erg-github shebang is /bin/sh"
else
    fail "erg-github shebang is not /bin/sh"
fi

# --- set up a fake repo with a fake gh ---
REPO="$TDIR/repo"
mkdir -p "$REPO/tickets" "$REPO/fakebin"
cp "$ERG_ABS" "$REPO/tickets/erg"
cp "$SCRIPT" "$REPO/tickets/erg-github"
chmod +x "$REPO/tickets/erg-github"
(cd "$REPO" && git init -q -b main)

# fake gh: prints $GH_FAKE_BODY for `gh pr view ... --json body --jq .body`
cat > "$REPO/fakebin/gh" <<'GH'
#!/bin/sh
printf '%s' "${GH_FAKE_BODY:-}"
GH
chmod +x "$REPO/fakebin/gh"

mk_ticket() { # id closed?
    f="$REPO/tickets/$1-x.erg"
    {
        echo "%erg 0.1"
        echo "Title: X"
        echo "Created: 2026-06-02"
        echo "Author: t"
        [ "${2:-}" = "closed" ] && echo "Closed: 2026-06-02"
        echo ""
        echo "--- log ---"
        echo "--- body ---"
    } > "$f"
}

run_verify() { # PATH-extra GH_BODY ci? prnum
    ( cd "$REPO" && PATH="$REPO/fakebin:$PATH" GH_FAKE_BODY="$2" GITHUB_ACTIONS="$3" sh tickets/erg-github verify "$4" 2>&1 )
}

# --- verify: closed ticket -> PASS (exit 0) ---
mk_ticket 0042 closed
out=$(run_verify x "**Ticket:** tickets/0042-x.erg" "" 7) && rc=0 || rc=$?
if [ "$rc" -eq 0 ] && echo "$out" | grep -q "PASS"; then
    pass "verify: closed ticket passes"
else
    fail "verify: closed ticket should pass (rc=$rc, out: $out)"
fi

# --- verify: open ticket -> FAIL (exit 1) ---
mk_ticket 0042 open
out=$(run_verify x "**Ticket:** tickets/0042-x.erg" "" 7) && rc=0 || rc=$?
if [ "$rc" -eq 1 ] && echo "$out" | grep -qi "please close ticket 0042"; then
    pass "verify: open ticket fails with actionable message"
else
    fail "verify: open ticket should fail (rc=$rc, out: $out)"
fi

# --- verify: no ticket referenced -> escape hatch PASS, loud ---
out=$(run_verify x "A normal PR body with no ticket line." "" 7) && rc=0 || rc=$?
if [ "$rc" -eq 0 ] && echo "$out" | grep -qi "escape hatch"; then
    pass "verify: no-ref escape hatch passes loudly"
else
    fail "verify: no-ref should pass with escape-hatch notice (rc=$rc, out: $out)"
fi

# --- verify: multiple ticket refs -> FAIL (one PR closes one ticket) ---
mk_ticket 0042 closed
mk_ticket 0043 closed
multi="**Ticket:** tickets/0042-x.erg
**Ticket:** tickets/0043-x.erg"
out=$(run_verify x "$multi" "" 7) && rc=0 || rc=$?
if [ "$rc" -eq 1 ] && echo "$out" | grep -qi "one PR closes one ticket"; then
    pass "verify: multiple ticket refs rejected"
else
    fail "verify: multiple refs should fail (rc=$rc, out: $out)"
fi

# A fake gh that FAILS (simulates unauthenticated / unreachable forge): the
# same could-not-verify entry point as a truly absent gh. Tools stay on PATH.
cat > "$REPO/fakebin/gh-fail" <<'GH'
#!/bin/sh
exit 1
GH
chmod +x "$REPO/fakebin/gh-fail"
run_verify_failgh() { # ci?
    ( cd "$REPO" && cp fakebin/gh-fail fakebin/gh && PATH="$REPO/fakebin:$PATH" GITHUB_ACTIONS="$1" sh tickets/erg-github verify 7 2>&1 )
}

# --- verify: could-not-verify + local -> lenient exit 0 ---
out=$(run_verify_failgh "") && rc=0 || rc=$?
if [ "$rc" -eq 0 ] && echo "$out" | grep -qi "skipping"; then
    pass "verify: could-not-verify locally is non-blocking (exit 0)"
else
    fail "verify: local could-not-verify should exit 0 (rc=$rc, out: $out)"
fi

# --- verify: could-not-verify + CI -> fail closed exit 1 ---
out=$(run_verify_failgh "true") && rc=0 || rc=$?
if [ "$rc" -eq 1 ] && echo "$out" | grep -qi "failing closed"; then
    pass "verify: could-not-verify in CI fails closed (exit 1)"
else
    fail "verify: CI could-not-verify should fail closed (rc=$rc, out: $out)"
fi

# --- install: writes a pinned workflow YAML ---
( cd "$REPO" && sh tickets/erg-github install >/dev/null 2>&1 )
Y="$REPO/.github/workflows/erg-verify.yml"
if [ -f "$Y" ]; then
    pass "install: writes erg-verify.yml"
else
    fail "install: did not write erg-verify.yml"
fi
ok=1
grep -q "pull_request:" "$Y" || ok=0
grep -q "paths:" "$Y" && ok=0   # a required check must run on every PR
grep -q "pull-requests: read" "$Y" || ok=0
grep -q "contents: read" "$Y" || ok=0
grep -q "head.sha" "$Y" || ok=0
grep -q "GH_TOKEN" "$Y" || ok=0
if [ "$ok" -eq 1 ]; then
    pass "install: YAML is pinned (pull_request, no paths, perms, head.sha, GH_TOKEN)"
else
    fail "install: YAML missing a pinned property"
fi

# --- install: idempotent (does not clobber an existing workflow) ---
printf 'custom\n' > "$Y"
( cd "$REPO" && sh tickets/erg-github install >/dev/null 2>&1 )
if grep -q "custom" "$Y"; then
    pass "install: leaves an existing workflow untouched"
else
    fail "install: clobbered an existing workflow"
fi

echo ""
echo "erg-github: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
