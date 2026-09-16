#!/bin/sh
# Whole-class asset invariant — ticket 0278's integration pass.
#
# THE INVARIANT: no tracked file is ever overwritten or reverted without saying
# so. Its two halves are equally load-bearing, and this suite asserts both:
#   (a) a managed asset whose bytes changed must be named in the command's
#       output, with an undo hint;
#   (b) a managed asset whose bytes did NOT change must not be narrated as
#       refreshed or downgraded -- a false claim of change costs trust just as a
#       silent change costs data.
#
# WHY A SEPARATE SUITE. 0278's children each proved their own path inside their
# own scope: 0279 the direction-blind upgrade (init), 0283 the stampless report
# (check/update), 0224/0280 migrate's charter overwrite. Nothing exercised the
# three commands over ONE store in ONE provenance state, which is where a
# composition defect would live: init writes the stamp that update reads and
# migrate rewrites, so each command changes the state the next one judges by.
# This file covers what tests/README.md § Testing layers already calls out as
# shell-integration territory: "cross-command interactions".
#
# It is NOT free of overlap with the per-child suites, and saying otherwise
# would overstate it. Four assertions repeat single-command post-conditions the
# children already lock down -- clean/init against test_init.sh:98,
# edited/init's preserve-and-exit-2 against test_init.sh:96-125, clean/update
# against test_update.sh:343-357, stampless/update against
# test_update.sh:327-330. They are kept deliberately: each is the guard that
# proves its own composition arm reached the code path at all. Drop
# "erg: updated" from an update arm and the arm still passes when no swap
# happened, on an assertion about a report that never ran. The cost is that
# four grep literals now live in two files, and nothing governs a reword of
# them. An earlier revision of this comment claimed all four were prefixes of
# the cross-version constants ticket 0292 pins to extend-at-the-end; measured,
# that is false on every count -- two of the four come from init.go, none is
# one of the three named constants, and the stampless/update literal is an
# infix of a line 0292's own Action 6 plans to reword rather than extend. The
# duplication is a real maintenance cost, accepted for the reachability it
# buys, not a cost someone else's discipline already covers.
#
# "Reachability" is the right defence for two of the four, not all four.
# "erg: updated" is what proves an update arm swapped a binary at all, and
# without it the arm passes on a post-swap report that never ran. But
# edited/init's exit 2 and clean/init's all-unchanged summary were measured
# against their arms and found redundant -- each arm already carries an
# independent guard. They stay for a weaker reason: an exit code is the
# machine-readable half of "saying so", and the clean arm otherwise has no
# positive statement that nothing happened. A trim is a defensible follow-up;
# calling them guards is not.
#
# THE TRAP THIS SUITE IS BUILT AROUND. `erg migrate` overwrites a diverged
# tickets/AGENTS.md unconditionally. That is a SETTLED CHARTER DECISION (ticket
# 0224, PR #275, docs/erg-imagine-charter.md L122-127), already guarded by
# src/go/migrate_test.go:448 and tests/test_migrate.sh:389: agent operating
# instructions must track the binary. It is the WANTED outcome, and a naive
# reading of "no tracked file is ever overwritten" would flag it as a regression
# and reopen a closed decision. What the invariant forbids is an overwrite that
# happens SILENTLY. So the migrate arm below asserts the overwrite HAPPENED and
# that it was announced -- asserting preservation there would be the bug.
#
# FOUR FIXTURE STATES, each driven through init, update and migrate:
#   clean     -- stamp present, assets byte-identical to embedded.
#   edited    -- stamp present, BOTH assets carrying a genuine local edit.
#                .ergrc is outside migrateAssetPaths, AGENTS.md is inside it, so
#                one fixture exercises both sides of the charter boundary.
#   stampless -- no .erg-assets manifest, .ergrc diverged from embedded.
#                (AGENTS.md left pristine here so the NOTE has exactly one
#                subject and the arm cannot pass on the wrong asset.)
#   rollback  -- stamp present, recording the bytes ON DISK, dated after this
#                binary's build. The running erg PREDATES the last init, so an
#                overwrite here is a revert. This is ticket 0279's defect class,
#                the one path in the set demonstrated to lose data, and the
#                suite shipped without it: a red-team mutant that forced
#                `downgrade := false` (init.go:267) passed the first three arms
#                42/0, because no fixture ever stamped newer than the binary.
#                The arm exists so the word "downgraded" is load-bearing.
#
# NEGATIVE-CONTROL RECORD (red step, ticket 0278). Each arm was watched to fail
# before being trusted:
#   1. The edited/migrate arm was first written the naive way -- assert the
#      AGENTS.md edit SURVIVES migrate. It went red on the real binary, which is
#      what proves the fixture genuinely reaches migrate's force-overwrite leg
#      rather than stopping at "already clean" (0278's path-D bullet reports a
#      fixture that never got there).
#   2. audit_file's announcement grep was pointed at a token installAssets never
#      prints. The migrate arms -- the only ones in which a managed file really
#      does change -- then reported "changed SILENTLY", which is the FAIL path this
#      suite exists to detect; a silenced installAssets would look exactly the
#      same from here. Without that mutation the audit's changed-and-unannounced
#      branch is unreachable and the whole file is decoration.
#   3. Half (b) needed its own control -- non-vacuity is per case, and controls
#      1 and 2 both exercise half (a). audit_file's unchanged-branch grep was
#      pointed at a token the binary DOES print on every run; every
#      "unchanged, and not claimed otherwise" assertion then present (seventeen of
#      them, before the rollback arm) flipped to FAIL, across every state and
#      every command. Both halves are wired to a reachable failure, not just the
#      one the trap lives in.
#   4. The coverage pin got one too: commenting out a single audit_step call
#      dropped the run to 40 assertions, which `[ "$FAIL" -eq 0 ]` reports as a
#      clean pass. With the pin, that run exits 1 and names the shortfall. An
#      all-clear indistinguishable from "I never ran" is not a check.
#   5. The rollback arm is itself the answer to a control the suite FAILED. A
#      red-team mutant forcing `downgrade := false` (init.go:267) -- which
#      makes erg narrate a revert as an ordinary refresh -- passed the first
#      three arms 42/0, because no fixture stamped newer than the binary and
#      the direction logic 0279 exists for was never reached. Re-run against
#      the arm below, the same mutant dies on
#      "rollback/migrate: the revert is narrated as a downgrade".
#
# Note for anyone extending the announcement grep: `erg migrate` ALWAYS prints
# the summary line "migrate: AGENTS.md refreshed (N created, N refreshed, N
# unchanged)", even when the counts say nothing was refreshed. Greping for the
# word "refreshed" in migrate's output therefore matches unconditionally and
# proves nothing. The audit reads the per-file line that installAssets emits --
# "init: refreshed tickets/AGENTS.md (git restore -- ... to undo)" -- which is
# printed only on a real write.
set -eu

