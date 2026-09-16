#!/bin/sh
# Integration tests for: erg init
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg init ==="

TDIR=$(mktemp -d)
trap 'rm -rf "$TDIR"' EXIT

REPO="$TDIR/repo"
mkdir -p "$REPO/tickets"

# --- init without binary exits 1 ---

if OUT=$($ERG init "$REPO" 2>&1); then
    fail "init without binary should exit 1"
else
    if echo "$OUT" | grep -q "binary not found"; then
        pass "init without binary: exits 1 with 'binary not found'"
    else
        fail "init without binary: expected 'binary not found' in output (got: $OUT)"
    fi
fi

# --- place a fake binary ---

touch "$REPO/tickets/erg"

# --- init unpacks exactly 2 files ---

OUT=$($ERG init "$REPO" 2>&1)

if [ -f "$REPO/tickets/AGENTS.md" ]; then
    pass "init creates AGENTS.md"
else
    fail "init creates AGENTS.md"
fi

if [ -f "$REPO/tickets/.ergrc" ]; then
    pass "init creates .ergrc"
else
    fail "init creates .ergrc"
fi

if [ -f "$REPO/tickets/spec-erg-v1.md" ]; then
    fail "init must not deposit spec-erg-v1.md (now: erg spec)"
else
    pass "init does not deposit spec-erg-v1.md"
fi

if [ -f "$REPO/tickets/integration.md" ]; then
    fail "init must not deposit integration.md (now: erg integration)"
else
    pass "init does not deposit integration.md"
fi

# --- no integration/ directory created ---

if [ -d "$REPO/tickets/integration" ]; then
    fail "init must not create tickets/integration/ directory"
else
    pass "init does not create tickets/integration/ directory"
fi

# --- no JSON bootstrap manifest (rejected design), no root AGENTS.md, no .gitignore ---
# Note: init DOES write tickets/.erg-assets (ticket 0210); that is tested below.
# This guards only against the old rejected .erg-bootstrap-manifest.json shape.

if [ -f "$REPO/tickets/.erg-bootstrap-manifest.json" ]; then
    fail "init must not write the rejected JSON bootstrap manifest"
else
    pass "init does not write the rejected JSON bootstrap manifest"
fi

if [ -f "$REPO/AGENTS.md" ]; then
    fail "init must not touch AGENTS.md"
else
    pass "init does not touch AGENTS.md"
fi

if [ -f "$REPO/.gitignore" ]; then
    fail "init must not touch .gitignore"
else
    pass "init does not touch .gitignore"
fi

# --- re-init is idempotent ---

OUT2=$($ERG init "$REPO" 2>&1)

if echo "$OUT2" | grep -q "0 created, 0 refreshed, 0 skipped (preserved), 2 unchanged"; then
    pass "re-init is idempotent (2 unchanged)"
else
    fail "re-init is idempotent (expected '0 created, 0 refreshed, 0 skipped (preserved), 2 unchanged', got: $OUT2)"
fi

# --- re-init refuses to overwrite user-edited files ---
printf "# user edit\n" >> "$REPO/tickets/.ergrc"
OUT3=$($ERG init "$REPO" 2>&1) && RC3=0 || RC3=$?
if [ "$RC3" -ne 0 ]; then
    pass "re-init with local edits: exits non-zero"
else
    fail "re-init with local edits: exits non-zero (rc=$RC3)"
fi
if [ "$RC3" -eq 2 ]; then
    pass "re-init with local edits: exit code is 2 (skipped, not hard error)"
else
    fail "re-init with local edits: expected exit 2, got $RC3"
fi
if grep -q "# user edit" "$REPO/tickets/.ergrc"; then
    pass "re-init with local edits: modified file preserved"
else
    fail "re-init with local edits: modified file was overwritten"
fi
if echo "$OUT3" | grep -q "local edits"; then
    pass "re-init with local edits: mentions 'local edits' in output"
else
    fail "re-init with local edits: expected 'local edits' in output (got: $OUT3)"
fi

