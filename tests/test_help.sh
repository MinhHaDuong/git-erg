#!/bin/sh
# Integration tests for: erg help output (encoding and completeness)
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg help ==="

# --- -h: exit 0 ---
if out=$("$ERG" -h); then
    pass "-h: exit 0"
else
    fail "-h: expected exit 0"
fi

# --- --help: exit 0 ---
if out=$("$ERG" --help); then
    pass "--help: exit 0"
else
    fail "--help: expected exit 0"
fi

# --- help: exit 0 ---
if out=$("$ERG" help); then
    pass "help cmd: exit 0"
else
    fail "help cmd: expected exit 0"
fi

# Capture help output once for remaining assertions
help_out=$("$ERG" -h)

# --- --help appears in usage header ---
if echo "$help_out" | grep -q '\[--help\]'; then
    pass "help header contains [--help]"
else
    fail "help header missing [--help]"
fi

# --- each canonical command name appears in help output ---
for cmd in validate check ready next-id new close log archive migrate init version update; do
    if echo "$help_out" | grep -q "$cmd"; then
        pass "help mentions command: $cmd"
    else
        fail "help missing command: $cmd"
    fi
done

# --- no angle brackets in help output ---
if echo "$help_out" | grep -q '[<>]'; then
    fail "help output contains angle brackets"
else
    pass "help output: no angle brackets"
fi

for cmd in close validate new log; do
    stderr=$($ERG $cmd 2>&1 || true)
    if echo "$stderr" | grep -q '[<>]'; then
        fail "per-command usage: '$cmd' contains angle brackets (got: $stderr)"
    else
        pass "per-command usage: '$cmd' has no angle brackets"
    fi
done

# subcommand --help/-h must exit 0 and not create a --help/ directory
for sub in init check ready next-id archive migrate; do
    tmp=$(mktemp -d)
    (cd "$tmp" && "$ERG" "$sub" --help >/dev/null 2>&1) && pass "$sub --help: exits 0" || fail "$sub --help: exits 0"
    if [ -d "$tmp/--help" ]; then
        fail "$sub --help: spurious --help/ directory created"
    else
        pass "$sub --help: no spurious directory"
    fi
    rm -rf "$tmp"
done

# --- per-command --help: content checks ---

# erg close --help must include the phrase "Inserts a Closed: REASON header"
if $ERG close --help 2>/dev/null | grep -q "Inserts a Closed: REASON header"; then
    pass "close --help: contains 'Inserts a Closed: REASON header'"
else
    fail "close --help: missing 'Inserts a Closed: REASON header'"
fi

# erg validate --help must print per-command text (unique string not in global usage)
if $ERG validate --help 2>/dev/null | grep -q "Magic first line"; then
    pass "validate --help: contains per-command text 'Magic first line'"
else
    fail "validate --help: missing per-command text 'Magic first line'"
fi

# erg unknowncmd --help falls back to global usage (contains [--help])
if $ERG unknowncmd --help 2>/dev/null | grep -q '\[--help\]'; then
    pass "unknowncmd --help: falls back to global usage"
else
    fail "unknowncmd --help: expected fallback to global usage"
fi

# per-command --help output goes to stdout (not stderr): stderr must be empty
stderr_out=$($ERG close --help 2>&1 1>/dev/null)
if [ -z "$stderr_out" ]; then
    pass "close --help: no output on stderr (help goes to stdout only)"
else
    fail "close --help: unexpected stderr output: $stderr_out"
fi

# --help --all: must list all 12 command sections
count=$($ERG --help --all 2>/dev/null | grep -c "^## erg " || true)
if [ "$count" -eq 12 ]; then
    pass "--help --all: 12 sections"
else
    fail "--help --all: expected 12 sections, got $count"
fi

# --help --all: first line must be the H1 document title
first_line=$("$ERG" --help --all 2>/dev/null | head -1)
if [ "$first_line" = "# erg manual" ]; then
    pass "--help --all: first line is '# erg manual'"
else
    fail "--help --all: expected first line '# erg manual', got '$first_line'"
fi

# standalone COMMAND --help: must start with ## heading (not # H1)
first_cmd_line=$("$ERG" validate --help 2>/dev/null | head -1)
if echo "$first_cmd_line" | grep -q "^## erg validate"; then
    pass "validate --help: first line starts with '## erg validate'"
else
    fail "validate --help: expected '## erg validate...', got '$first_cmd_line'"
fi

echo "help: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
