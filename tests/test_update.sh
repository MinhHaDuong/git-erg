#!/bin/sh
# Integration tests for: erg version, erg update
#
# erg update fetches the committed binary via git (no embedded network client),
# so these tests build local git remote fixtures rather than an HTTP server.
set -eu

ERG="${ERG_BIN:-build/erg}"
ERG_ABS=$(readlink -f "$ERG")
PASS=0; FAIL=0

# Local-path git remotes are central to these fixtures. Hardened hosts set
# protocol.file.allow=never globally (CVE-2022-39253 mitigation), which would
# break both our `git clone <path>` calls and erg's own child `git fetch <path>`.
# Inject the override via env so it reaches every git invocation we spawn *and*
# every git that erg spawns (env is inherited; the -c flag would not reach erg's
# child git). This mirrors how the fixtures pin commit.gpgsign.
export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0=protocol.file.allow
export GIT_CONFIG_VALUE_0=always

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg update/version ==="

# git_init DIR — create a repo with signing/identity that works unattended,
# independent of the caller's global git config (which may force commit signing).
git_init() {
    git init -q "$1"
    git -C "$1" config user.email test@example.com
    git -C "$1" config user.name test
    git -C "$1" config commit.gpgsign false
}

# Test: erg version exits 0 and prints structured output with hash and arch
VER=$("$ERG" version)
if echo "$VER" | grep -qE '^[[:space:]]+sha256:[[:space:]]+[0-9a-f]{64}$' && echo "$VER" | grep -q 'arch:'; then
    pass "version prints structured info"
else
    fail "version output: $VER"
fi

# --- git-fetch-based update tests ---

WORKROOT=$(mktemp -d)
cleanup() { rm -rf "$WORKROOT"; }
trap cleanup EXIT

# Build an "origin" remote whose committed tickets/erg differs from $ERG.
REMOTE="$WORKROOT/remote"
git_init "$REMOTE"
mkdir "$REMOTE/tickets"
cp "$ERG_ABS" "$REMOTE/tickets/erg"
printf 'X' >> "$REMOTE/tickets/erg"   # make the remote binary differ from $ERG
cat > "$REMOTE/tickets/0001-normal.erg" <<'ERGEOF'
%erg 0.1
Title: Normal ticket
Created: 2026-01-01
Author: a

--- log ---
2026-01-01T10:00Z a created

--- body ---
ERGEOF
git -C "$REMOTE" add -A
git -C "$REMOTE" commit -qm init
REMOTE_HASH=$(sha256sum "$REMOTE/tickets/erg" | cut -c1-12)

# Clone it; overwrite the checked-out binary with $ERG so update has work to do.
WORK="$WORKROOT/work"
git clone -q "$REMOTE" "$WORK"
cp "$ERG_ABS" "$WORK/tickets/erg"
# A legacy Status: ticket so the post-update migration hint fires.
cat > "$WORK/tickets/0002-legacy.erg" <<'ERGEOF'
%erg 0.1
Title: Legacy ticket
Created: 2026-01-01
Author: a
Status: open

--- log ---
2026-01-01T10:00Z a created

--- body ---
ERGEOF
LEGACY_BEFORE=$(cat "$WORK/tickets/0002-legacy.erg")

# Test: update from origin replaces the stale binary with the committed one.
OUT=$(cd "$WORK" && ERG_TICKET_DIR="$WORK/tickets" ./tickets/erg update 2>&1 || true)
AFTER_HASH=$(sha256sum "$WORK/tickets/erg" | cut -c1-12)
if [ "$AFTER_HASH" = "$REMOTE_HASH" ]; then
    pass "update replaces stale binary with origin's committed binary"
else
    fail "update did not replace binary: after=$AFTER_HASH want=$REMOTE_HASH ($OUT)"
fi

# Test: update emits the migrate hint for legacy Status: tickets...
if echo "$OUT" | grep -q "erg migrate"; then
    pass "update emits migrate hint"
else
    fail "update missing migrate hint: $OUT"
fi
# ...but never rewrites ticket files itself.
if [ "$(cat "$WORK/tickets/0002-legacy.erg")" = "$LEGACY_BEFORE" ]; then
    pass "update does not rewrite ticket files"
else
    fail "update rewrote ticket files"
fi

# Test: running update again is a no-op (hash now matches origin).
OUT=$(cd "$WORK" && ERG_TICKET_DIR="$WORK/tickets" ./tickets/erg update 2>&1 || true)
if echo "$OUT" | grep -q "already up to date"; then
    pass "update on hash match is a no-op"