# --- output mentions erg install ---

if echo "$OUT" | grep -q "erg install"; then
    pass "init output mentions erg install"
else
    fail "init output mentions erg install (got: $OUT)"
fi

# --- uninstall subcommand is removed ---

if $ERG uninstall "$REPO" >/dev/null 2>&1; then
    fail "uninstall subcommand should not exist"
else
    pass "uninstall subcommand removed"
fi

# --- every unpacked store asset is pure ASCII (no U+FFFD, no stray Unicode) ---
# The original bug (0160) was a U+FFFD replacement character introduced by a
# Unicode round-trip. Asserting pure ASCII is strictly stronger than checking
# for U+FFFD alone and forecloses the whole corruption class.
# Cover BOTH assets erg init unpacks (see initAssetPaths in src/go/init.go), not
# just AGENTS.md: a non-ASCII byte in .ergrc was previously unguarded (0245).

for asset in tickets/.ergrc tickets/AGENTS.md; do
    if LC_ALL=C grep -nq '[^[:print:][:space:]]' "$REPO/$asset"; then
        fail "init-unpacked $asset contains non-ASCII or non-printable bytes"
    else
        pass "init-unpacked $asset is pure ASCII"
    fi
done

# unknown flag rejection (ticket 0178)
    out=$($ERG init --bogus 2>&1) && rc=0 || rc=$?
    if [ "$rc" -ne 0 ] && echo "$out" | grep -q "unknown flag"; then
        pass "unknown flag rejected with usage message"
    else
        fail "unknown flag not rejected (rc=$rc, got: $out)"
    fi

# --- orphan cleanup: matching files are removed ---

ORPHAN="$TDIR/orphan"
mkdir -p "$ORPHAN/tickets"
touch "$ORPHAN/tickets/erg"
# Pre-place spec-erg-v1.md with the exact embedded content
$ERG spec > "$ORPHAN/tickets/spec-erg-v1.md" 2>/dev/null
$ERG integration > "$ORPHAN/tickets/integration.md" 2>/dev/null
OUT_ORPHAN=$($ERG init "$ORPHAN" 2>&1)
if [ ! -f "$ORPHAN/tickets/spec-erg-v1.md" ]; then
    pass "orphan cleanup: matching spec-erg-v1.md removed"
else
    fail "orphan cleanup: matching spec-erg-v1.md still present"
fi
if [ ! -f "$ORPHAN/tickets/integration.md" ]; then
    pass "orphan cleanup: matching integration.md removed"
else
    fail "orphan cleanup: matching integration.md still present"
fi
if echo "$OUT_ORPHAN" | grep -q "removed orphaned asset"; then
    pass "orphan cleanup: logged removal message"
else
    fail "orphan cleanup: no removal message (got: $OUT_ORPHAN)"
fi

# --- orphan cleanup: divergent files are preserved ---

DIVERGE="$TDIR/diverge"
mkdir -p "$DIVERGE/tickets"
touch "$DIVERGE/tickets/erg"
printf "# my custom spec\n" > "$DIVERGE/tickets/spec-erg-v1.md"
printf "# my custom integration\n" > "$DIVERGE/tickets/integration.md"
$ERG init "$DIVERGE" > /dev/null 2>&1
if [ -f "$DIVERGE/tickets/spec-erg-v1.md" ]; then
    pass "orphan cleanup: divergent spec-erg-v1.md preserved"
else
    fail "orphan cleanup: divergent spec-erg-v1.md was removed"
fi
if [ -f "$DIVERGE/tickets/integration.md" ]; then
    pass "orphan cleanup: divergent integration.md preserved"
else
    fail "orphan cleanup: divergent integration.md was removed"
fi

# --- dry-run: no side effects (ticket 0207) ---

DRY="$TDIR/dry"
mkdir -p "$DRY/tickets"
touch "$DRY/tickets/erg"
# Pre-place a matching orphan that a real init would remove.
$ERG spec > "$DRY/tickets/spec-erg-v1.md" 2>/dev/null
OUT_DRY=$($ERG init -n "$DRY" 2>&1) && RC_DRY=0 || RC_DRY=$?
if [ "$RC_DRY" -eq 0 ]; then
    pass "dry-run: exits 0 on a clean preview"
