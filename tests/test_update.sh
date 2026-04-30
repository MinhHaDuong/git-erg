#!/bin/sh
set -e
BIN="${BIN:-tickets/tools/go/erg}"
PASS=0; FAIL=0

# Test: erg version exits 0 and prints 64-char hex
VER=$("$BIN" version)
if echo "$VER" | grep -qE '^[0-9a-f]{64}$'; then
    echo "PASS: version prints hex"
    PASS=$((PASS+1))
else
    echo "FAIL: version output: $VER"
    FAIL=$((FAIL+1))
fi

# Test: erg update with bad URL exits 0
if ERG_UPDATE_URL=http://127.0.0.1:1 "$BIN" update 2>/dev/null; then
    echo "PASS: update offline exits 0"
    PASS=$((PASS+1))
else
    echo "FAIL: update offline should exit 0"
    FAIL=$((FAIL+1))
fi

# Test: erg update with local server serving same binary → already up to date
PORT=18734
TMPDIR=$(mktemp -d)
cp "$BIN" "$TMPDIR/erg"
( cd "$TMPDIR" && exec python3 -m http.server $PORT >/dev/null 2>&1 ) &
SERVER_PID=$!
trap "kill $SERVER_PID 2>/dev/null; rm -rf $TMPDIR" EXIT

# Give the server a moment to start
sleep 0.5

OUT=$(ERG_UPDATE_URL=http://127.0.0.1:$PORT/erg "$BIN" update 2>&1)
if echo "$OUT" | grep -q "already up to date"; then
    echo "PASS: update same binary is no-op"
    PASS=$((PASS+1))
else
    echo "FAIL: update same binary: $OUT"
    FAIL=$((FAIL+1))
fi

echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