else
    fail "update should report already up to date: $OUT"
fi

# Test: git fetch only touches the binary — working-tree assets stay put.
if [ "$(cat "$WORK/tickets/0001-normal.erg")" = "$(cat "$REMOTE/tickets/0001-normal.erg")" ]; then
    pass "update does not rewrite managed assets"
else
    fail "update altered a working-tree asset"
fi

# Test: ERG_UPDATE_URL overrides origin — fetch upstream's binary instead.
UPSTREAM="$WORKROOT/upstream"
git_init "$UPSTREAM"
mkdir "$UPSTREAM/tickets"
cp "$ERG_ABS" "$UPSTREAM/tickets/erg"
printf 'UPSTREAM' >> "$UPSTREAM/tickets/erg"   # distinct from both $ERG and origin
git -C "$UPSTREAM" add -A
git -C "$UPSTREAM" commit -qm upstream
UPSTREAM_HASH=$(sha256sum "$UPSTREAM/tickets/erg" | cut -c1-12)

WORK2="$WORKROOT/work2"
git clone -q "$REMOTE" "$WORK2"
cp "$ERG_ABS" "$WORK2/tickets/erg"
OUT=$(cd "$WORK2" && ERG_TICKET_DIR="$WORK2/tickets" ERG_UPDATE_URL="$UPSTREAM" ./tickets/erg update 2>&1 || true)
WORK2_HASH=$(sha256sum "$WORK2/tickets/erg" | cut -c1-12)
if [ "$WORK2_HASH" = "$UPSTREAM_HASH" ]; then
    pass "ERG_UPDATE_URL override fetches from the given remote, not origin"
else
    fail "override ignored: after=$WORK2_HASH want=$UPSTREAM_HASH ($OUT)"
fi

# Test: .ergrc [update] url override is honored when the env var is unset.
WORK3="$WORKROOT/work3"
git clone -q "$REMOTE" "$WORK3"
cp "$ERG_ABS" "$WORK3/tickets/erg"
printf '[update]\nurl = %s\n' "$UPSTREAM" > "$WORK3/tickets/.ergrc"
OUT=$(cd "$WORK3" && ERG_TICKET_DIR="$WORK3/tickets" ./tickets/erg update 2>&1 || true)
WORK3_HASH=$(sha256sum "$WORK3/tickets/erg" | cut -c1-12)
if [ "$WORK3_HASH" = "$UPSTREAM_HASH" ]; then
    pass ".ergrc [update] url override is honored"
else
    fail ".ergrc override ignored: after=$WORK3_HASH want=$UPSTREAM_HASH ($OUT)"
fi

# Test: offline / no reachable remote exits 0 and leaves the binary untouched.
OFFLINE="$WORKROOT/offline"
mkdir -p "$OFFLINE/tickets"
cp "$ERG_ABS" "$OFFLINE/tickets/erg"
cp "$WORK/tickets/0001-normal.erg" "$OFFLINE/tickets/0001-normal.erg"
OFFLINE_BEFORE=$(sha256sum "$OFFLINE/tickets/erg" | cut -c1-12)
if (cd "$OFFLINE" && ERG_TICKET_DIR="$OFFLINE/tickets" ./tickets/erg update >/dev/null 2>&1); then
    OFFLINE_AFTER=$(sha256sum "$OFFLINE/tickets/erg" | cut -c1-12)
    if [ "$OFFLINE_BEFORE" = "$OFFLINE_AFTER" ]; then
        pass "update offline exits 0 and leaves binary untouched"
    else
        fail "update offline changed the binary"
    fi
else
    fail "update offline should exit 0"
fi