ERG="${ERG_BIN:-build/erg}"
ERG_ABS=$(readlink -f "$ERG")
# Repo root (this script lives in tests/), for the embedded asset sources.
ROOT=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
EMBEDDED_AGENTS="$ROOT/src/go/assets/AGENTS.md"

PASS=0
FAIL=0
pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

# Coverage pin. `[ "$FAIL" -eq 0 ]` alone cannot see an assertion that stopped
# running: comment out one audit_step call and this file exits 0 with two fewer
# audits and no other signal. Ticket 0278's log cites the count as evidence, so
# the count is asserted. Bump it deliberately when adding an arm -- a surprise
# here means coverage moved without anyone deciding it should.
EXPECTED_ASSERTIONS=57

# Local-path git remotes drive the `erg update` arms (update fetches the
# committed binary via git, never HTTP). Hardened hosts set
# protocol.file.allow=never globally; inject the override via env so it reaches
# both our git calls and the child git that erg itself spawns. The override is
# unconditional, not additive -- it would clobber an outer GIT_CONFIG_COUNT
# scheme (an insteadOf proxy rule, say) for this process tree. That is the form
# test_update.sh established and CI sets no such vars; breaking ranks in one
# suite would be the surprising move.
export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0=protocol.file.allow
export GIT_CONFIG_VALUE_0=always

# Scratch space. mktemp honours TMPDIR, so point TMPDIR at a real filesystem if
# /tmp is small or full: the git clones below are real binaries, and an ENOSPC
# here aborts with a git error that reads like an invariant failure until you
# look. The trap fires on that abort too.
WORKROOT=$(mktemp -d)
trap 'rm -rf "$WORKROOT"' EXIT

echo "=== asset invariant across init/update/migrate (ticket 0278) ==="