else
    fail "dry-run: expected exit 0, got $RC_DRY"
fi
if [ ! -f "$DRY/tickets/.ergrc" ] && [ ! -f "$DRY/tickets/AGENTS.md" ]; then
    pass "dry-run: did not create any asset"
else
    fail "dry-run: created assets (.ergrc or AGENTS.md present)"
fi
if [ -f "$DRY/tickets/spec-erg-v1.md" ]; then
    pass "dry-run: did not remove the matching orphan"
else
    fail "dry-run: removed the orphan (should only preview)"
fi
if echo "$OUT_DRY" | grep -q "dry-run"; then
    pass "dry-run: output labels itself as dry-run"
else
    fail "dry-run: output missing 'dry-run' label (got: $OUT_DRY)"
fi
# --dry-run long form is also accepted
$ERG init --dry-run "$DRY" >/dev/null 2>&1 && pass "long --dry-run accepted" || fail "long --dry-run rejected"

# --- dry-run reports exit 2 when it would skip a local edit ---

DRY2="$TDIR/dry2"
mkdir -p "$DRY2/tickets"
touch "$DRY2/tickets/erg"
$ERG init "$DRY2" >/dev/null 2>&1
printf "# edit\n" >> "$DRY2/tickets/.ergrc"
$ERG init -n "$DRY2" >/dev/null 2>&1 && RC_DRY2=0 || RC_DRY2=$?
if [ "$RC_DRY2" -eq 2 ]; then
    pass "dry-run: exit 2 when a local edit would be skipped"
else
    fail "dry-run: expected exit 2 for would-skip, got $RC_DRY2"
fi
# The local edit must still be present (dry-run never writes).
if grep -q "# edit" "$DRY2/tickets/.ergrc"; then
    pass "dry-run: local edit untouched"
else
    fail "dry-run: local edit was modified"
fi

# --- --force overwrites divergent files (exit 0) ---

FORCE="$TDIR/force"
mkdir -p "$FORCE/tickets"
touch "$FORCE/tickets/erg"
$ERG init "$FORCE" >/dev/null 2>&1
printf "# user edit to clobber\n" >> "$FORCE/tickets/.ergrc"
$ERG init --force "$FORCE" >/dev/null 2>&1 && RC_FORCE=0 || RC_FORCE=$?
if [ "$RC_FORCE" -eq 0 ]; then
    pass "--force: exits 0"
else
    fail "--force: expected exit 0, got $RC_FORCE"
fi
if grep -q "# user edit to clobber" "$FORCE/tickets/.ergrc"; then
    fail "--force: local edit survived (should be overwritten)"
else
    pass "--force: local edit overwritten"
fi

# --- chained read-only check surfaces a corpus warning, init still exits 0 ---

CHAIN="$TDIR/chain"
mkdir -p "$CHAIN/tickets"
touch "$CHAIN/tickets/erg"
# A closed ticket placed outside closed/ triggers folderClosure WARN.
cat > "$CHAIN/tickets/9001-chain.erg" <<'ERGEOF'
%erg 0.1
Title: Chain test closed ticket
Created: 2026-06-02
Author: claude
Closed: 2026-06-02

--- log ---
2026-06-02T00:00Z claude created

--- body ---
Closed ticket outside closed/ to trigger a folderClosure warning.
ERGEOF
OUT_CHAIN=$($ERG init "$CHAIN" 2>&1) && RC_CHAIN=0 || RC_CHAIN=$?
if echo "$OUT_CHAIN" | grep -q "closed ticket not in closed/ directory"; then
    pass "chaining: init surfaces the corpus warning"
else
    fail "chaining: warning not surfaced (got: $OUT_CHAIN)"
fi
if [ "$RC_CHAIN" -eq 0 ]; then
    pass "chaining: init exit code reflects init (0), not the warning"
