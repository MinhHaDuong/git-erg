#!/bin/sh
# gofmt + go vet adherence ratchet (ticket 0217) -- the Go analog of the
# Python ruff check the harness rules mandate.
#
# Asserts src/go is gofmt-clean and go vet-clean, so formatting/vet drift is
# caught mechanically instead of by a human or an ad-hoc /verify pass.
#
# Interaction with the ASCII guard (test_encoding.sh): gofmt's doc-comment
# formatter smart-quotes `` `` `` and '' into U+201C/U+201D. Ticket 0217 made
# those two glyphs legal in *.go (only), so `gofmt -l` and the ASCII guard no
# longer fight: a gofmt-formatted file is also ASCII-policy-clean. Avoid the
# literal pairs in comments if you want to keep a comment plain ASCII.
set -eu

PASS=0; FAIL=0
pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== gofmt + go vet (ticket 0217) ==="

# --- 1. src/go is gofmt-clean ------------------------------------------------
GOFMT_OUT=$(gofmt -l src/go/ 2>/dev/null || true)
if [ -z "$GOFMT_OUT" ]; then
    pass "gofmt -l src/go/ is clean"
else
    fail "gofmt would reformat: $GOFMT_OUT (run gofmt -w src/go/)"
fi

# Negative control: a parseable but mis-indented temp .go must be reported by
# gofmt -l, proving the check is non-vacuous.
TMPGO=$(mktemp src/go/test_gofmt_negctrl_XXXXXX.go)
printf 'package main\nfunc F()        {\nreturn\n}\n' > "$TMPGO"
NEG=$(gofmt -l src/go/ 2>/dev/null || true)
rm -f "$TMPGO"
if echo "$NEG" | grep -q "$(basename "$TMPGO")"; then
    pass "gofmt guard (neg control): mis-indented .go is flagged"
else
    fail "gofmt guard (neg control): mis-indented .go NOT flagged -- guard is vacuous"
fi

# --- 2. go vet is clean ------------------------------------------------------
if (cd src/go && go vet ./... >/dev/null 2>&1); then
    pass "go vet ./... is clean"
else
    fail "go vet ./... reported issues"
fi

echo "gofmt: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
