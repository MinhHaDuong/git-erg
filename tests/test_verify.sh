#!/bin/sh
# Integration test for: make verify — reproducible-build rebuild and byte-diff.
#
# `make verify` rebuilds the committed tickets/erg from its embedded git
# revision (in a fresh shared clone) with the recovered -ldflags stamps and the
# binary's original Go toolchain, then byte-diffs the result against the
# committed binary. This test asserts that rebuild reports PASS.
#
# Reproducibility is 0151's keystone supply-chain control: it lets anyone
# confirm the committed bootstrap binary was built from the tracked source and
# carries no hidden modification.
#
# This test is special: it invokes `make verify` (recursive make) rather than
# exercising $ERG_BIN directly. The verify target pins GOTOOLCHAIN to the
# version the binary was built with (go1.21.13); on a host with only a newer
# toolchain, Go auto-downloads it on first use. If that download is unavailable
# (air-gapped / no network), verify cannot run and the test SKIPs rather than
# fails. A genuine hash MISMATCH still FAILs.
set -eu

OUT=$(make verify 2>&1) || true

if echo "$OUT" | grep -q 'verify: PASS'; then
	echo "test_verify: PASS"
	exit 0
fi

# The verify target itself emits 'verify: SKIP' when it cannot obtain the
# embedded revision (e.g. a shallow clone where the ancestor commit is absent).
# That is an environment limitation, not a reproducibility regression.
if echo "$OUT" | grep -q 'verify: SKIP'; then
	echo "test_verify: SKIP (embedded revision unavailable — shallow clone)"
	exit 0
fi

# Toolchain download / availability failure → environment limitation, not a
# reproducibility regression. Match only toolchain-fetch / network errors,
# never the verify FAIL line ("NOT reproducible", no skip keyword) nor a
# genuine compile error. These strings come from `go`/`GOTOOLCHAIN` when it
# cannot obtain the pinned toolchain offline.
if echo "$OUT" | grep -qiE 'go: download|toolchain.*(not available|unavailable|download)|cannot find toolchain|no such host|dial tcp|network is unreachable|connection refused|timeout'; then
	echo "test_verify: SKIP (Go toolchain unavailable offline)"
	exit 0
fi

echo "$OUT"
echo "test_verify: FAIL"
exit 1
