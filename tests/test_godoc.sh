#!/bin/sh
# Integration tests for: Go doc comments on cmdXxx functions
set -eu

PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg godoc ==="

SRC="src/go"

# go doc must be available (it ships with Go)
if ! command -v go >/dev/null 2>&1; then
    echo "  SKIP: go not found in PATH"
    exit 0
fi

# Helper: check that `go doc -u . SYMBOL` output contains PATTERN
doc_contains() {
    sym="$1"; pattern="$2"; label="$3"
    if (cd "$SRC" && go doc -u . "$sym" 2>/dev/null) | grep -qF "$pattern"; then
        pass "$label"
    else
        fail "$label — expected '$pattern' in: go doc -u . $sym"
    fi
}

# cmdValidate: all 13 rules must be listed
doc_contains cmdValidate "1. Magic first line" "cmdValidate rule 1 present"
doc_contains cmdValidate "2. All required headers" "cmdValidate rule 2 present"
doc_contains cmdValidate "3. No unknown headers" "cmdValidate rule 3 present"
doc_contains cmdValidate "13. No dependency cycles" "cmdValidate rule 13 present"

# cmdClose: three-step atomicity
doc_contains cmdClose "three-step atomic" "cmdClose three-step semantics"
doc_contains cmdClose "1. Insert" "cmdClose step 1"
doc_contains cmdClose "2. Append" "cmdClose step 2"
doc_contains cmdClose "3. Scan" "cmdClose step 3"

# cmdReady: readiness criteria
doc_contains cmdReady "ready when all" "cmdReady readiness criteria"
doc_contains cmdReady "blocked_by" "cmdReady JSON schema"

# cmdCheck: global invariants
doc_contains cmdCheck "No duplicate ticket IDs" "cmdCheck duplicate-ID check"
doc_contains cmdCheck "No dependency cycles" "cmdCheck cycle check"

# cmdArchive: stale-blocker guard
doc_contains cmdArchive "Blocked-by:" "cmdArchive stale-blocker guard"

# cmdMigrate: conversion rules
doc_contains cmdMigrate "Status: closed" "cmdMigrate closed-rule"
doc_contains cmdMigrate "Idempotent" "cmdMigrate idempotent"

# cmdInit: asset list
doc_contains cmdInit "AGENTS.md" "cmdInit asset AGENTS.md"
doc_contains cmdInit "spec-erg-v1.md" "cmdInit asset spec"
doc_contains cmdInit "integration.md" "cmdInit asset integration"

# cmdNew: ID allocation and O_EXCL
doc_contains cmdNew "O_EXCL" "cmdNew atomic creation"

# cmdLog: format description
doc_contains cmdLog "YYYY-MM-DDThh:mmZ" "cmdLog timestamp format"

# cmdNextID: optimistic allocation note
doc_contains cmdNextID "optimistic" "cmdNextID optimistic allocation"

# cmdVersion: output fields
doc_contains cmdVersion "hash" "cmdVersion hash field"
doc_contains cmdVersion "ERG_VERSION_NO_DISCOVER" "cmdVersion no-discover env"

# cmdUpdate: offline-safe exit code
doc_contains cmdUpdate "offline" "cmdUpdate offline safety"
doc_contains cmdUpdate "migration" "cmdUpdate migration hint"

# Format constant doc comments
doc_contains requiredHeaders "mandatory preamble" "requiredHeaders doc comment"
doc_contains validTagValues "needs-human" "validTagValues doc comment"
doc_contains logLineRE "YYYY-MM-DDThh:mmZ" "logLineRE doc comment"

echo ""
if [ "$FAIL" -eq 0 ]; then
    echo "godoc: PASS ($PASS checks)"
    exit 0
else
    echo "godoc: FAIL ($FAIL/$((PASS + FAIL)) checks failed)"
    exit 1
fi
