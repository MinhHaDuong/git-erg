#!/bin/sh
# Strict-write round-trip guard suite (ticket 0227).
#
# Postel's law (spec + PEP section 11) promises strict-on-write: every
# mutating command must leave a store whose every .erg file re-validates
# clean. Enforcement was scattered per-command with no single universal guard.
# This suite is that guard: one loop over the mutating commands, each run
# against a fresh fixture store, followed by `erg validate` on every .erg in
# the store. Future write commands join automatically by being added to the
# CMDS list below.
#
# Scope: this guard owns "output re-validates clean". Body-preservation and
# log-append losslessness are owned by tests/test_datasafety.sh (Guard 2) --
# referenced, not duplicated here.
#
# Excluded by design:
#   rm       deletes a file; there is nothing to re-validate (its DAG/refusal
#            behaviour is covered by test_datasafety.sh Guard 6).
#   init     touches non-.erg files (.ergrc), not tickets.
#   install  / update touch non-.erg files (installed binary), not tickets.
set -eu

ERG="${ERG_BIN:-build/erg}"
FIXTURES=$(mktemp -d)
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

trap 'rm -rf "$FIXTURES"' EXIT

echo "=== strict-write round-trip guard suite ==="

# Helper: write a minimal open ticket at path $1 with title $2.
write_open() {
    cat > "$1" <<EOF
%erg 0.1
Title: $2
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
}

# Helper: validate every .erg under store $1 (recursively, so archive's
# moved files in closed/ are checked too). Returns non-zero on first failure.
validate_all() {
    rc=0
    for f in $(find "$1" -name '*.erg' | sort); do
        if ! $ERG validate "$f" >/dev/null 2>&1; then
            echo "    invalid after mutation: $f" >&2
            rc=1
        fi
    done
    return $rc
}

# Helper: seed a fresh store for command $2 at directory $1. The seed differs
# per command -- each must arrive at a precondition where the command actually
# performs its write (mirrors test_datasafety.sh's per-command seeding).
seed() {
    ws="$1"; cmd="$2"
    mkdir -p "$ws"
    case "$cmd" in
    new)
        # new allocates a fresh ID; an empty store is the natural precondition.
        ;;
    log|close|label)
        write_open "$ws/0001-seed.erg" "Seed Ticket"
        ;;
    unlabel)
        # unlabel needs an existing Label: to remove.
        write_open "$ws/0001-seed.erg" "Seed Ticket"
        $ERG label 0001 needs-human "$ws" >/dev/null 2>&1
        ;;
    archive)
        # archive moves closed tickets to closed/; seed a closed ticket.
        cat > "$ws/0001-seed.erg" <<EOF
%erg 0.1
Title: Closed Ticket
Created: 2026-01-01
Author: claude
Closed: done

--- log ---
2026-01-01T10:00Z claude created
2026-01-01T11:00Z claude closed - done

--- body ---
EOF
        ;;
    migrate)
        # migrate is a no-op on a clean 0.1 store; seed a legacy file whose
        # migrated form must validate clean (Tags: -> Label:, %erg v1 -> 0.1).
        cat > "$ws/0001-seed.erg" <<EOF
%erg v1
Title: Legacy Ticket
Created: 2026-01-01
Author: claude
Tags: needs-human

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
        ;;
    esac
}

# Helper: run command $2 against store $1.
run_cmd() {
    ws="$1"; cmd="$2"
    case "$cmd" in
    new)     $ERG new "Round Trip New" "$ws" >/dev/null 2>&1 ;;
    log)     $ERG log 0001 "claude note round-trip check" "$ws" >/dev/null 2>&1 ;;
    close)   $ERG close 0001 "done" "$ws" >/dev/null 2>&1 ;;
    label)   $ERG label 0001 needs-human "$ws" >/dev/null 2>&1 ;;
    unlabel) $ERG unlabel 0001 needs-human "$ws" >/dev/null 2>&1 ;;
    archive) $ERG archive "$ws" >/dev/null 2>&1 ;;
    migrate) $ERG migrate "$ws" >/dev/null 2>&1 ;;
    esac
}

# ---------------------------------------------------------------------------
# Guard - strict-on-write round-trip: for each mutating command, a fresh store
# is seeded, the command is run, then every .erg in the store must validate.
# A command that emits a malformed file (bad header, dropped separator, stray
# line) fails its assertion.
# ---------------------------------------------------------------------------
CMDS="new close log label unlabel archive migrate"
for cmd in $CMDS; do
    WS="$FIXTURES/$cmd"
    seed "$WS" "$cmd"
    run_cmd "$WS" "$cmd"
    if validate_all "$WS"; then
        pass "round-trip: store re-validates clean after erg $cmd"
    else
        fail "round-trip: erg $cmd left an invalid .erg in the store"
    fi
done

# ---------------------------------------------------------------------------
# Negative control - prove the guard has teeth. After a clean cycle, corrupt
# the magic line of a surviving file and re-run validate_all; it must report a
# failure. A no-op validator (or a validate_all that never inspects files)
# would pass here, masking real corruption.
# ---------------------------------------------------------------------------
WS="$FIXTURES/control"
seed "$WS" "log"
run_cmd "$WS" "log"
if validate_all "$WS"; then
    : # expected: clean before corruption
else
    fail "negative-control: store should be clean before corruption"
fi
# Corrupt the magic line of the (still-present) seed file.
sed -i '1s/.*/NOT A MAGIC LINE/' "$WS/0001-seed.erg"
if validate_all "$WS" 2>/dev/null; then
    fail "negative-control: validate_all passed on a corrupted store (guard has no teeth)"
else
    pass "negative-control: validate_all catches a corrupted magic line"
fi

echo "strictwrite: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