ERGRC_MARK="# LOCAL EDIT 0278 -- must survive every path"
AGENTS_MARK="<!-- LOCAL EDIT 0278 -- migrate is allowed to take this, loudly -->"
# A stamp date this binary cannot predate away from. Must match buildDateLayout
# byte for byte (manifest.go): same width, same separators, Z, and a possible
# calendar instant -- anything else is "not comparable" and isRollback returns
# false, silently turning the rollback arm into an ordinary local-edit arm.
FUTURE_STAMP="2099-01-01T00:00:00Z"

sha_of() {
    if [ -f "$1" ]; then sha256sum "$1" | cut -d' ' -f1; else echo "ABSENT"; fi
}

git_init() {
    git init -q "$1"
    git -C "$1" config user.email test@example.com
    git -C "$1" config user.name test
    git -C "$1" config commit.gpgsign false
}

# --- the shared "origin" remote -------------------------------------------
# Its committed tickets/erg differs from $ERG by one trailing byte, so a clone
# whose binary is $ERG has a genuine swap to perform and `erg update` reaches
# its post-swap asset-condition report. The remote deliberately ships NO
# managed asset: every fixture state is built in the clone, by us.
REMOTE="$WORKROOT/remote"
git_init "$REMOTE"
mkdir "$REMOTE/tickets"
cp "$ERG_ABS" "$REMOTE/tickets/erg"
printf 'X' >> "$REMOTE/tickets/erg"
cat > "$REMOTE/tickets/0001-subject.erg" <<'ERGEOF'
%erg 0.1
Title: A ticket so the store is a real corpus
Created: 2026-01-01
Author: a

--- log ---
2026-01-01T10:00Z a created

--- body ---
ERGEOF
git -C "$REMOTE" add -A
git -C "$REMOTE" commit -qm init

# --- fixture builder -------------------------------------------------------
# make_store DIR STATE -- a clone of REMOTE whose tickets/ is in STATE.
make_store() {
    _dir=$1
    _state=$2
    git clone -q "$REMOTE" "$_dir"
    cp "$ERG_ABS" "$_dir/tickets/erg"
    # init lays down both managed assets at the embedded bytes and stamps them.
    "$ERG_ABS" init "$_dir" >/dev/null 2>&1 || true
    case "$_state" in
    clean) ;;
    edited)
        printf '\n%s\n' "$ERGRC_MARK" >> "$_dir/tickets/.ergrc"
        printf '\n%s\n' "$AGENTS_MARK" >> "$_dir/tickets/AGENTS.md"
        ;;
    stampless)
        rm -f "$_dir/tickets/.erg-assets"
        printf '\n%s\n' "$ERGRC_MARK" >> "$_dir/tickets/.ergrc"
        ;;
    rollback)
        # Divergent bytes on disk, and a stamp that RECORDS those bytes and
        # claims a build date this binary predates. That combination is the
        # only one erg reads as "the deployed assets are ahead of me": the
        # stamp branch of isCleanUpgrade needs disk == stamp before isRollback
        # is even consulted. The date must have buildDateLayout's exact shape
        # or looksLikeBuildDate rejects it and the whole arm silently degrades
        # to the ordinary local-edit path -- which is what assert_state below
        # is for.
        printf '\n%s\n' "$ERGRC_MARK" >> "$_dir/tickets/.ergrc"
        printf '\n%s\n' "$AGENTS_MARK" >> "$_dir/tickets/AGENTS.md"
        printf '# erg provenance manifest -- do not edit\nrev: future\ndate: %s\nassets:\n  .ergrc sha256:%s\n  AGENTS.md sha256:%s\n' \
            "$FUTURE_STAMP" \
            "$(sha_of "$_dir/tickets/.ergrc")" \
            "$(sha_of "$_dir/tickets/AGENTS.md")" \
            > "$_dir/tickets/.erg-assets"
        ;;
    *)
        fail "make_store: unknown state $_state"
        return 1
        ;;
    esac
}

