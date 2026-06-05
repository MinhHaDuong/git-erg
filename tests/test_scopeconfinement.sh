#!/bin/sh
# Scope confinement guard suite (ticket 0237).
#
# AGENTS.md "Scope confinement" paragraph: install is the only erg verb that
# mutates outside tickets/. This suite enforces that contract at runtime: for
# each non-install command a scratch git repo is set up, a snapshot of all
# files outside tickets/ (path plus a cksum content signature) is taken before
# the command, the command runs, and the snapshot is compared after. The content
# signature catches in-place overwrites, not just creation/deletion. Any file
# created, deleted, or modified outside tickets/ is a test failure.
#
# Excluded by design:
#   install  is the one command permitted to write outside tickets/ (behind
#            explicit opt-in flags --hooks and --inject-agents). Excluding it
#            IS the invariant -- not an exception to it.
#
# Note on update: update replaces the running binary (tickets/erg in normal
# use). In this test the binary is build/erg, outside the scratch repo. But
# when the scratch repo has no git remote configured, 'git fetch' fails and
# update exits 0 without writing anything. The scope check therefore passes
# for update, and the binary outside the scratch repo is unaffected.
set -eu

ERG="${ERG_BIN:-build/erg}"
ERG_ABS=$(CDPATH= cd "$(dirname "$ERG")" && pwd)/$(basename "$ERG")
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== scope confinement guard suite ==="

TDIR=$(mktemp -d)
trap 'rm -rf "$TDIR"' EXIT

# new_repo <name>: create a minimal git repo at TDIR/<name> with tickets/ dir,
# the erg binary copied in (required by erg init), and a seeded open ticket.
# Echoes the repo path.
new_repo() {
    r="$TDIR/$1"
    mkdir -p "$r/tickets"
    cp "$ERG_ABS" "$r/tickets/erg"
    (cd "$r" && git init -q -b main >/dev/null 2>&1)
    cat > "$r/tickets/0001-seed.erg" <<'EOF'
%erg 0.1
Title: Seed Ticket
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
EOF
    echo "$r"
}

# snapshot <repo>: list all regular files outside tickets/ paired with a
# content signature (cksum). The signature makes the snapshot detect in-place
# overwrites of existing files, not just creation/deletion -- a path-only list
# is blind to a command that rewrites a file's CONTENT.
# Excludes volatile git internals (.git/objects, .git/refs, .git/logs,
# .git/index, .git/FETCH_HEAD, .git/packed-refs) that read-only git
# calls may touch; includes .git/hooks/ because that is the known
# mutation target of erg install --hooks.
snapshot() {
    repo="$1"
    find "$repo" \
        ! -path "$repo/tickets" \
        ! -path "$repo/tickets/*" \
        ! -path "$repo/.git/objects" \
        ! -path "$repo/.git/objects/*" \
        ! -path "$repo/.git/refs" \
        ! -path "$repo/.git/refs/*" \
        ! -path "$repo/.git/logs" \
        ! -path "$repo/.git/logs/*" \
        ! -path "$repo/.git/index" \
        ! -path "$repo/.git/FETCH_HEAD" \
        ! -path "$repo/.git/packed-refs" \
        -type f | sort | xargs cksum
}

