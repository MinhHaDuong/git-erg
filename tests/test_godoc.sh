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

# Per-command help is now defined as `const help<Cmd>` in each command file
# (the cmd* doc comments are one-liners that reference these consts). The
# checks below target the consts — the single source of truth for the
# user-facing summary printed by `erg COMMAND --help`.

# helpValidate: all 13 rules must be listed
doc_contains helpValidate "1. Magic first line" "helpValidate rule 1 present"
doc_contains helpValidate "2. All required headers" "helpValidate rule 2 present"
doc_contains helpValidate "3. No unknown headers" "helpValidate rule 3 present"
doc_contains helpValidate "13. No dependency cycles" "helpValidate rule 13 present"

# helpClose: three-step semantics (step 3 is non-atomic per 0123 clarification)
doc_contains helpClose "idempotent but not atomic" "helpClose step 3 idempotent-not-atomic"
doc_contains helpClose "1. Insert" "helpClose step 1"
doc_contains helpClose "2. Append" "helpClose step 2"
doc_contains helpClose "3. Scan" "helpClose step 3"

# helpReady: readiness criteria
doc_contains helpReady "ready when all" "helpReady readiness criteria"
doc_contains helpReady "blocked_by" "helpReady JSON schema"

# helpCheck: global invariants
doc_contains helpCheck "No duplicate ticket IDs" "helpCheck duplicate-ID check"
doc_contains helpCheck "No dependency cycles" "helpCheck cycle check"

# helpArchive: stale-blocker guard
doc_contains helpArchive "Blocked-by:" "helpArchive stale-blocker guard"

# helpMigrate: conversion rules
doc_contains helpMigrate "Status: closed" "helpMigrate closed-rule"
doc_contains helpMigrate "Idempotent" "helpMigrate idempotent"

# helpInit: asset list
doc_contains helpInit "AGENTS.md" "helpInit asset AGENTS.md"
doc_contains helpInit "spec-erg-v1.md" "helpInit asset spec"
doc_contains helpInit "integration.md" "helpInit asset integration"

# helpNew: ID allocation and O_EXCL
doc_contains helpNew "O_EXCL" "helpNew atomic creation"

# helpLog: format description
doc_contains helpLog "YYYY-MM-DDThh:mmZ" "helpLog timestamp format"

# helpNextID: optimistic allocation note
doc_contains helpNextID "optimistic" "helpNextID optimistic allocation"

# helpVersion: output fields
doc_contains helpVersion "hash" "helpVersion hash field"
doc_contains helpVersion "ERG_VERSION_NO_DISCOVER" "helpVersion no-discover env"

# helpUpdate: offline-safe exit code
doc_contains helpUpdate "offline" "helpUpdate offline safety"
doc_contains helpUpdate "migration" "helpUpdate migration hint"

# Format constant doc comments. RequiredHeaders/SingletonHeaders/
# ValidHeaders were removed in ticket 0116; ticket 0117 then merged
# parser and validator into one pass. Ticket 0126 replaced validTagValues
# with defaultTags in config.go (configurable via .ergrc); ticket 0175
# renamed it to defaultLabels (Tag → Label).
doc_contains defaultLabels "needs-human" "defaultLabels doc comment"
doc_contains logLineRE "YYYY-MM-DDThh:mmZ" "logLineRE doc comment"

echo ""
if [ "$FAIL" -eq 0 ]; then
    echo "godoc: PASS ($PASS checks)"
    exit 0
else
    echo "godoc: FAIL ($FAIL/$((PASS + FAIL)) checks failed)"
    exit 1
fi