# Non-vacuity guard: a fixture that is not in the state it claims makes every
# assertion below pass for the wrong reason. Asserted on the fixture itself,
# before any command under test runs.
assert_state() {
    _dir=$1
    _state=$2
    case "$_state" in
    clean)
        if [ -f "$_dir/tickets/.erg-assets" ] &&
            cmp -s "$_dir/tickets/AGENTS.md" "$EMBEDDED_AGENTS"; then
            pass "fixture clean: stamped, AGENTS.md byte-identical to embedded"
        else
            fail "fixture clean: expected a stamp and a pristine AGENTS.md"
        fi
        ;;
    edited)
        if [ -f "$_dir/tickets/.erg-assets" ] &&
            grep -qF "$ERGRC_MARK" "$_dir/tickets/.ergrc" &&
            grep -qF "$AGENTS_MARK" "$_dir/tickets/AGENTS.md"; then
            pass "fixture edited: stamped, both assets carry a local edit"
        else
            fail "fixture edited: expected a stamp and both edits in place"
        fi
        ;;
    stampless)
        if [ ! -f "$_dir/tickets/.erg-assets" ] &&
            grep -qF "$ERGRC_MARK" "$_dir/tickets/.ergrc" &&
            cmp -s "$_dir/tickets/AGENTS.md" "$EMBEDDED_AGENTS"; then
            pass "fixture stampless: no manifest, .ergrc diverged, AGENTS.md pristine"
        else
            fail "fixture stampless: expected no manifest and a diverged .ergrc only"
        fi
        ;;
    rollback)
        # The stamp must record the bytes ON DISK -- if it did not, isCleanUpgrade
        # would take its disk != stamp exit and the arm would exercise the
        # local-edit path under a rollback label, proving nothing about direction.
        if grep -q "^date: $FUTURE_STAMP\$" "$_dir/tickets/.erg-assets" &&
            grep -q "AGENTS.md sha256:$(sha_of "$_dir/tickets/AGENTS.md")\$" "$_dir/tickets/.erg-assets" &&
            ! cmp -s "$_dir/tickets/AGENTS.md" "$EMBEDDED_AGENTS"; then
            pass "fixture rollback: stamp records the on-disk bytes and postdates this binary"
        else
            fail "fixture rollback: expected a future-dated stamp matching the on-disk hashes"
        fi
        ;;
    esac
}

# --- the auditor -----------------------------------------------------------
# AUDIT_OUT carries the last runner's combined output; audit_file reads it.
AUDIT_OUT=""

# runners: each takes the store dir and runs one command under test.
run_init() { "$ERG_ABS" init "$1" 2>&1 || true; }
run_migrate() { "$ERG_ABS" migrate "$1/tickets" 2>&1 || true; }
run_update() {
    # update must run from inside the clone, as the checked-out binary, so the
    # post-swap re-exec is the NEW binary reading its own embedded assets.
    (cd "$1" && ERG_TICKET_DIR="$1/tickets" ./tickets/erg update 2>&1 || true)
}

# audit_file LABEL NAME BEFORE AFTER -- the invariant, both halves.
audit_file() {
    _label=$1
    _name=$2
    _before=$3
    _after=$4
    if [ "$_before" = "$_after" ]; then
        if echo "$AUDIT_OUT" | grep -qE "^init: (refreshed|downgraded) tickets/$_name"; then
            fail "$_label: $_name is unchanged on disk yet narrated as refreshed/downgraded"
        else
            pass "$_label: $_name unchanged, and not claimed otherwise"
        fi
    else
        if echo "$AUDIT_OUT" | grep -qE "^init: (refreshed|downgraded) tickets/$_name \(git restore"; then
            pass "$_label: $_name changed AND was announced with an undo hint"
        else
            fail "$_label: $_name changed SILENTLY -- invariant violated"
            echo "    ---- output was ----"
            echo "$AUDIT_OUT" | sed 's/^/    /'
        fi
    fi
}

# audit_step STORE LABEL RUNNER -- run one command, audit both managed assets.
audit_step() {
    _store=$1
    _label=$2
    _runner=$3
    _b_rc=$(sha_of "$_store/tickets/.ergrc")
    _b_ag=$(sha_of "$_store/tickets/AGENTS.md")
    AUDIT_OUT=$("$_runner" "$_store")
    audit_file "$_label" ".ergrc" "$_b_rc" "$(sha_of "$_store/tickets/.ergrc")"
    audit_file "$_label" "AGENTS.md" "$_b_ag" "$(sha_of "$_store/tickets/AGENTS.md")"
}

# ===========================================================================
# Arm 1 -- clean: silent no-op on every path.
# ===========================================================================
S="$WORKROOT/clean"
make_store "$S" clean
assert_state "$S" clean