else
    fail "chaining: init exit should be 0 despite warning, got $RC_CHAIN"
fi

# --- provenance manifest .erg-assets (ticket 0210) ---
MAN="$TDIR/manifest"
mkdir -p "$MAN/tickets"
touch "$MAN/tickets/erg"
$ERG init "$MAN" >/dev/null 2>&1
MFILE="$MAN/tickets/.erg-assets"
if [ -f "$MFILE" ]; then
    pass "manifest: init writes tickets/.erg-assets"
else
    fail "manifest: .erg-assets not written"
fi
if head -1 "$MFILE" | grep -q "erg provenance manifest"; then
    pass "manifest: has the provenance header"
else
    fail "manifest: missing header (got: $(head -1 "$MFILE"))"
fi
if grep -q "^rev: " "$MFILE" && grep -q "^date: " "$MFILE"; then
    pass "manifest: records rev and date"
else
    fail "manifest: missing rev/date"
fi
if grep -q "AGENTS.md sha256:[0-9a-f]" "$MFILE" && grep -q ".ergrc sha256:[0-9a-f]" "$MFILE"; then
    pass "manifest: records sha256 for each asset"
else
    fail "manifest: missing per-asset sha256"
fi
# .ergrc sorts before AGENTS.md (deterministic order)
if [ "$(grep -n 'sha256:' "$MFILE" | head -1 | grep -c '.ergrc')" -eq 1 ]; then
    pass "manifest: assets are in deterministic (.ergrc-first) order"
else
    fail "manifest: asset order is not deterministic"
fi
# idempotence: same binary + same assets => byte-identical manifest
m1=$(cat "$MFILE")
$ERG init "$MAN" >/dev/null 2>&1
m2=$(cat "$MFILE")
if [ "$m1" = "$m2" ]; then
    pass "manifest: re-init is byte-identical (deterministic)"
else
    fail "manifest: re-init changed the manifest"
fi
# check ignores the manifest (it is not a .erg file -> never trips the hook)
$ERG check "$MAN/tickets" >/dev/null 2>&1 && crc=0 || crc=$?
if [ "$crc" -eq 0 ]; then
    pass "manifest: erg check ignores .erg-assets (exit 0)"
else
    fail "manifest: erg check tripped on .erg-assets (rc=$crc)"
fi
# dry-run does NOT write the manifest
DRYM="$TDIR/drymanifest"
mkdir -p "$DRYM/tickets"; touch "$DRYM/tickets/erg"
$ERG init -n "$DRYM" >/dev/null 2>&1
if [ ! -f "$DRYM/tickets/.erg-assets" ]; then
    pass "manifest: dry-run writes no manifest"
else
    fail "manifest: dry-run wrote a manifest"
fi

# --- dpkg 3-state compare (ticket 0211) ---
# (Rows 1/5 and the Go unit test cover the rest; these drive the binary.)

# Row 2: clean upgrade -- on-disk == stamp but != embedded -> overwrite silently.
UP="$TDIR/dpkg-upgrade"
mkdir -p "$UP/tickets"; touch "$UP/tickets/erg"
printf 'OLD PRISTINE ERGRC\n' > "$UP/tickets/.ergrc"
oldhash=$(printf 'OLD PRISTINE ERGRC\n' | sha256sum | cut -d' ' -f1)
printf '# erg provenance manifest -- do not edit\nrev: x\ndate: y\nassets:\n  .ergrc sha256:%s\n  AGENTS.md sha256:deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef\n' "$oldhash" > "$UP/tickets/.erg-assets"
OUT_UP=$($ERG init "$UP" 2>&1) && rc=$? || rc=$?
if ! grep -q 'OLD PRISTINE ERGRC' "$UP/tickets/.ergrc"; then
    pass "dpkg row2: on-disk==stamp!=embedded is a clean upgrade (overwritten)"
else
    fail "dpkg row2: clean upgrade was not applied"