# Test: with no discoverable ticket store, update refuses rather than pulling
# the binary from whatever unrelated repo the user happens to be standing in.
# The hijack remote commits a (distinct) tickets/erg blob; the work repo wires
# it as origin but has NO checked-out tickets/ dir and NO .erg files, so store
# discovery finds nothing. Before the guard, the cwd-repo fallback would fetch
# origin's tickets/erg and overwrite the running binary.
HJ_REMOTE="$WORKROOT/hijack-remote"
git_init "$HJ_REMOTE"
mkdir "$HJ_REMOTE/tickets"
cp "$ERG_ABS" "$HJ_REMOTE/tickets/erg"
printf 'HIJACK' >> "$HJ_REMOTE/tickets/erg"   # distinct hash — would show if pulled
git -C "$HJ_REMOTE" add -A
git -C "$HJ_REMOTE" commit -qm hijack
HJ_WORK="$WORKROOT/hijack-work"
git_init "$HJ_WORK"
git -C "$HJ_WORK" remote add origin "$HJ_REMOTE"   # origin wired, nothing checked out
# Run a copy of erg from a dir with no .erg files and not named "tickets".
mkdir "$HJ_WORK/run"
cp "$ERG_ABS" "$HJ_WORK/run/erg"
HJ_BEFORE=$(sha256sum "$HJ_WORK/run/erg" | cut -c1-12)
OUT=$(cd "$HJ_WORK" && ERG_TICKET_DIR= ./run/erg update 2>&1 || true)
HJ_AFTER=$(sha256sum "$HJ_WORK/run/erg" | cut -c1-12)
if [ "$HJ_BEFORE" = "$HJ_AFTER" ] && echo "$OUT" | grep -q "no git-erg ticket store"; then
    pass "update refuses when no ticket store is found (no cwd-repo hijack)"
else
    fail "update without a store changed the binary or gave no warning: $OUT"
fi

# Test: the binary carries no embedded network/TLS client. `erg update` now
# shells out to git, so the offline invariant holds everywhere — guard it.
if grep -rEn 'net/http|crypto/tls' src/go/ >/dev/null 2>&1; then
    fail "source imports net/http or crypto/tls — erg must carry no network code"
else
    pass "no net/http or crypto/tls in source (offline invariant)"
fi

# --- vcsRevision-based outdated detection tests ---

# Test: erg version output includes revision: line when vcsRevision is embedded.
VER2=$("$ERG" version 2>&1)
if echo "$VER2" | grep -qE '^\s+revision:'; then
    pass "version: revision: line present in output"
else
    fail "version: revision: line missing from output: $VER2"
fi

# Test: a binary claiming the same vcsRevision is NOT marked [outdated].
# We create a shell stub that prints erg version output with the same revision
# as the running binary, but a different hash. Place it in a temp PATH dir so
# erg discovers it, then assert no [outdated] label appears.
SELF_REVISION=$(echo "$VER2" | grep -E '^\s+revision:' | sed 's/.*revision:[[:space:]]*//')
VERSION_TMPDIR=$(mktemp -d)
STUB="$VERSION_TMPDIR/erg"
cat > "$STUB" <<STUBEOF
#!/bin/sh
if [ "\$1" = "version" ]; then
    echo "erg version"
    echo "  path:    $VERSION_TMPDIR/erg"
    echo "  sha256:  aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
    echo "  built:   2020-01-01T00:00:00Z"
    echo "  revision: $SELF_REVISION"
    echo "  arch:    linux/amd64"
fi
STUBEOF
chmod +x "$STUB"

OUT=$(PATH="$VERSION_TMPDIR:$PATH" "$ERG" version 2>&1)
if echo "$OUT" | grep -F "$VERSION_TMPDIR/erg" | grep -q "\[outdated"; then
    fail "version: same-revision stub incorrectly marked [outdated]: $OUT"
else
    pass "version: same-revision binary not marked [outdated]"
fi
rm -rf "$VERSION_TMPDIR"

# Test: a binary with a different (older) vcsRevision IS marked [outdated].
VERSION_TMPDIR2=$(mktemp -d)
STUB2="$VERSION_TMPDIR2/erg"
cat > "$STUB2" <<STUBEOF2
#!/bin/sh
if [ "\$1" = "version" ]; then
    echo "erg version"
    echo "  path:    $VERSION_TMPDIR2/erg"
    echo "  sha256:  deadbeefcafe00112233445566778899aabbccddeeff00112233445566778899"
    echo "  built:   2020-01-01T00:00:00Z"
    echo "  revision: olddeadbeef"
    echo "  arch:    linux/amd64"
fi
STUBEOF2
chmod +x "$STUB2"

OUT2=$(PATH="$VERSION_TMPDIR2:$PATH" "$ERG" version 2>&1)
if echo "$OUT2" | grep -q "\[outdated"; then
    pass "version: older-revision binary marked [outdated]"
else
    fail "version: older-revision binary not marked [outdated]: $OUT2"
fi
rm -rf "$VERSION_TMPDIR2"

echo "update: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
