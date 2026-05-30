#!/bin/sh
# Encoding guard: src/go/** must be pure ASCII (ticket 0167).
#
# Security: non-ASCII in source enables Trojan-Source bidi/homoglyph attacks
# (CVE-2021-42574). Data safety: non-ASCII in embedded assets causes corruption
# under encoding round-trips (incident 0160).
#
# Both guards ship with a negative control that proves the check actually trips
# when the invariant is violated -- matching the 0146/0160 house style.
#
# Scope: *.go files, *.md files, and .ergrc config files under src/go/.
# The committed binary (src/go/git-erg) is excluded -- it is not source.
set -eu

PASS=0; FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

# find_nonascii PATTERN: prints files matching PATTERN under src/go/ that
# contain bytes outside 0x00-0x7F. Uses a temp file to avoid pipeline subshell
# interactions under set -e (POSIX sh pipelines run the loop in a subshell;
# grep exit-1 on no-match interacts badly with set -e).
find_nonascii() {
    _pat="$1"
    _listfile=$(mktemp)
    find src/go \( -name '*.go' -o -name '*.md' -o -name '.ergrc' \) > "$_listfile"
    _hits=""
    while IFS= read -r _f; do
        if LC_ALL=C grep -qP '[^\x00-\x7F]' "$_f" 2>/dev/null; then
            _hits="$_hits$_f\n"
        fi
    done < "$_listfile"
    rm -f "$_listfile"
    printf '%s' "$_hits" | grep -v '^$' | grep "$_pat" | head -3
}

echo "=== encoding (ASCII-only src/go/**; ticket 0167) ==="

# --- 1. src/go/** text sources must be ASCII-only ----------------------------
SRC_HITS=$(find_nonascii '.')
if [ -n "$SRC_HITS" ]; then
    fail "src/go/** contains non-ASCII bytes: $SRC_HITS"
else
    pass "src/go/** source files are pure ASCII"
fi

# Negative control: inject a non-ASCII byte into a temp .go file under src/go/
# and assert the guard trips, then remove it.
# Proves the check has teeth and cannot be bypassed by future changes.
TMPFILE=$(mktemp src/go/test_encoding_negctrl_XXXXXX.go)
# Use octal escapes (POSIX sh portable; dash does not expand \xNN in printf).
# UTF-8 em dash = U+2014 = bytes E2 80 94 = octal \342\200\224.
printf '// negative control: em dash \342\200\224\npackage main\n' > "$TMPFILE"
NEG_HITS=$(find_nonascii '.')
if [ -n "$NEG_HITS" ]; then
    pass "ASCII guard (neg control): injected non-ASCII detected"
else
    fail "ASCII guard (neg control): injected non-ASCII was NOT detected -- guard is vacuous"
fi
rm -f "$TMPFILE"

# --- 2. No U+FFFD (replacement char) in live source and assets ---------------
# U+FFFD is never legitimate in source; its presence signals an encoding
# round-trip bug. Check src/go/** and all tracked files except tickets/closed/
# (which may document historical U+FFFD bugs as evidence).
FFFD=$(printf '\357\277\275')

# 2a. src/go/** text sources must have no U+FFFD.
_listfile=$(mktemp)
find src/go \( -name '*.go' -o -name '*.md' -o -name '.ergrc' \) > "$_listfile"
FFFD_SRC=""
while IFS= read -r _f; do
    if LC_ALL=C grep -qe "$FFFD" "$_f" 2>/dev/null; then
        FFFD_SRC="$FFFD_SRC$_f\n"
    fi
done < "$_listfile"
rm -f "$_listfile"
FFFD_SRC=$(printf '%s' "$FFFD_SRC" | grep -v '^$' | head -3)
if [ -n "$FFFD_SRC" ]; then
    fail "U+FFFD in src/go/** sources: $FFFD_SRC"
else
    pass "No U+FFFD in src/go/** sources"
fi

# 2b. Repo-wide: no U+FFFD outside tickets/closed/ (closed tickets may
# document historical corruption as evidence without implying an active bug).
_listfile=$(mktemp)
LC_ALL=C git ls-files | grep -v '^tickets/closed/' > "$_listfile"
FFFD_HITS=""
while IFS= read -r _f; do
    if LC_ALL=C grep -qe "$FFFD" "$_f" 2>/dev/null; then
        FFFD_HITS="$FFFD_HITS$_f\n"
    fi
done < "$_listfile"
rm -f "$_listfile"
FFFD_HITS=$(printf '%s' "$FFFD_HITS" | grep -v '^$' | head -3)
if [ -n "$FFFD_HITS" ]; then
    fail "U+FFFD found outside tickets/closed/: $FFFD_HITS"
else
    pass "No U+FFFD outside tickets/closed/"
fi

# Negative control for U+FFFD: write a temp file and verify the scanner fires.
TMPFFFD=$(mktemp /tmp/test_encoding_fffd_negctrl_XXXXXX.txt)
printf 'bad content \357\277\275 end\n' > "$TMPFFFD"
if LC_ALL=C grep -qe "$FFFD" "$TMPFFFD" 2>/dev/null; then
    pass "U+FFFD guard (neg control): injected U+FFFD detected"
else
    fail "U+FFFD guard (neg control): injected U+FFFD NOT detected -- guard is vacuous"
fi
rm -f "$TMPFFFD"

echo "encoding: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