fi
if echo "$OUT_UP" | grep -q "git restore -- tickets/.ergrc"; then
    pass "dpkg: overwrite prints a git restore reversibility hint"
else
    fail "dpkg: missing git restore hint (got: $OUT_UP)"
fi

# Row 3: local edit -- on-disk != stamp and != embedded -> preserve, exit 2.
LE="$TDIR/dpkg-localedit"
mkdir -p "$LE/tickets"; touch "$LE/tickets/erg"
printf 'MY LOCAL EDIT\n' > "$LE/tickets/.ergrc"
printf '# erg provenance manifest -- do not edit\nrev: x\ndate: y\nassets:\n  .ergrc sha256:0000000000000000000000000000000000000000000000000000000000000000\n  AGENTS.md sha256:x\n' > "$LE/tickets/.erg-assets"
$ERG init "$LE" >/dev/null 2>&1 && lrc=0 || lrc=$?
if [ "$lrc" -eq 2 ] && grep -q 'MY LOCAL EDIT' "$LE/tickets/.ergrc"; then
    pass "dpkg row3: divergent-from-stamp is a local edit (preserved, exit 2)"
else
    fail "dpkg row3: local edit not preserved (rc=$lrc)"
fi

# Row 5: manifest ABSENT + unknown on-disk -> preserve (exit 2), never clobber.
AB="$TDIR/dpkg-absent"
mkdir -p "$AB/tickets"; touch "$AB/tickets/erg"
printf 'UNKNOWN NEVER-SHIPPED CONTENT\n' > "$AB/tickets/.ergrc"
$ERG init "$AB" >/dev/null 2>&1 && arc=0 || arc=$?
if [ "$arc" -eq 2 ] && grep -q 'UNKNOWN NEVER-SHIPPED' "$AB/tickets/.ergrc"; then
    pass "dpkg row5: no stamp + unknown hash is a local edit (preserved, exit 2)"
else
    fail "dpkg row5: unknown asset not preserved (rc=$arc)"
fi

# --force still bypasses the dpkg compare (overwrites a local edit).
$ERG init "$AB" --force >/dev/null 2>&1 && frc=0 || frc=$?
if [ "$frc" -eq 0 ] && ! grep -q 'UNKNOWN NEVER-SHIPPED' "$AB/tickets/.ergrc"; then
    pass "dpkg: --force overwrites a local edit (exempt from the compare)"
else
    fail "dpkg: --force did not overwrite (rc=$frc)"
fi

# migrate no longer touches .ergrc at all (ticket 0224): configuration delivery
# is erg init's job (the dpkg 3-state compare). A locally-edited .ergrc must
# survive erg migrate byte-identical -- migrate's asset refresh covers AGENTS.md.
MG="$TDIR/dpkg-migrate"
mkdir -p "$MG/tickets"; touch "$MG/tickets/erg"
printf 'EDITED ERGRC\n' > "$MG/tickets/.ergrc"
$ERG migrate "$MG/tickets" >/dev/null 2>&1
if grep -q 'EDITED ERGRC' "$MG/tickets/.ergrc"; then
    pass "dpkg: migrate leaves .ergrc untouched (config delivery is erg init's job)"
else
    fail "dpkg: migrate clobbered a locally-edited .ergrc (should leave it untouched)"
fi

# Loud output (criterion 5): an unchanged file names itself in NORMAL mode,
# not only under --dry-run. Re-init a freshly-initialized dir: both assets are
# byte-identical to embedded, so each must print an "unchanged" per-file line.
UC="$TDIR/dpkg-unchanged"
mkdir -p "$UC/tickets"; touch "$UC/tickets/erg"
$ERG init "$UC" >/dev/null 2>&1 || true
OUT_UC=$($ERG init "$UC" 2>&1 || true)
if echo "$OUT_UC" | grep -q "init: tickets/.ergrc unchanged"; then
    pass "dpkg: unchanged file names itself in normal mode (criterion 5)"
else
    fail "dpkg: unchanged file not named per-file in normal mode (got: $OUT_UC)"