SUM_RC=$(sha_of "$S/tickets/.ergrc")
SUM_AG=$(sha_of "$S/tickets/AGENTS.md")

audit_step "$S" "clean/init" run_init
if echo "$AUDIT_OUT" | grep -q "0 created, 0 refreshed, 0 skipped (preserved), 2 unchanged"; then
    pass "clean/init: reports two unchanged assets, nothing created or refreshed"
else
    fail "clean/init: expected an all-unchanged summary (got: $AUDIT_OUT)"
fi

# `erg check` is the reporting channel: a clean store must raise neither the
# drift WARN nor the stampless NOTE. Content assertion, not exit code -- the
# defect class this suite guards is silence, and silence has no exit code.
CHK=$("$ERG_ABS" check "$S/tickets" 2>&1 || true)
if echo "$CHK" | grep -qE "WARN .*(\.ergrc|AGENTS\.md)|NOTE .*(\.ergrc|AGENTS\.md)"; then
    fail "clean/check: a pristine stamped store raised an asset warning (got: $CHK)"
else
    pass "clean/check: pristine stamped store says nothing about its assets"
fi

audit_step "$S" "clean/update" run_update
if echo "$AUDIT_OUT" | grep -q "erg: updated"; then
    pass "clean/update: the binary swap really happened (arm is not vacuous)"
else
    fail "clean/update: no swap, so the post-swap asset report never ran (got: $AUDIT_OUT)"
fi
if echo "$AUDIT_OUT" | grep -qE "run 'erg init' to refresh|carry no \.erg-assets stamp"; then
    fail "clean/update: clean store nagged about its assets (got: $AUDIT_OUT)"
else
    pass "clean/update: clean store gets no asset nag after the swap"
fi

audit_step "$S" "clean/migrate" run_migrate
if echo "$AUDIT_OUT" | grep -q "migrate: AGENTS.md refreshed (0 created, 0 refreshed, 1 unchanged)"; then
    pass "clean/migrate: counts report one unchanged asset, none refreshed"
else
    fail "clean/migrate: expected an all-unchanged asset count (got: $AUDIT_OUT)"
fi

if [ "$(sha_of "$S/tickets/.ergrc")" = "$SUM_RC" ] &&
    [ "$(sha_of "$S/tickets/AGENTS.md")" = "$SUM_AG" ]; then
    pass "clean: both assets survive init+update+migrate byte-identical"
else
    fail "clean: an asset changed across the three-command sequence"
fi

# ===========================================================================
# Arm 2 -- edited: the edit is preserved and the preservation is announced,
# except where the charter says migrate takes AGENTS.md -- loudly.
# ===========================================================================
S="$WORKROOT/edited"
make_store "$S" edited
assert_state "$S" edited

audit_step "$S" "edited/init" run_init
if echo "$AUDIT_OUT" | grep -q "tickets/.ergrc has local edits -- preserving" &&
    echo "$AUDIT_OUT" | grep -q "tickets/AGENTS.md has local edits -- preserving"; then
    pass "edited/init: both edits preserved, and init says so for each file"
else
    fail "edited/init: expected a per-file preservation message (got: $AUDIT_OUT)"
fi
"$ERG_ABS" init "$S" >/dev/null 2>&1 && INIT_RC=0 || INIT_RC=$?
if [ "$INIT_RC" -eq 2 ]; then
    pass "edited/init: exits 2 (preserved), so a script cannot mistake it for a no-op"
else
    fail "edited/init: expected exit 2 on preservation, got $INIT_RC"
fi

CHK=$("$ERG_ABS" check "$S/tickets" 2>&1 || true)
if echo "$CHK" | grep -qE "WARN|NOTE"; then
    fail "edited/check: a deliberate local edit must not be nagged about (got: $CHK)"
else
    pass "edited/check: a local edit under a current stamp raises nothing"
fi

audit_step "$S" "edited/update" run_update
if echo "$AUDIT_OUT" | grep -q "erg: updated"; then
    pass "edited/update: the binary swap really happened (arm is not vacuous)"
else
    fail "edited/update: no swap, so the post-swap asset report never ran"
fi
if grep -qF "$ERGRC_MARK" "$S/tickets/.ergrc" &&
    grep -qF "$AGENTS_MARK" "$S/tickets/AGENTS.md"; then
    pass "edited/update: update replaces the binary only, never a store file"
