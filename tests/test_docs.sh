#!/bin/sh
# Integration tests for: make docs target
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg docs ==="

# --help --all: each command has a section header
count=$("$ERG" --help --all 2>/dev/null | grep -c "^## erg " || true)
if [ "$count" -eq 19 ]; then
    pass "--help --all: 19 section headers found"
else
    fail "--help --all: expected 19 section headers, got $count"
fi

# --help=all: alternate form also prints 19 section headers
count=$("$ERG" --help=all 2>/dev/null | grep -c "^## erg " || true)
if [ "$count" -eq 19 ]; then
    pass "--help=all: 19 section headers found"
else
    fail "--help=all: expected 19 section headers, got $count"
fi

# --help --all: output goes to stdout (stderr should be empty)
stderr_out=$("$ERG" --help --all 2>&1 >/dev/null || true)
if [ -z "$stderr_out" ]; then
    pass "--help --all: no output on stderr"
else
    fail "--help --all: unexpected stderr: $stderr_out"
fi

# make docs produces a non-empty docs/erg-manual.md
make docs >/dev/null 2>&1
if [ -s docs/erg-manual.md ]; then
    pass "make docs: docs/erg-manual.md is non-empty"
else
    fail "make docs: docs/erg-manual.md is missing or empty"
fi

# README must not show `erg validate <ID>` — validate takes file paths, not IDs
# (regression guard for 0161; the broken example was `tickets/erg validate 01`).
if grep -Eq 'erg validate [0-9]+( |$)' README.md; then
    fail "README has an erg validate example with a bare ID (validate takes paths)"
else
    pass "README erg validate examples use file paths, not bare IDs"
fi

# README install step must name a concrete binary source, not a bare
# "prebuilt one" (regression guard for 0164 / finding F10). The fix points at
# the committed tickets/erg; the negative control is the old vague wording.
install_step=$(grep -A2 'Drop the .*erg.* binary' README.md || true)
if echo "$install_step" | grep -q 'tickets/erg'; then
    pass "README install step names the committed tickets/erg as the prebuilt source"
else
    fail "README install step is vague about where the prebuilt binary comes from"
fi

# CONTRIBUTING.md exists and its add-a-subcommand checklist names every
# touch-point (regression guard for 0163 / findings F16+F17). A stub guide
# missing the checklist must fail.
if [ -f CONTRIBUTING.md ]; then
    pass "CONTRIBUTING.md exists"
    missing=""
    for token in helptext.go main.go TEST_SUITES "tests/test_" TestDispatchRegistrySync; do
        grep -qF "$token" CONTRIBUTING.md || missing="$missing $token"
    done
    if [ -z "$missing" ]; then
        pass "CONTRIBUTING.md subcommand checklist names all touch-points"
    else
        fail "CONTRIBUTING.md subcommand checklist omits:$missing"
    fi
else
    fail "CONTRIBUTING.md is missing"
fi

# Makefile defines a `check` pre-PR gate alias (ticket 0215) wired to the real
# gate (test + validate) and declared .PHONY. Source-inspection guard -- no
# slow subprocess. The 0210-0213 exit criteria say "make check passe"; this
# pins the target so that statement is true.
if grep -Eq '^check:[[:space:]]*test[[:space:]]+validate' Makefile; then
    pass "Makefile: check target runs test + validate"
else
    fail "Makefile: missing 'check: test validate' gate alias"
fi
if grep -Eq '^\.PHONY:.*[[:space:]]check([[:space:]]|$)' Makefile; then
    pass "Makefile: check is .PHONY"
else
    fail "Makefile: check is not declared .PHONY"
fi

# helpUpdate must mention 'erg init' as the asset/defaults delivery step
# (regression guard for 0223: update-only is not enough to absorb new defaults).
if "$ERG" update --help 2>/dev/null | grep -qF 'erg init'; then
    pass "helpUpdate names 'erg init' as the asset/defaults delivery step"
else
    fail "helpUpdate missing 'erg init' reference (ticket 0223 regression)"
fi

# integration.md must contain the 'Keeping a store current' subsection.
INTEG_SRC="${INTEG_SRC:-src/go/assets/integration.md}"
if [ -f "$INTEG_SRC" ] && grep -qF 'Keeping a store current' "$INTEG_SRC"; then
    pass "integration.md contains 'Keeping a store current' subsection"
else
    fail "integration.md missing 'Keeping a store current' subsection (ticket 0223 regression)"
fi

echo ""
if [ "$FAIL" -eq 0 ]; then
    echo "docs: PASS ($PASS checks)"
    exit 0
else
    echo "docs: FAIL ($FAIL/$((PASS + FAIL)) checks failed)"
    exit 1
fi
