#!/bin/sh
# Integration tests for: erg next-id
set -eu

ERG="${ERG_BIN:-build/erg}"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

TDIR=$(mktemp -d)
trap 'rm -rf "$TDIR"' EXIT

echo "=== erg next-id ==="

# --- Missing dir → 0001 ---
out=$($ERG next-id "$TDIR/nonexistent")
if [ "$out" = "0001" ]; then
    pass "missing dir returns 0001"
else
    fail "missing dir returns 0001 (got: $out)"
fi

# --- Empty dir → 0001 ---
mkdir -p "$TDIR/empty"
out=$($ERG next-id "$TDIR/empty")
if [ "$out" = "0001" ]; then
    pass "empty dir returns 0001"
else
    fail "empty dir returns 0001 (got: $out)"
fi

# --- Single ticket 0042-foo.erg → 0043 ---
mkdir -p "$TDIR/single"
touch "$TDIR/single/0042-foo.erg"
out=$($ERG next-id "$TDIR/single")
if [ "$out" = "0043" ]; then
    pass "single ticket 0042 returns 0043"
else
    fail "single ticket 0042 returns 0043 (got: $out)"
fi

# --- Bare numeric filename 0042.erg → 0043 ---
mkdir -p "$TDIR/bare"
touch "$TDIR/bare/0042.erg"
out=$($ERG next-id "$TDIR/bare")
if [ "$out" = "0043" ]; then
    pass "bare numeric filename 0042.erg returns 0043"
else
    fail "bare numeric filename 0042.erg returns 0043 (got: $out)"
fi

# --- Gap in sequence (0001, 0005) → 0006 ---
mkdir -p "$TDIR/gap"
touch "$TDIR/gap/0001-alpha.erg"
touch "$TDIR/gap/0005-beta.erg"
out=$($ERG next-id "$TDIR/gap")
if [ "$out" = "0006" ]; then
    pass "gap sequence returns 0006"
else
    fail "gap sequence returns 0006 (got: $out)"
fi

# --- Closed/archive subdirectory IDs ARE counted (recursive scan) ---
mkdir -p "$TDIR/scoped/archive"
touch "$TDIR/scoped/0003-low.erg"
touch "$TDIR/scoped/archive/0099-high.erg"
out=$($ERG next-id "$TDIR/scoped")
if [ "$out" = "0100" ]; then
    pass "archive subdir counted"
else
    fail "archive subdir counted (got: $out)"
fi

# --- Non-.erg files ignored ---
mkdir -p "$TDIR/filter"
touch "$TDIR/filter/0010-real.erg"
touch "$TDIR/filter/0099-fake.txt"
out=$($ERG next-id "$TDIR/filter")
if [ "$out" = "0011" ]; then
    pass "non-.erg file ignored"
else
    fail "non-.erg file ignored (got: $out)"
fi

# --- .erg file without numeric prefix ignored ---
mkdir -p "$TDIR/nonum"
touch "$TDIR/nonum/0005-valid.erg"
touch "$TDIR/nonum/notes.erg"
out=$($ERG next-id "$TDIR/nonum")
if [ "$out" = "0006" ]; then
    pass "non-numeric .erg file ignored"
else
    fail "non-numeric .erg file ignored (got: $out)"
fi

# --- Custom dir argument is used (not hardcoded default) ---
mkdir -p "$TDIR/custom"
touch "$TDIR/custom/0007-item.erg"
out=$($ERG next-id "$TDIR/custom")
if [ "$out" = "0008" ]; then
    pass "custom dir argument used"
else
    fail "custom dir argument used (got: $out)"
fi