else
    fail "edited/update: an asset edit did not survive the binary swap"
fi

# The charter arm. migrate MUST overwrite the diverged AGENTS.md (ticket 0224:
# agent operating instructions track the binary) and MUST NOT touch .ergrc
# (configuration delivery is init's job). Asserting the opposite for AGENTS.md
# would reverse a closed author decision and break two committed tests.
audit_step "$S" "edited/migrate" run_migrate
if cmp -s "$S/tickets/AGENTS.md" "$EMBEDDED_AGENTS"; then
    pass "edited/migrate: AGENTS.md force-overwritten to embedded, as the charter wants (0224)"
else
    fail "edited/migrate: AGENTS.md was NOT refreshed -- charter decision 0224 regressed"
fi
if echo "$AUDIT_OUT" | grep -q "init: refreshed tickets/AGENTS.md (git restore -- tickets/AGENTS.md to undo)"; then
    pass "edited/migrate: the overwrite names the file and the command that undoes it"
else
    fail "edited/migrate: the overwrite was not announced with an undo hint (got: $AUDIT_OUT)"
fi
if grep -qF "$ERGRC_MARK" "$S/tickets/.ergrc"; then
    pass "edited/migrate: .ergrc is outside migrateAssetPaths and keeps its edit"
else
    fail "edited/migrate: migrate destroyed a .ergrc local edit"
fi

# ===========================================================================
# Arm 3 -- stampless: unattributable divergence reports itself (ticket 0283).
# ===========================================================================
S="$WORKROOT/stampless"
make_store "$S" stampless
assert_state "$S" stampless

STAMPLESS_NOTE="no .erg-assets stamp -- cannot tell whether this is a clean upgrade or a local edit"

CHK=$("$ERG_ABS" check "$S/tickets" 2>&1 || true)
if echo "$CHK" | grep -qF "NOTE .ergrc: $STAMPLESS_NOTE"; then
    pass "stampless/check: the diverged asset is named, with no direction claimed"
else
    fail "stampless/check: expected the 0283 stampless NOTE (got: $CHK)"
fi
if echo "$CHK" | grep -q "AGENTS.md"; then
    fail "stampless/check: reported a pristine asset -- the gate is divergence, not absence of a stamp"
else
    pass "stampless/check: the pristine asset stays silent"
fi
if echo "$CHK" | grep -q "run 'erg init' to refresh"; then
    fail "stampless/check: made a stamp-relative claim with no stamp to stand on"
else
    pass "stampless/check: no stamp, so no stamp-relative claim"
fi

# update reaches the stampless branch only by re-execing the swapped-in binary;
# this is the sole caller that exercises that path end to end.
audit_step "$S" "stampless/update" run_update
if echo "$AUDIT_OUT" | grep -q "erg: updated" &&
    echo "$AUDIT_OUT" | grep -qF "carry no .erg-assets stamp"; then
    pass "stampless/update: the post-swap report surfaces the unstamped divergence"
else
    fail "stampless/update: expected the stampless hint after the swap (got: $AUDIT_OUT)"
fi

audit_step "$S" "stampless/init" run_init
if echo "$AUDIT_OUT" | grep -q "tickets/.ergrc has local edits -- preserving"; then
    pass "stampless/init: an unattributable divergence is preserved, not clobbered"
else
    fail "stampless/init: expected preservation of the unstamped divergence (got: $AUDIT_OUT)"
fi
if grep -qF "$ERGRC_MARK" "$S/tickets/.ergrc"; then
    pass "stampless/init: the diverged bytes are still on disk afterwards"
else
    fail "stampless/init: the diverged .ergrc was destroyed"
fi
# Deliberately NOT asserted here: that init leaves the store unstamped. It does
# not -- installAssets writes the manifest at the end of the run, stamping the
# preserved edit with the EMBEDDED hash, which silences the NOTE from the next
# run on with the divergence still on disk. That is a known wart, tracked by
# ticket 0292 and documented on assetStamplessSignal; pinning today's behaviour
# with an assertion here would turn 0292's fix into a test failure.

audit_step "$S" "stampless/migrate" run_migrate
if grep -qF "$ERGRC_MARK" "$S/tickets/.ergrc"; then
    pass "stampless/migrate: .ergrc keeps its divergence through the layout sweep"
else
    fail "stampless/migrate: migrate destroyed the diverged .ergrc"
fi