fi

# --- channel 2 of ticket 0283's criterion 1: erg init reports a stampless
# --- store's condition (ticket 0292, defect 2)
#
# 0283 named three channels for an unstamped divergence -- erg check, erg
# init's chained post-init check, erg update's post-swap hint -- and the middle
# one could not fire for any asset warning. A stampless store with a diverged
# asset always skips, and `if skipped > 0 { return 2 }` sits several lines above
# the chained corpusWarnings block on both legs; on the non-skipping leg the
# manifest has already been written, so the store is no longer stampless by the
# time the check runs. The channel was closed by giving installAssets a third
# preserve reason carried on the per-file line, rather than by moving the
# chained check -- that keeps init's exit ordering and, in the same stroke,
# stops the run asserting an edit it never observed.
SL="$TDIR/stampless"
mkdir -p "$SL/tickets"
touch "$SL/tickets/erg"
printf '# an .ergrc that is not what this binary embeds\nlabels = whatever\n' > "$SL/tickets/.ergrc"
# Guard: the store must really be stampless, or this exercises the stamped
# branch and the "local edits" verdict would be the correct one.
if [ -f "$SL/tickets/.erg-assets" ]; then
    fail "stampless init: fixture carries a manifest (test would prove nothing)"
else
    OUT_SL=$($ERG init "$SL" 2>&1 || true)
    if echo "$OUT_SL" | grep -qF "has no usable .erg-assets stamp -- preserving"; then
        pass "stampless init: erg init reports the condition itself (0283 channel 2)"
    else
        fail "stampless init: no channel-2 report from erg init (got: $OUT_SL)"
    fi
    if echo "$OUT_SL" | grep -q "has local edits"; then
        fail "stampless init: claimed an edit no stamp attests (got: $OUT_SL)"
    else
        pass "stampless init: no local-edit attribution without a stamp to support it"
    fi
    if echo "$OUT_SL" | grep -qF "erg init --show .ergrc"; then
        pass "stampless init: the report names the command that answers the question"
    else
        fail "stampless init: the report leaves the reader with no next step (got: $OUT_SL)"
    fi
fi

# The same channel through the dry run, which prints its own short label from a
# separate string and so can regress on its own.
SLN="$TDIR/stampless-dryrun"
mkdir -p "$SLN/tickets"
touch "$SLN/tickets/erg"
printf '# an .ergrc that is not what this binary embeds\nlabels = whatever\n' > "$SLN/tickets/.ergrc"
OUT_SLN=$($ERG init -n "$SLN" 2>&1 || true)
if echo "$OUT_SLN" | grep -qF "would preserve (differs, no usable stamp, reason unknown)"; then
    pass "stampless init -n: the dry run reports the condition too"
else
    fail "stampless init -n: dry run gave no reason, or the wrong one (got: $OUT_SLN)"
fi
# Control for the two arms above: with a stamp on disk that the file no longer
# matches, "local edits" is a verdict the record supports and must survive.
# Without this arm, deleting the wording everywhere passes both.
SLC="$TDIR/stamped-edit"
mkdir -p "$SLC/tickets"
touch "$SLC/tickets/erg"
$ERG init "$SLC" >/dev/null 2>&1 || true
printf '# user edit\n' >> "$SLC/tickets/.ergrc"
OUT_SLC=$($ERG init "$SLC" 2>&1 || true)
if echo "$OUT_SLC" | grep -q "has local edits"; then
    pass "stamped edit: a file differing from its own stamp is still named a local edit"
else
    fail "stamped edit: the justified verdict was lost with the unjustified one (got: $OUT_SLC)"
fi

