#!/bin/sh
# Integration tests for: erg version, erg update
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0; FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg update/version ==="

# Test: erg version exits 0 and prints structured output with hash and arch
VER=$("$ERG" version)
if echo "$VER" | grep -qE '^[[:space:]]+hash:[[:space:]]+[0-9a-f]{12}$' && echo "$VER" | grep -q 'arch:'; then
    pass "version prints structured info"
else
    fail "version output: $VER"
fi

# Test: erg update with bad URL exits 0
if ERG_UPDATE_URL=http://127.0.0.1:1 "$ERG" update 2>/dev/null; then
    pass "update offline exits 0"
else
    fail "update offline should exit 0"
fi

# Test: erg update with local server serving same binary → already up to date
PORT=$(python3 -c "import socket; s=socket.socket(); s.bind(('',0)); print(s.getsockname()[1]); s.close()")
SRV_DIR=$(mktemp -d)
TICKET_DIR=
WORKSPACE=
cp "$ERG" "$SRV_DIR/erg"
( cd "$SRV_DIR" && exec python3 -m http.server $PORT >/dev/null 2>&1 ) &
SERVER_PID=$!
cleanup() {
    kill $SERVER_PID 2>/dev/null || true
    rm -rf "$SRV_DIR"
    [ -n "$TICKET_DIR" ] && rm -rf "$TICKET_DIR"
    [ -n "$WORKSPACE" ] && rm -rf "$WORKSPACE"
}
trap cleanup EXIT

# Wait for server to be ready
i=0; until curl -s http://127.0.0.1:$PORT/ >/dev/null 2>&1 || [ $i -ge 20 ]; do sleep 0.1; i=$((i+1)); done

OUT=$(ERG_UPDATE_URL=http://127.0.0.1:$PORT/erg "$ERG" update 2>&1)
if echo "$OUT" | grep -q "already up to date"; then
    pass "update same binary is no-op"
else
    fail "update same binary: $OUT"
fi

# Test: update with Status: tickets → emits hint, does NOT rewrite files
TICKET_DIR=$(mktemp -d)
cat > "$TICKET_DIR/0001-legacy.erg" <<'ERGEOF'
%erg v1
Title: Legacy ticket
Created: 2026-01-01
Author: a
Status: open

--- log ---
2026-01-01T10:00Z a created

--- body ---
ERGEOF
BEFORE=$(cat "$TICKET_DIR/0001-legacy.erg")
# Force a real update path (different hash) so migrate hint logic runs.
cp "$ERG" "$SRV_DIR/erg-new"
printf '\n' >> "$SRV_DIR/erg-new"
OUT=$(ERG_UPDATE_URL=http://127.0.0.1:$PORT/erg-new ERG_TICKET_DIR="$TICKET_DIR" "$ERG" update 2>&1 || true)
AFTER=$(cat "$TICKET_DIR/0001-legacy.erg")
if [ "$BEFORE" = "$AFTER" ]; then
    pass "update does not rewrite ticket files"
else
    fail "update rewrote ticket files"
fi
if echo "$OUT" | grep -q "erg migrate"; then
    pass "update emits migrate hint"
else
    fail "update missing migrate hint"
fi
rm -rf "$TICKET_DIR"

# Test: erg update with 404 response exits 0 and stderr contains "server returned"
OUT=$(ERG_UPDATE_URL=http://127.0.0.1:$PORT/no-such-file "$ERG" update 2>&1 || true)
if echo "$OUT" | grep -q "server returned"; then
    pass "update 404 exits 0 and emits 'server returned'"
else
    fail "update 404: $OUT"
fi

# Test: erg update with 200 empty body exits 0 and stderr contains "empty body"
printf '' > "$SRV_DIR/erg-empty"
OUT=$(ERG_UPDATE_URL=http://127.0.0.1:$PORT/erg-empty "$ERG" update 2>&1 || true)
if echo "$OUT" | grep -q "empty body"; then
    pass "update empty-body 200 exits 0 and emits 'empty body'"
else
    fail "update empty-body 200: $OUT"
fi

# Test: update does NOT rewrite managed assets (binary-only contract)
WORKSPACE=$(mktemp -d)
ERG_ABS=$(readlink -f "$ERG")
mkdir -p "$WORKSPACE/tickets"
echo "stale content" > "$WORKSPACE/tickets/AGENTS.md"
cp "$ERG_ABS" "$SRV_DIR/erg-new2"
printf '\x00' >> "$SRV_DIR/erg-new2"
OUT=$(cd "$WORKSPACE" && ERG_UPDATE_URL=http://127.0.0.1:$PORT/erg-new2 "$ERG_ABS" update 2>&1 || true)
STALE_CONTENT=$(cat "$WORKSPACE/tickets/AGENTS.md")
if [ "$STALE_CONTENT" = "stale content" ]; then
    pass "update does not rewrite managed assets"
else
    fail "update rewrote managed asset (AGENTS.md content changed)"
fi
rm -rf "$WORKSPACE"

# --- vcsRevision-based outdated detection tests ---

# Test: erg version output includes revision: line when vcsRevision is embedded.
VER2=$("$ERG" version 2>&1)
if echo "$VER2" | grep -qE '^\s+revision:'; then
    pass "version: revision: line present in output"
else
    fail "version: revision: line missing from output: $VER2"
fi

# Test: a binary claiming the same vcsRevision is NOT marked [outdated].
# We create a shell stub that prints erg version output with the same revision
# as the running binary, but a different hash. Place it in a temp PATH dir so
# erg discovers it, then assert no [outdated] label appears.
SELF_REVISION=$(echo "$VER2" | grep -E '^\s+revision:' | sed 's/.*revision:[[:space:]]*//')
VERSION_TMPDIR=$(mktemp -d)
STUB="$VERSION_TMPDIR/erg"
cat > "$STUB" <<STUBEOF
#!/bin/sh
if [ "\$1" = "version" ]; then
    echo "erg version"
    echo "  path:    $VERSION_TMPDIR/erg"
    echo "  hash:    aabbccddeeff"
    echo "  built:   2020-01-01T00:00:00Z"
    echo "  revision: $SELF_REVISION"
    echo "  arch:    linux/amd64"
fi
STUBEOF
chmod +x "$STUB"

OUT=$(PATH="$VERSION_TMPDIR:$PATH" "$ERG" version 2>&1)
if echo "$OUT" | grep -F "$VERSION_TMPDIR/erg" | grep -q "\[outdated"; then
    fail "version: same-revision stub incorrectly marked [outdated]: $OUT"
else
    pass "version: same-revision binary not marked [outdated]"
fi
rm -rf "$VERSION_TMPDIR"

# Test: a binary with a different (older) vcsRevision IS marked [outdated].
VERSION_TMPDIR2=$(mktemp -d)
STUB2="$VERSION_TMPDIR2/erg"
cat > "$STUB2" <<STUBEOF2
#!/bin/sh
if [ "\$1" = "version" ]; then
    echo "erg version"
    echo "  path:    $VERSION_TMPDIR2/erg"
    echo "  hash:    deadbeefcafe"
    echo "  built:   2020-01-01T00:00:00Z"
    echo "  revision: olddeadbeef"
    echo "  arch:    linux/amd64"
fi
STUBEOF2
chmod +x "$STUB2"

OUT2=$(PATH="$VERSION_TMPDIR2:$PATH" "$ERG" version 2>&1)
if echo "$OUT2" | grep -q "\[outdated"; then
    pass "version: older-revision binary marked [outdated]"
else
    fail "version: older-revision binary not marked [outdated]: $OUT2"
fi
rm -rf "$VERSION_TMPDIR2"

echo "update: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