# run_cmd <repo> <cmd>: run erg <cmd> with arguments appropriate to the scratch
# repo. Exit codes are ignored -- scope confinement is about side effects, not
# success. Commands that need to be run from within the repo (update) use a
# subshell cd so the store auto-discovery finds the scratch tickets/ dir.
run_cmd() {
    repo="$1"; cmd="$2"; tickets="$repo/tickets"
    case "$cmd" in
    validate)    "$ERG_ABS" validate "$tickets/0001-seed.erg" >/dev/null 2>&1 || true ;;
    check)       "$ERG_ABS" check "$tickets" >/dev/null 2>&1 || true ;;
    list)        "$ERG_ABS" list "$tickets" >/dev/null 2>&1 || true ;;
    ready)       "$ERG_ABS" ready "$tickets" >/dev/null 2>&1 || true ;;
    next-id)     "$ERG_ABS" next-id "$tickets" >/dev/null 2>&1 || true ;;
    new)         "$ERG_ABS" new "Scope Test" "$tickets" >/dev/null 2>&1 || true ;;
    close)       "$ERG_ABS" close 0001 "done" "$tickets" >/dev/null 2>&1 || true ;;
    log)         "$ERG_ABS" log 0001 "claude note scope check" "$tickets" >/dev/null 2>&1 || true ;;
    label)       "$ERG_ABS" label 0001 needs-human "$tickets" >/dev/null 2>&1 || true ;;
    unlabel)     "$ERG_ABS" unlabel 0001 needs-human "$tickets" >/dev/null 2>&1 || true ;;
    archive)     "$ERG_ABS" archive "$tickets" >/dev/null 2>&1 || true ;;
    rm)          "$ERG_ABS" rm 0001 "$tickets" --force >/dev/null 2>&1 || true ;;
    migrate)     "$ERG_ABS" migrate "$tickets" >/dev/null 2>&1 || true ;;
    init)        "$ERG_ABS" init "$repo" >/dev/null 2>&1 || true ;;
    spec)        "$ERG_ABS" spec >/dev/null 2>&1 || true ;;
    integration) "$ERG_ABS" integration >/dev/null 2>&1 || true ;;
    version)     "$ERG_ABS" version >/dev/null 2>&1 || true ;;
    update)      (cd "$repo" && "$ERG_ABS" update >/dev/null 2>&1) || true ;;
    esac
}

# ---------------------------------------------------------------------------
# Guard: for each non-install command, snapshot the tree outside tickets/
# before and after, then assert the two snapshots are identical.
# ---------------------------------------------------------------------------

# CMDS: every registry command except install (the one command explicitly
# allowed to write outside tickets/ behind explicit opt-in flags).
CMDS="validate check list ready next-id new close log label unlabel archive rm migrate init spec integration version update"

for cmd in $CMDS; do
    REPO=$(new_repo "$cmd")
    before=$(snapshot "$REPO")
    run_cmd "$REPO" "$cmd"
    after=$(snapshot "$REPO")
    if [ "$before" = "$after" ]; then
        pass "scope: erg $cmd wrote nothing outside tickets/"
    else
        # Write the diff to stderr so CI logs capture it.
        tmp_b="$TDIR/before.$cmd"
        tmp_a="$TDIR/after.$cmd"
        printf '%s\n' "$before" > "$tmp_b"
        printf '%s\n' "$after"  > "$tmp_a"
        diff_out=$(diff "$tmp_b" "$tmp_a" 2>/dev/null || true)
        fail "scope: erg $cmd wrote outside tickets/: $diff_out"
    fi
done

# ---------------------------------------------------------------------------
# Negative control: manually write a file at repo root and verify the
# snapshot detects the change. Proves the snapshot function has teeth -- a
# no-op snapshot that always returns the same value would pass every command
# check above but fail here.
# ---------------------------------------------------------------------------
NC_REPO=$(new_repo "negative-control")
nc_before=$(snapshot "$NC_REPO")
printf 'intruder\n' > "$NC_REPO/intruder.txt"
nc_after=$(snapshot "$NC_REPO")
if [ "$nc_before" != "$nc_after" ]; then
    pass "negative-control: snapshot detects file written at repo root"
else
    fail "negative-control: snapshot did not detect intruder.txt at repo root (guard has no teeth)"
fi

# ---------------------------------------------------------------------------
# Negative control: overwrite the CONTENT of a file that already exists outside
# tickets/ and verify the snapshot detects it. A path-only snapshot is blind to
# in-place overwrites, so this proves the content signature has teeth. The
# control creates its own probe file rather than borrowing a git-template
# artifact like .git/description (absent under a stripped init.templateDir,
# where the control would degrade to a vacuous creation test). The overwrite
# keeps the byte length identical, so detection must come from the content
# checksum, not the size column.
# ---------------------------------------------------------------------------
OW_REPO=$(new_repo "overwrite-control")
printf 'aaaa\n' > "$OW_REPO/probe.txt"
ow_before=$(snapshot "$OW_REPO")
printf 'bbbb\n' > "$OW_REPO/probe.txt"
ow_after=$(snapshot "$OW_REPO")
if [ "$ow_before" != "$ow_after" ]; then
    pass "negative-control: snapshot detects same-length in-place overwrite of probe.txt"
else
    fail "negative-control: snapshot did not detect same-length overwrite of probe.txt (content signature has no teeth)"
fi

echo "scopeconfinement: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