# ===========================================================================
# Arm 4 -- rollback: this binary predates the stamp, so an overwrite is a
# revert. Ticket 0279's defect class; the only path in the set demonstrated to
# lose data.
# ===========================================================================
S="$WORKROOT/rollback"
make_store "$S" rollback
assert_state "$S" rollback

CHK=$("$ERG_ABS" check "$S/tickets" 2>&1 || true)
if echo "$CHK" | grep -q "WARN AGENTS.md: embedded version is older than the .erg-assets stamp"; then
    pass "rollback/check: names the direction -- the binary is behind, not the store"
else
    fail "rollback/check: expected the rollback WARN (got: $CHK)"
fi
if echo "$CHK" | grep -q "run 'erg init' to refresh"; then
    fail "rollback/check: advised an init that would revert the store (got: $CHK)"
else
    pass "rollback/check: does not advise the command that would revert the store"
fi

audit_step "$S" "rollback/init" run_init
if echo "$AUDIT_OUT" | grep -q "tickets/AGENTS.md is newer than this binary -- preserving"; then
    pass "rollback/init: preserved, and named as a rollback rather than as local edits"
else
    fail "rollback/init: expected the rollback preservation message (got: $AUDIT_OUT)"
fi
if echo "$AUDIT_OUT" | grep -q "has local edits"; then
    fail "rollback/init: called a revert a local edit -- wrong reason, wrong remedy"
else
    pass "rollback/init: does not misreport the reason it preserved"
fi
# 0279's other half: the run declined to touch the assets, so it must not stamp
# them either. Rewriting the manifest here would destroy the very evidence that
# established the direction, and the next run would read the same store as an
# ordinary local edit.
if grep -q "^date: $FUTURE_STAMP\$" "$S/tickets/.erg-assets"; then
    pass "rollback/init: the stamp survives, so the direction is still established next run"
else
    fail "rollback/init: init overwrote the provenance that proved the rollback"
fi

audit_step "$S" "rollback/update" run_update
if echo "$AUDIT_OUT" | grep -q "erg: updated"; then
    pass "rollback/update: the binary swap really happened (arm is not vacuous)"
else
    fail "rollback/update: no swap, so nothing about the asset state was exercised"
fi
# Not asserted: whether update relays the rollback condition. It does not, and
# that is deliberate -- assetRollbackSignal has no cross-version consumer because
# update only ever re-execs a strictly NEWER binary (manifest.go). This fixture
# re-execs the same build, a state update cannot reach in the field, so an
# assertion either way would pin an artifact of the fixture.

# migrate takes AGENTS.md here too, and this is where the word matters: it is a
# revert, not a refresh, and installAssets must say "downgraded". A mutant
# forcing `downgrade := false` (init.go:267) passes every other arm in this file.
audit_step "$S" "rollback/migrate" run_migrate
if echo "$AUDIT_OUT" | grep -q "init: downgraded tickets/AGENTS.md (git restore -- tickets/AGENTS.md to undo)"; then
    pass "rollback/migrate: the revert is narrated as a downgrade, not as a refresh"
else
    fail "rollback/migrate: a revert was reported as an ordinary refresh (got: $AUDIT_OUT)"
fi
if grep -qF "$ERGRC_MARK" "$S/tickets/.ergrc"; then
    pass "rollback/migrate: .ergrc, outside migrateAssetPaths, is not reverted"
else
    fail "rollback/migrate: migrate reverted an asset it does not manage"
fi
# Observed here and NOT asserted, because it is a defect rather than a contract:
# this migrate run rewrites .erg-assets with the current binary's rev/date and
# the EMBEDDED hashes for BOTH assets -- including .ergrc, which it never
# touched and which still holds the divergence. The rollback WARN goes silent
# from the next run on. init declines exactly this rewrite (0279's exemption
# above), but the exemption is set only on the preserve leg, which migrate's
# refuseDiverged=false call never takes. Filed as ticket 0296; asserting
# today's behaviour would turn its fix into a test failure.

echo ""
echo "=== $PASS passed, $FAIL failed ==="
RAN=$((PASS + FAIL))
if [ "$RAN" -ne "$EXPECTED_ASSERTIONS" ]; then
    echo "  FAIL: coverage pin: ran $RAN assertions, expected $EXPECTED_ASSERTIONS" >&2
    exit 1
fi
[ "$FAIL" -eq 0 ] || exit 1
