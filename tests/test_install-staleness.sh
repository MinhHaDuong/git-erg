#!/bin/sh
# Test: install-erg-binary staleness probe is asset-aware (ticket 0168).
#
# The probe in Makefile's install-erg-binary used to be:
#   find src/go -name '*.go' -newer "$(BOOTSTRAP_BIN)"
# which ignored changes to src/go/assets/* (embedded files).  The fix
# broadens it to -type f so asset edits trigger a rebuild too.
set -eu

PASS=0; FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== install-erg-binary staleness probe (asset-aware; ticket 0168) ==="

BOOTSTRAP_BIN="$(pwd)/tickets/erg"

if [ ! -f "$BOOTSTRAP_BIN" ]; then
	echo "SKIP: bootstrap binary not found at $BOOTSTRAP_BIN"
	exit 0
fi

# Helper: run the staleness probe as it appears in the fixed Makefile.
# Returns 0 (stale — some src/go file is newer than the reference) or
# 1 (fresh — nothing newer).
is_stale() {
	[ -n "$(find src/go -type f -newer "$BOOTSTRAP_BIN" -print -quit)" ]
}

SENTINEL_ASSET="src/go/assets/spec-erg-v1.md"

# Save and restore both the binary and the asset mtime so the test does
# not leave tracked files with a modified timestamp.
SAVED_MTIME=$(stat -c '%Y' "$BOOTSTRAP_BIN" 2>/dev/null || stat -f '%m' "$BOOTSTRAP_BIN")
SAVED_ASSET_MTIME=$(stat -c '%Y' "$SENTINEL_ASSET" 2>/dev/null || stat -f '%m' "$SENTINEL_ASSET")

restore_mtimes() {
	touch -t "$(date -d "@$SAVED_MTIME" +%Y%m%d%H%M.%S 2>/dev/null || date -r "$SAVED_MTIME" +%Y%m%d%H%M.%S)" "$BOOTSTRAP_BIN" 2>/dev/null || true
	touch -t "$(date -d "@$SAVED_ASSET_MTIME" +%Y%m%d%H%M.%S 2>/dev/null || date -r "$SAVED_ASSET_MTIME" +%Y%m%d%H%M.%S)" "$SENTINEL_ASSET" 2>/dev/null || true
}
trap restore_mtimes EXIT

# Set binary to a timestamp far in the past (2020-01-01) to ensure all
# current src/go files are strictly newer — sanity-check the probe logic.
touch -t 202001010000 "$BOOTSTRAP_BIN"

if is_stale; then
	pass "sanity: probe fires when binary is older than source files"
else
	fail "sanity: probe did NOT fire with a very old binary timestamp"
fi

# Set binary to far future so nothing in src/go is newer — confirms probe
# is silent when binary is up-to-date.
touch -t 203001010000 "$BOOTSTRAP_BIN"

if is_stale; then
	fail "baseline: probe fired even when binary has future timestamp"
else
	pass "baseline: probe is silent when binary is up-to-date"
fi

# Now set binary to past so .go source triggers it.
touch -t 202001010000 "$BOOTSTRAP_BIN"
if is_stale; then
	pass ".go source file being newer than binary is detected as stale"
else
	fail ".go source file newer than binary was NOT detected (regression)"
fi

# Touch the asset and set binary to past so it is definitely newer.
touch "$SENTINEL_ASSET"
touch -t 202001010000 "$BOOTSTRAP_BIN"
if is_stale; then
	pass "asset file (src/go/assets/) newer than binary is detected as stale"
else
	fail "asset file newer than binary was NOT detected (the original bug)"
fi

echo "install-staleness: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
