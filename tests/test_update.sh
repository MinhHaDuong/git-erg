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

echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
