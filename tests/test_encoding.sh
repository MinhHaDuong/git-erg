#!/bin/sh
# Encoding guard: src/go/** is ASCII-only, with one scoped exception (ticket
# 0167; exception added in 0217).
#
# Security: non-ASCII in source enables Trojan-Source bidi/homoglyph attacks
# (CVE-2021-42574). Data safety: non-ASCII in embedded assets causes corruption
# under encoding round-trips (incident 0160).
#
# Scoped exception (ticket 0217): *.go files MAY contain U+201C and U+201D --
# the two curly double quotes gofmt's doc-comment formatter emits for `` `` ``
# and '' (proposal #51082, Go 1.19+). Allowing exactly those two glyphs stops
# the gofmt-vs-ASCII fight without reopening the attack surface: both are
# printing, non-control, non-bidi, non-invisible, and Go identifiers cannot
# contain them. The exception is *.go ONLY -- the embedded assets (*.md,
# .ergrc), which ship inside the verified binary, stay STRICTLY ASCII (the
# 0160 corruption class). Every other non-ASCII byte is still rejected in .go.
#
# Both guards ship with negative controls that prove the check trips when the
# invariant is violated -- matching the 0146/0160 house style.
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
        case "$_f" in
        *.go)
            # Scoped exception (0217): .go may contain U+201C (e2 80 9c) and
            # U+201D (e2 80 9d) -- gofmt's doc-comment smart quotes. Strip just
            # those two sequences, then flag any residual non-ASCII byte.
            if LC_ALL=C sed 's/\xe2\x80\x9c//g; s/\xe2\x80\x9d//g' "$_f" 2>/dev/null \
                 | LC_ALL=C grep -qP '[^\x00-\x7F]'; then
                _hits="$_hits$_f\n"
            fi
            ;;
        *)
            # Embedded assets (*.md, .ergrc) stay strictly ASCII (0160 class).
            if LC_ALL=C grep -qP '[^\x00-\x7F]' "$_f" 2>/dev/null; then
                _hits="$_hits$_f\n"
            fi
            ;;
        esac
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
    pass "ASCII guard (neg control): disallowed non-ASCII (em dash) detected in .go"
else
    fail "ASCII guard (neg control): injected non-ASCII was NOT detected -- guard is vacuous"
fi
rm -f "$TMPFILE"

# Scoped-exception control A (0217): U+201D (e2 80 9d) in a .go file must be
# ALLOWED (the gofmt smart quote). If this trips, the exception is broken.
TMPSMART=$(mktemp src/go/test_encoding_smart_XXXXXX.go)
printf '// smart quote allowed in .go: \342\200\235\npackage main\n' > "$TMPSMART"
SMART_HITS=$(find_nonascii '.')
rm -f "$TMPSMART"
if [ -z "$SMART_HITS" ]; then
    pass "scoped exception: U+201D allowed in a .go file"
else
    fail "scoped exception: U+201D wrongly flagged in .go ($SMART_HITS)"
fi

# Scoped-exception control B (0217): the SAME U+201D in an embedded asset
# (*.md) must STILL be flagged -- assets stay strictly ASCII (0160 class). This
# proves the exception did not leak to the asset scan and the branches did not
# collapse.
TMPASSET=$(mktemp src/go/test_encoding_asset_XXXXXX.md)
printf 'smart quote must be rejected in assets: \342\200\235\n' > "$TMPASSET"
ASSET_HITS=$(find_nonascii '.')
rm -f "$TMPASSET"
if [ -n "$ASSET_HITS" ]; then
    pass "scoped exception: U+201D still rejected in an embedded asset (.md)"
else
    fail "scoped exception LEAKED: U+201D not rejected in a .md asset -- assets must stay strict"
fi

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
