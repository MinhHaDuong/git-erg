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

# make docs must leave no drift: the committed docs/erg-manual.md must match
# the freshly regenerated output (drift guard for PR #269 / ticket 0235 -- a
# help-text change with no manual regen slipped past the non-empty check).
if git diff --exit-code docs/erg-manual.md >/dev/null 2>&1; then
    pass "make docs: docs/erg-manual.md is current (no drift)"
else
    fail "make docs: docs/erg-manual.md is stale -- run 'make docs' and commit"
fi

# Every corpus-integrity violation folderClosure() can emit must be documented
# in the spec it cites. check.go's basename-only error names the spec section
# "Closed / not-closed criterion" as its own authority, so a user who follows
# that citation must find their case enumerated there (drift guard for 0256;
# the same class as 0096/0102/0127/0228 spec-alignment defects).
if grep -q 'basename alone' src/go/assets/spec-erg-v1.md; then
    pass "spec documents the basename-only closure violation erg check emits"
else
    fail "check.go emits a folderClosure violation the spec does not document"
fi

# The path test is bounded to the store: 0285 made closure read the path BELOW
# the directory erg was given, so a store under an ancestor named *-closed is
# read like any other. The spec stated the component rule without ever saying
# where the path starts, which is the gap that let the unbounded walk look
# correct for four years (drift guard for 0285).
if grep -q 'below the directory .erg. was given' src/go/assets/spec-erg-v1.md; then
    pass "spec says which path the closure component test is applied to"
else
    fail "spec states the closure component rule without saying where the path starts"
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

# helpCheck must not describe the stampless NOTE as a whole-store condition
# (ticket 0292, defect 3). The gate is per asset: a manifest that stamps some
# assets and not others is what `erg init` writes whenever it preserves one, and
# the NOTE fires for the unstamped asset in that store. The pre-0292 wording --
# "there is NO .erg-assets manifest at all" -- told a reader the opposite of
# what tests/test_check.sh's PARTIAL fixture asserts, and nothing caught it
# because check.go was untouched by the change that invalidated it.
CHECKHELP=$("$ERG" check --help 2>&1) || true
if echo "$CHECKHELP" | grep -qF 'no .erg-assets entry stamps this asset'; then
    pass "helpCheck describes the stampless NOTE as a per-asset condition"
else
    fail "helpCheck still ties the stampless NOTE to a whole missing manifest"
fi

# README's re-vendor recipe must name the offline route (ticket 0292).
# `erg init --show erg-github` exists partly to give that recipe a source that
# needs neither the network nor a clone -- and init.go's own comment cites the
# recipe as the reason. A code justification pointing at documentation that
# never mentions it is the drift this check closes.
if sed -n '/^## Forge layer/,/^## Install into a project/p' README.md |
    grep -qF 'init --show erg-github'; then
    pass "README: the re-vendor recipe names the offline source"
else
    fail "README: the re-vendor recipe offers only network/clone routes (ticket 0292)"
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