# --- Cross-worktree: sibling worktree with uncommitted higher-ID ticket ---
# Live incident regression from ticket 0140: parallel agents in sibling
# worktrees previously collided on the same ID. The scan must now see
# uncommitted .erg files in sibling worktrees. All tempdirs are anchored
# under $TDIR so the top-level EXIT trap cleans them on any failure.
if command -v git >/dev/null 2>&1; then
    XWR="$TDIR/xwr"
    mkdir -p "$XWR/repo"
    git init -q -b main "$XWR/repo"
    git -C "$XWR/repo" config user.email "test@example.com"
    git -C "$XWR/repo" config user.name "Test"
    git -C "$XWR/repo" config commit.gpgsign false
    mkdir -p "$XWR/repo/tickets"
    touch "$XWR/repo/tickets/0050-primary.erg"
    echo "init" > "$XWR/repo/README"
    git -C "$XWR/repo" add README
    git -C "$XWR/repo" commit -q -m "init"
    git -C "$XWR/repo" worktree add -q -b wt2-branch "$XWR/wt2" >/dev/null 2>&1
    mkdir -p "$XWR/wt2/tickets"
    touch "$XWR/wt2/tickets/0123-uncommitted-in-sibling.erg"
    out=$("$ERG" next-id "$XWR/repo/tickets")
    if [ "$out" = "0124" ]; then
        pass "cross-worktree: skips uncommitted ticket in sibling worktree"
    else
        fail "cross-worktree: skips uncommitted ticket in sibling worktree (got: $out)"
    fi

    # --- Cross-worktree: ticket committed on an unchecked-out branch ---
    YWR="$TDIR/ywr"
    mkdir -p "$YWR/repo"
    git init -q -b main "$YWR/repo"
    git -C "$YWR/repo" config user.email "test@example.com"
    git -C "$YWR/repo" config user.name "Test"
    git -C "$YWR/repo" config commit.gpgsign false
    echo "init" > "$YWR/repo/README"
    git -C "$YWR/repo" add README
    git -C "$YWR/repo" commit -q -m "init"
    git -C "$YWR/repo" checkout -q -b other
    mkdir -p "$YWR/repo/tickets"
    touch "$YWR/repo/tickets/0200-on-other-branch.erg"
    git -C "$YWR/repo" add tickets/0200-on-other-branch.erg
    git -C "$YWR/repo" commit -q -m "add 0200"
    git -C "$YWR/repo" checkout -q main
    rm -rf "$YWR/repo/tickets"
    mkdir -p "$YWR/repo/tickets"
    out=$("$ERG" next-id "$YWR/repo/tickets")
    if [ "$out" = "0201" ]; then
        pass "cross-worktree: skips ticket committed on un-checked-out branch"
    else
        fail "cross-worktree: skips ticket committed on un-checked-out branch (got: $out)"
    fi

    # --- Cross-worktree: dir outside a git repo still works ---
    ZWR="$TDIR/zwr"
    mkdir -p "$ZWR"
    touch "$ZWR/0007-only-local.erg"
    out=$("$ERG" next-id "$ZWR")
    if [ "$out" = "0008" ]; then
        pass "cross-worktree: dir outside any git repo falls back to local scan"
    else
        fail "cross-worktree: dir outside any git repo falls back to local scan (got: $out)"
    fi

    # --- Branch whose name ends in /HEAD must still be scanned (ticket 0250) ---
    # The knownBranches /HEAD suffix filter dropped real branches like
    # feature/HEAD, hiding their IDs and risking reuse. Distinguish the
    # origin/HEAD symref by %(symref), not by name suffix.
    HWR="$TDIR/hwr"
    mkdir -p "$HWR/repo"
    git init -q -b main "$HWR/repo"
    git -C "$HWR/repo" config user.email "test@example.com"
    git -C "$HWR/repo" config user.name "Test"
    git -C "$HWR/repo" config commit.gpgsign false
    echo "init" > "$HWR/repo/README"
    git -C "$HWR/repo" add README
    git -C "$HWR/repo" commit -q -m "init"
    git -C "$HWR/repo" checkout -q -b feature/HEAD
    mkdir -p "$HWR/repo/tickets"
    touch "$HWR/repo/tickets/0300-on-head-branch.erg"
    git -C "$HWR/repo" add tickets/0300-on-head-branch.erg
    git -C "$HWR/repo" commit -q -m "add 0300"
    git -C "$HWR/repo" checkout -q main
    rm -rf "$HWR/repo/tickets"; mkdir -p "$HWR/repo/tickets"
    out=$("$ERG" next-id "$HWR/repo/tickets")
    if [ "$out" = "0301" ]; then
        pass "cross-worktree: counts a ticket on a feature/HEAD branch"
    else
        fail "cross-worktree: counts a ticket on a feature/HEAD branch (got: $out)"
    fi

    # --- Symlinked store path still runs the branch scan (ticket 0250) ---
    # filepath.Abs left the symlink unresolved while git --show-toplevel
    # resolves it, so the worktree-boundary check tripped and silently skipped
    # the branch scan -> ID reuse. Resolve symlinks before the boundary test.
    SWR="$TDIR/swr"
    mkdir -p "$SWR/real"
    git init -q -b main "$SWR/real"
    git -C "$SWR/real" config user.email "test@example.com"
    git -C "$SWR/real" config user.name "Test"
    git -C "$SWR/real" config commit.gpgsign false
    echo "init" > "$SWR/real/README"
    git -C "$SWR/real" add README
    git -C "$SWR/real" commit -q -m "init"
    git -C "$SWR/real" checkout -q -b feat
    mkdir -p "$SWR/real/tickets"
    touch "$SWR/real/tickets/0400-on-branch.erg"
    git -C "$SWR/real" add tickets/0400-on-branch.erg
    git -C "$SWR/real" commit -q -m "add 0400"
    git -C "$SWR/real" checkout -q main
    rm -rf "$SWR/real/tickets"; mkdir -p "$SWR/real/tickets"
    ln -s "$SWR/real" "$SWR/link"
    out=$("$ERG" next-id "$SWR/link/tickets")
    if [ "$out" = "0401" ]; then
        pass "symlinked store path still scans branch tips (no ID reuse)"
    else
        fail "symlinked store path still scans branch tips (got: $out)"
    fi
fi

# --- Range exhaustion: 9999 exists -> error, no 5-digit ID ---
mkdir -p "$TDIR/maxed"
touch "$TDIR/maxed/9999-bad.erg"
if out=$($ERG next-id "$TDIR/maxed" 2>&1); then
    fail "next-id should error when range is exhausted (got: $out)"
else
    pass "next-id errors on range exhaustion"
fi

# --- Stray 5-digit file is ignored, normal IDs work ---
mkdir -p "$TDIR/stray5"
touch "$TDIR/stray5/0005-valid.erg"
touch "$TDIR/stray5/10000-stray.erg"
out=$($ERG next-id "$TDIR/stray5")
if [ "$out" = "0006" ]; then
    pass "stray 5-digit file ignored, next-id returns 0006"
else
    fail "stray 5-digit file ignored, next-id returns 0006 (got: $out)"
fi

# unknown flag rejection (ticket 0180)
    out=$($ERG next-id --bogus 2>&1) && rc=0 || rc=$?
    if [ "$rc" -ne 0 ] && echo "$out" | grep -q "unknown flag"; then
        pass "unknown flag rejected with usage message"
    else
        fail "unknown flag not rejected (rc=$rc, got: $out)"
    fi

echo "next-id: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
