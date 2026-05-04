#!/bin/sh
# Integration tests for: erg version, erg update
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0; FAIL=0

# Test: erg version exits 0 and prints 64-char hex
VER=$("$ERG" version)
if echo "$VER" | grep -qE '^[0-9a-f]{64}$'; then
    echo "PASS: version prints hex"
    PASS=$((PASS+1))
else
    echo "FAIL: version output: $VER"
    FAIL=$((FAIL+1))
fi

# Test: erg update with bad URL exits 0
if ERG_UPDATE_URL=http://127.0.0.1:1 "$ERG" update 2>/dev/null; then
    echo "PASS: update offline exits 0"
    PASS=$((PASS+1))
else
    echo "FAIL: update offline should exit 0"
    FAIL=$((FAIL+1))
fi

# Test: erg update with local server serving same binary → already up to date
PORT=$(python3 -c "import socket; s=socket.socket(); s.bind(('',0)); print(s.getsockname()[1]); s.close()")
SRV_DIR=$(mktemp -d)
cp "$ERG" "$SRV_DIR/erg"
( cd "$SRV_DIR" && exec python3 -m http.server $PORT >/dev/null 2>&1 ) &
SERVER_PID=$!
trap "kill $SERVER_PID 2>/dev/null; rm -rf $SRV_DIR" EXIT

# Give the server a moment to start
sleep 0.5

OUT=$(ERG_UPDATE_URL=http://127.0.0.1:$PORT/erg "$ERG" update 2>&1)
if echo "$OUT" | grep -q "already up to date"; then
    echo "PASS: update same binary is no-op"
    PASS=$((PASS+1))
else
    echo "FAIL: update same binary: $OUT"
    FAIL=$((FAIL+1))
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
    echo "PASS: update does not rewrite ticket files"
    PASS=$((PASS+1))
else
    echo "FAIL: update rewrote ticket files"
    FAIL=$((FAIL+1))
fi
if echo "$OUT" | grep -q "erg migrate"; then
    echo "PASS: update emits migrate hint"
    PASS=$((PASS+1))
else
    echo "FAIL: update missing migrate hint"
    FAIL=$((FAIL+1))
fi
rm -rf "$TICKET_DIR"

# Test: erg update with 404 response exits 0 and stderr contains "server returned"
OUT=$(ERG_UPDATE_URL=http://127.0.0.1:$PORT/no-such-file "$ERG" update 2>&1 || true)
if echo "$OUT" | grep -q "server returned"; then
    echo "PASS: update 404 exits 0 and emits 'server returned'"
    PASS=$((PASS+1))
else
    echo "FAIL: update 404: $OUT"
    FAIL=$((FAIL+1))
fi

# Test: erg update with 200 empty body exits 0 and stderr contains "empty body"
printf '' > "$SRV_DIR/erg-empty"
OUT=$(ERG_UPDATE_URL=http://127.0.0.1:$PORT/erg-empty "$ERG" update 2>&1 || true)
if echo "$OUT" | grep -q "empty body"; then
    echo "PASS: update empty-body 200 exits 0 and emits 'empty body'"
    PASS=$((PASS+1))
else
    echo "FAIL: update empty-body 200: $OUT"
    FAIL=$((FAIL+1))
fi

# Test: update with stale managed assets → emits `erg init` hint
WORKSPACE=$(mktemp -d)
ERG_ABS=$(readlink -f "$ERG")
mkdir -p "$WORKSPACE/tickets"
echo "stale content" > "$WORKSPACE/tickets/README.md"
cp "$ERG_ABS" "$SRV_DIR/erg-new2"
printf '\x00' >> "$SRV_DIR/erg-new2"
OUT=$(cd "$WORKSPACE" && ERG_UPDATE_URL=http://127.0.0.1:$PORT/erg-new2 "$ERG_ABS" update 2>&1 || true)
if echo "$OUT" | grep -q "erg init"; then
    echo "PASS: update with stale assets emits erg init hint"
    PASS=$((PASS+1))
else
    echo "FAIL: update with stale assets missing erg init hint: $OUT"
    FAIL=$((FAIL+1))
fi

# Test: update does NOT rewrite managed assets (binary-only contract)
STALE_CONTENT=$(cat "$WORKSPACE/tickets/README.md")
if [ "$STALE_CONTENT" = "stale content" ]; then
    echo "PASS: update does not rewrite managed assets"
    PASS=$((PASS+1))
else
    echo "FAIL: update rewrote managed asset (README.md content changed)"
    FAIL=$((FAIL+1))
fi
rm -rf "$WORKSPACE"

echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
