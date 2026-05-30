#!/bin/sh
# Standing regression test: every erg command must reject unknown flags.
#
# Derives the command list dynamically from `erg --help` output so that a
# new command added without the unknown-flag guard causes this test to fail
# automatically — no hardcoded list to forget to update.
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== unknown-flag rejection (all commands) ==="

CMDS=$($ERG --help 2>&1 | awk '/^  [a-z]/{print $1}')

if [ -z "$CMDS" ]; then
    fail "could not parse command list from erg --help"
    echo "unknown_flags: $PASS passed, $FAIL failed"
    exit 1
fi

for cmd in $CMDS; do
    out=$($ERG "$cmd" --bogus-flag-xyzzy 2>&1) || rc=$?
    if [ "${rc:-0}" -ne 0 ] && echo "$out" | grep -q "unknown flag"; then
        pass "$cmd rejects unknown flag"
    else
        fail "$cmd did not reject unknown flag (rc=${rc:-0}, got: $out)"
    fi
done

echo "unknown_flags: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