# --- erg init --show NAME prints the embedded copy (ticket 0292, defect 4) ---
# Every asset report tells a reader their file differs from the copy the binary
# ships, and no subcommand could show them that copy: init -n reports only THAT
# it differs, spec and integration dump different embedded files. So the
# messages pointed at the store's version-control history, which a directory
# under no version control does not have at all and an untracked asset does not
# have for that file. Byte identity is the contract -- the output is meant for
# diff and sha256sum -- so it is compared against the file a clean init wrote,
# not greped for a substring.
SHOWDIR="$TDIR/show"
mkdir -p "$SHOWDIR/tickets"
touch "$SHOWDIR/tickets/erg"
$ERG init "$SHOWDIR" >/dev/null 2>&1 || true
$ERG init --show .ergrc > "$TDIR/shown-ergrc" 2>"$TDIR/shown-err" || true
if [ -s "$TDIR/shown-ergrc" ] && cmp -s "$TDIR/shown-ergrc" "$SHOWDIR/tickets/.ergrc"; then
    pass "--show: prints the embedded .ergrc byte-identical to what init lays down"
else
    fail "--show: output differs from the installed asset (stderr: $(cat "$TDIR/shown-err"))"
fi
# The manifest is the independent witness: init stamped .ergrc with the SHA-256
# of the embedded copy, so --show piped through sha256sum must reproduce it.
# This is the check the ticket's verification list names, and it closes the loop
# without trusting either side of the comparison above on its own.
SHOWN_SUM=$($ERG init --show .ergrc | sha256sum | cut -d' ' -f1)
STAMPED_SUM=$(grep "^  \.ergrc sha256:" "$SHOWDIR/tickets/.erg-assets" | sed 's/.*sha256://')
if [ -n "$STAMPED_SUM" ] && [ "$SHOWN_SUM" = "$STAMPED_SUM" ]; then
    pass "--show: the printed bytes hash to the stamp a clean init recorded"
else
    fail "--show: hash mismatch against the manifest (shown=$SHOWN_SUM stamped=$STAMPED_SUM)"
fi
# The vendored helper is showable too: erg never writes tickets/erg-github, so
# README's re-vendor-by-hand recipe had no offline source for the shipped copy.
$ERG init --show erg-github > "$TDIR/shown-gh" 2>/dev/null || true
if [ -s "$TDIR/shown-gh" ] && cmp -s "$TDIR/shown-gh" src/go/assets/erg-github; then
    pass "--show: the vendored erg-github is showable, so re-vendoring has a source"
else
    fail "--show: erg-github does not match the embedded reference"
fi
# `erg init --show .ergrc` must never be read as an init of ./.ergrc. The flag
# consumes its argument; a parser that let it fall through to the positional
# DIR would report "binary not found", which reads like an unrelated
# environment problem rather than a parsing bug.
if $ERG init --show no-such-asset >/dev/null 2>"$TDIR/show-unknown"; then
    fail "--show with an unknown name must not report success"
else
    if grep -q "\.ergrc" "$TDIR/show-unknown" && grep -q "AGENTS\.md" "$TDIR/show-unknown"; then
        pass "--show: an unknown name errors and names what this binary ships"
    else
        fail "--show: the error names no alternative (got: $(cat "$TDIR/show-unknown"))"
    fi
fi
# --show needs no project: it reads nothing from disk. Run it from a directory
# with no tickets/ at all and it must still answer.
ERG_ABS_SHOW=$(readlink -f "$ERG")
EMPTYDIR="$TDIR/nostore"
mkdir -p "$EMPTYDIR"
if (cd "$EMPTYDIR" && "$ERG_ABS_SHOW" init --show .ergrc >/dev/null 2>&1); then
    pass "--show: answers without a project or a tickets/ directory"
else
    fail "--show: refused to print an embedded asset for want of a store"
fi

# --- Flags help: --force acknowledges the downgrade case ---
# "local edits are replaced" is no longer the whole story: on a rollback the
# overwrite is reported as "downgraded", and nothing there was locally edited.
# A user reading only the flag description would misread that line as a warning
# about their own edits.
HELPOUT=$($ERG init --help 2>&1) || true
if echo "$HELPOUT" | grep -q "downgrad"; then
    pass "help: --force description acknowledges the downgrade case"
else
    fail "help: --force description never mentions the downgrade case (got: $HELPOUT)"
fi

echo "init: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
