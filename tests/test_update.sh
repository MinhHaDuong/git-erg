#!/bin/sh
# Integration tests for: erg version, erg sync
#
# erg sync fetches the committed binary via git (no embedded network client),
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

echo "=== erg sync/version ==="

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

# --- git-fetch-based sync tests ---

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

# Clone it; overwrite the checked-out binary with $ERG so sync has work to do.
WORK="$WORKROOT/work"
git clone -q "$REMOTE" "$WORK"
cp "$ERG_ABS" "$WORK/tickets/erg"
# A legacy Status: ticket so the post-sync migration hint fires.
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

# Test: sync from project origin replaces the stale binary with the committed one.
OUT=$(cd "$WORK" && ERG_TICKET_DIR="$WORK/tickets" ./tickets/erg sync 2>&1 || true)
AFTER_HASH=$(sha256sum "$WORK/tickets/erg" | cut -c1-12)
if [ "$AFTER_HASH" = "$REMOTE_HASH" ]; then
    pass "sync replaces stale binary with origin's committed binary"
else
    fail "sync did not replace binary: after=$AFTER_HASH want=$REMOTE_HASH ($OUT)"
fi
if echo "$OUT" | grep -q "project origin"; then
    pass "default sync names project origin as its source"
else
    fail "default sync did not identify project origin: $OUT"
fi

# Test: sync emits the migrate hint for legacy Status: tickets...
if echo "$OUT" | grep -q "' migrate '"; then
    pass "sync emits migrate hint"
else
    fail "sync missing migrate hint: $OUT"
fi
# ...but never rewrites ticket files itself.
if [ "$(cat "$WORK/tickets/0002-legacy.erg")" = "$LEGACY_BEFORE" ]; then
    pass "sync does not rewrite ticket files"
else
    fail "sync rewrote ticket files"
fi

# Test: running sync again is a no-op (hash now matches origin).
OUT=$(cd "$WORK" && ERG_TICKET_DIR="$WORK/tickets" ./tickets/erg sync 2>&1 || true)
if echo "$OUT" | grep -q "already synchronized with project origin"; then
    pass "sync on hash match is a no-op"
else
    fail "sync should name project origin in the no-op result: $OUT"
fi

# Test: the old name remains a loud compatibility alias, not a second meaning.
OUT=$(cd "$WORK" && ERG_TICKET_DIR="$WORK/tickets" ./tickets/erg update 2>&1 || true)
if echo "$OUT" | grep -q "'update' is now 'sync'" && echo "$OUT" | grep -q "project origin"; then
    pass "update compatibility alias points to sync and preserves source semantics"
else
    fail "update alias was silent or changed source semantics: $OUT"
fi

# Test: git fetch only touches the binary — working-tree assets stay put.
if [ "$(cat "$WORK/tickets/0001-normal.erg")" = "$(cat "$REMOTE/tickets/0001-normal.erg")" ]; then
    pass "sync does not rewrite managed assets"
else
    fail "sync altered a working-tree asset"
fi

# Test: ERG_UPDATE_URL overrides origin — fetch upstream's binary instead.
UPSTREAM="$WORKROOT/upstream"
git_init "$UPSTREAM"
printf 'history that adopters do not need\n' > "$UPSTREAM/history.txt"
git -C "$UPSTREAM" add -A
git -C "$UPSTREAM" commit -qm upstream-parent
UPSTREAM_PARENT=$(git -C "$UPSTREAM" rev-parse HEAD)
mkdir "$UPSTREAM/tickets"
cp "$ERG_ABS" "$UPSTREAM/tickets/erg"
printf 'UPSTREAM' >> "$UPSTREAM/tickets/erg"   # distinct from both $ERG and origin
git -C "$UPSTREAM" add -A
git -C "$UPSTREAM" commit -qm upstream
UPSTREAM_URL="file://$UPSTREAM"
UPSTREAM_HASH=$(sha256sum "$UPSTREAM/tickets/erg" | cut -c1-12)
if [ "$UPSTREAM_HASH" != "$REMOTE_HASH" ]; then
    pass "source-selection fixture gives origin and upstream distinct hashes"
else
    fail "source-selection fixture is vacuous: origin and upstream hashes match"
fi

# Test: --upstream selects the canonical git-erg source even when the custom
# environment override points elsewhere. Git rewrites the canonical URL to the
# local fixture, keeping the test disconnected while exercising the real URL.
WORKUP="$WORKROOT/work-upstream"
git clone -q "$REMOTE" "$WORKUP"
cp "$ERG_ABS" "$WORKUP/tickets/erg"
OUT=$(cd "$WORKUP" && \
    GIT_CONFIG_COUNT=2 \
    GIT_CONFIG_KEY_1="url.$UPSTREAM_URL.insteadOf" \
    GIT_CONFIG_VALUE_1="https://github.com/MinhHaDuong/git-erg.git" \
    ERG_UPDATE_URL="$REMOTE" \
    ERG_TICKET_DIR="$WORKUP/tickets" \
    ./tickets/erg sync --upstream 2>&1 || true)
WORKUP_HASH=$(sha256sum "$WORKUP/tickets/erg" | cut -c1-12)
if [ "$WORKUP_HASH" = "$UPSTREAM_HASH" ] && echo "$OUT" | grep -q "git-erg upstream"; then
    pass "--upstream selects and names git-erg ahead of custom-source overrides"
else
    fail "--upstream selected the wrong source: after=$WORKUP_HASH want=$UPSTREAM_HASH ($OUT)"
fi
if git -C "$WORKUP" cat-file -e "$UPSTREAM_PARENT^{commit}" 2>/dev/null; then
    fail "--upstream imported history older than the fetched tip"
else
    pass "--upstream fetch is shallow and does not import git-erg history"
fi

# An explicit upstream import crosses a review boundary. The fetched bytes must
# be installed but never executed by the old process before the operator can
# inspect them.
EVIL_UPSTREAM="$WORKROOT/evil-upstream"
git_init "$EVIL_UPSTREAM"
mkdir "$EVIL_UPSTREAM/tickets"
EVIL_SENTINEL="$WORKROOT/import-was-executed"
printf '#!/bin/sh\ntouch "%s"\n' "$EVIL_SENTINEL" > "$EVIL_UPSTREAM/tickets/erg"
chmod +x "$EVIL_UPSTREAM/tickets/erg"
git -C "$EVIL_UPSTREAM" add -A
git -C "$EVIL_UPSTREAM" commit -qm malicious-fixture
EVIL_UPSTREAM_URL="file://$EVIL_UPSTREAM"
WORKREVIEW="$WORKROOT/work-review"
git clone -q "$REMOTE" "$WORKREVIEW"
cp "$ERG_ABS" "$WORKREVIEW/tickets/erg"
OUT=$(cd "$WORKREVIEW" && \
    GIT_CONFIG_COUNT=2 \
    GIT_CONFIG_KEY_1="url.$EVIL_UPSTREAM_URL.insteadOf" \
    GIT_CONFIG_VALUE_1="https://github.com/MinhHaDuong/git-erg.git" \
    ERG_TICKET_DIR="$WORKREVIEW/tickets" \
    ./tickets/erg sync --upstream 2>&1 || true)
if [ ! -e "$EVIL_SENTINEL" ] && echo "$OUT" | grep -q "without executing it"; then
    pass "upstream import is not executed before review"
else
    fail "upstream import executed unreviewed bytes or omitted its review warning: $OUT"
fi

# --upstream always reads git-erg's canonical tickets/erg, even when the
# adopter keeps its own store under another repo-relative directory.
WORKLAYOUT="$WORKROOT/work-layout"
git clone -q "$REMOTE" "$WORKLAYOUT"
mkdir "$WORKLAYOUT/issues"
cp "$ERG_ABS" "$WORKLAYOUT/issues/erg"
OUT=$(cd "$WORKLAYOUT" && \
    GIT_CONFIG_COUNT=2 \
    GIT_CONFIG_KEY_1="url.$UPSTREAM_URL.insteadOf" \
    GIT_CONFIG_VALUE_1="https://github.com/MinhHaDuong/git-erg.git" \
    ERG_TICKET_DIR="$WORKLAYOUT/issues" \
    ./issues/erg sync --upstream 2>&1 || true)
LAYOUT_HASH=$(sha256sum "$WORKLAYOUT/issues/erg" | cut -c1-12)
if [ "$LAYOUT_HASH" = "$UPSTREAM_HASH" ]; then
    pass "upstream import uses canonical tickets/erg with a noncanonical local store"
else
    fail "upstream import derived its source path from the adopter layout: $OUT"
fi

# Configured sources have their own repository layout too. A noncanonical local
# store must still request canonical tickets/erg from the configured source.
WORKCUSTOMLAYOUT="$WORKROOT/work-custom-layout"
git clone -q "$REMOTE" "$WORKCUSTOMLAYOUT"
mkdir "$WORKCUSTOMLAYOUT/issues"
cp "$ERG_ABS" "$WORKCUSTOMLAYOUT/issues/erg"
OUT=$(cd "$WORKCUSTOMLAYOUT" && \
    ERG_UPDATE_URL="$UPSTREAM" \
    ERG_TICKET_DIR="$WORKCUSTOMLAYOUT/issues" \
    ./issues/erg sync 2>&1 || true)
CUSTOM_LAYOUT_HASH=$(sha256sum "$WORKCUSTOMLAYOUT/issues/erg" | cut -c1-12)
if [ "$CUSTOM_LAYOUT_HASH" = "$UPSTREAM_HASH" ]; then
    pass "custom source uses canonical tickets/erg with a noncanonical local store"
else
    fail "custom source derived its blob path from the adopter layout: $OUT"
fi

# sync targets the project's vendored binary, not whichever system/PATH copy
# happened to invoke it.
WORKPATH="$WORKROOT/work-path"
git clone -q "$REMOTE" "$WORKPATH"
cp "$ERG_ABS" "$WORKPATH/tickets/erg"
mkdir "$WORKROOT/bin"
cp "$ERG_ABS" "$WORKROOT/bin/erg"
LAUNCHER_BEFORE=$(sha256sum "$WORKROOT/bin/erg" | cut -c1-12)
OUT=$(cd "$WORKPATH" && ERG_TICKET_DIR="$WORKPATH/tickets" "$WORKROOT/bin/erg" sync 2>&1 || true)
LAUNCHER_AFTER=$(sha256sum "$WORKROOT/bin/erg" | cut -c1-12)
PATH_TARGET_HASH=$(sha256sum "$WORKPATH/tickets/erg" | cut -c1-12)
if [ "$LAUNCHER_BEFORE" = "$LAUNCHER_AFTER" ] && [ "$PATH_TARGET_HASH" = "$REMOTE_HASH" ]; then
    pass "sync updates the vendored binary without replacing its launcher"
else
    fail "sync replaced its launcher or missed the vendored target: $OUT"
fi

# A predictable legacy temp name may already be a symlink. Atomic replacement
# must neither follow it nor truncate its target.
WORKTMP="$WORKROOT/work-temp"
git clone -q "$REMOTE" "$WORKTMP"
cp "$ERG_ABS" "$WORKTMP/tickets/erg"
TMP_SENTINEL="$WORKROOT/outside-temp-target"
printf 'must survive\n' > "$TMP_SENTINEL"
ln -s "$TMP_SENTINEL" "$WORKTMP/tickets/erg.tmp"
OUT=$(cd "$WORKTMP" && ERG_TICKET_DIR="$WORKTMP/tickets" ./tickets/erg sync 2>&1 || true)
if [ "$(cat "$TMP_SENTINEL")" = "must survive" ] && \
    [ "$(sha256sum "$WORKTMP/tickets/erg" | cut -c1-12)" = "$REMOTE_HASH" ]; then
    pass "sync uses an exclusive temp and cannot follow a planted erg.tmp symlink"
else
    fail "sync followed a planted temp symlink or failed atomic replacement: $OUT"
fi

WORK2="$WORKROOT/work2"
git clone -q "$REMOTE" "$WORK2"
cp "$ERG_ABS" "$WORK2/tickets/erg"
OUT=$(cd "$WORK2" && ERG_TICKET_DIR="$WORK2/tickets" ERG_UPDATE_URL="$UPSTREAM" ./tickets/erg sync 2>&1 || true)
WORK2_HASH=$(sha256sum "$WORK2/tickets/erg" | cut -c1-12)
if [ "$WORK2_HASH" = "$UPSTREAM_HASH" ]; then
    pass "ERG_UPDATE_URL override fetches from the given remote, not origin"
else
    fail "override ignored: after=$WORK2_HASH want=$UPSTREAM_HASH ($OUT)"
fi
if echo "$OUT" | grep -q "configured source (ERG_UPDATE_URL: local path, id " && ! echo "$OUT" | grep -qF "$UPSTREAM"; then
    pass "environment source is identified without echoing its URL"
else
    fail "environment source label is absent or leaked its URL: $OUT"
fi

# Test: .ergrc [update] url override is honored when the env var is unset.
WORK3="$WORKROOT/work3"
git clone -q "$REMOTE" "$WORK3"
cp "$ERG_ABS" "$WORK3/tickets/erg"
printf '[update]\nurl = %s\n' "$UPSTREAM" > "$WORK3/tickets/.ergrc"
OUT=$(cd "$WORK3" && ERG_TICKET_DIR="$WORK3/tickets" ./tickets/erg sync 2>&1 || true)
WORK3_HASH=$(sha256sum "$WORK3/tickets/erg" | cut -c1-12)
if [ "$WORK3_HASH" = "$UPSTREAM_HASH" ]; then
    pass ".ergrc [update] url override is honored"
else
    fail ".ergrc override ignored: after=$WORK3_HASH want=$UPSTREAM_HASH ($OUT)"
fi
if echo "$OUT" | grep -q "configured source (tickets/.ergrc: local path, id " && ! echo "$OUT" | grep -qF "$UPSTREAM"; then
    pass "config source is identified without echoing its URL"
else
    fail "config source label is absent or leaked its URL: $OUT"
fi

# A reachable source that does not contain canonical tickets/erg is a hard
# source/layout error, not an offline condition to swallow with exit 0.
MISSING="$WORKROOT/missing-blob"
git_init "$MISSING"
printf 'no vendored binary here\n' > "$MISSING/README"
git -C "$MISSING" add -A
git -C "$MISSING" commit -qm missing-blob
WORKMISSING="$WORKROOT/work-missing"
git clone -q "$REMOTE" "$WORKMISSING"
cp "$ERG_ABS" "$WORKMISSING/tickets/erg"
set +e
OUT=$(cd "$WORKMISSING" && \
    ERG_TICKET_DIR="$WORKMISSING/tickets" \
    ERG_UPDATE_URL="$MISSING" \
    ./tickets/erg sync 2>&1)
MISSING_RC=$?
set -e
if [ "$MISSING_RC" -ne 0 ] && echo "$OUT" | grep -q "could not read the committed vendored binary"; then
    pass "reachable source missing tickets/erg is reported as a hard error"
else
    fail "missing source blob was silent or exited 0 (rc=$MISSING_RC, out: $OUT)"
fi

# Git includes a failed fetch URL in stderr. sync must suppress that raw
# diagnostic and print only its sanitized source identity.
WORKSECRET="$WORKROOT/work-secret"
git clone -q "$REMOTE" "$WORKSECRET"
cp "$ERG_ABS" "$WORKSECRET/tickets/erg"
SECRET_URL='https://user:TOPSECRET@127.0.0.1:1/TOPSECRET/repo.git?access_token=TOPSECRET'
OUT=$(cd "$WORKSECRET" && \
    GIT_TERMINAL_PROMPT=0 \
    ERG_TICKET_DIR="$WORKSECRET/tickets" \
    ERG_UPDATE_URL="$SECRET_URL" \
    ./tickets/erg sync 2>&1 || true)
if echo "$OUT" | grep -q "host 127.0.0.1:1, id " && ! echo "$OUT" | grep -q "TOPSECRET"; then
    pass "failed custom fetch identifies its source without leaking credentials"
else
    fail "failed custom fetch omitted its safe identity or leaked credentials: $OUT"
fi

# Test: offline / no reachable remote exits 0 and leaves the binary untouched.
OFFLINE="$WORKROOT/offline"
mkdir -p "$OFFLINE/tickets"
cp "$ERG_ABS" "$OFFLINE/tickets/erg"
cp "$WORK/tickets/0001-normal.erg" "$OFFLINE/tickets/0001-normal.erg"
OFFLINE_BEFORE=$(sha256sum "$OFFLINE/tickets/erg" | cut -c1-12)
if (cd "$OFFLINE" && ERG_TICKET_DIR="$OFFLINE/tickets" ./tickets/erg sync >/dev/null 2>&1); then
    OFFLINE_AFTER=$(sha256sum "$OFFLINE/tickets/erg" | cut -c1-12)
    if [ "$OFFLINE_BEFORE" = "$OFFLINE_AFTER" ]; then
        pass "sync offline exits 0 and leaves binary untouched"
    else
        fail "sync offline changed the binary"
    fi
else
    fail "sync offline should exit 0"
fi

# Test: with no discoverable ticket store, sync refuses rather than pulling
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
OUT=$(cd "$HJ_WORK" && ERG_TICKET_DIR= ./run/erg sync 2>&1 || true)
HJ_AFTER=$(sha256sum "$HJ_WORK/run/erg" | cut -c1-12)
if [ "$HJ_BEFORE" = "$HJ_AFTER" ] && echo "$OUT" | grep -q "no git-erg ticket store"; then
    pass "sync refuses when no ticket store is found (no cwd-repo hijack)"
else
    fail "sync without a store changed the binary or gave no warning: $OUT"
fi

# Test: the binary carries no embedded network/TLS client. `erg sync` delegates
# its explicit transfer to git; every other workflow remains disconnected.
if grep -rEn --include='*.go' 'net/http|crypto/tls' src/go/ >/dev/null 2>&1; then
    fail "source imports net/http or crypto/tls — erg must carry no network code"
else
    pass "no embedded network/TLS client in the binary"
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

# --- post-sync asset-drift hint (ticket 0212) ---
# After the swap, sync re-execs the NEW binary's `erg check`; if a stamped
# asset differs from the new binary's embedded version, it nudges `erg init`.
WORKD="$WORKROOT/work-drift"
git clone -q "$REMOTE" "$WORKD"
cp "$ERG_ABS" "$WORKD/tickets/erg"
# A manifest whose stamps will NOT match the swapped binary's embedded assets.
printf '# erg provenance manifest -- do not edit\nrev: x\ndate: y\nassets:\n  .ergrc sha256:0000000000000000000000000000000000000000000000000000000000000000\n  AGENTS.md sha256:1111111111111111111111111111111111111111111111111111111111111111\n' > "$WORKD/tickets/.erg-assets"
OUTD=$(cd "$WORKD" && ERG_TICKET_DIR="$WORKD/tickets" ./tickets/erg sync 2>&1 || true)
if echo "$OUTD" | grep -q "init to refresh"; then
    pass "post-sync: drift hint fires when a stamped asset differs from the new embedded"
else
    fail "post-sync: expected the erg init drift hint (got: $OUTD)"
fi

# No manifest -> no drift hint. The remote fixture never writes tickets/.ergrc
# or tickets/AGENTS.md, so the clone has no managed asset on disk at all: this
# is the "truly nothing to compare" control, and it stays silent on BOTH hints.
# It is not a control for the stampless hint's gate -- absent is not diverged --
# which is why the fixture below exists.
WORKND="$WORKROOT/work-nodrift"
git clone -q "$REMOTE" "$WORKND"
cp "$ERG_ABS" "$WORKND/tickets/erg"
OUTND=$(cd "$WORKND" && ERG_TICKET_DIR="$WORKND/tickets" ./tickets/erg sync 2>&1 || true)
if echo "$OUTND" | grep -q "init to refresh"; then
    fail "post-sync: drift hint fired without a manifest (should not)"
else
    pass "post-sync: no manifest -> no drift hint"
fi
if echo "$OUTND" | grep -qF "carry no .erg-assets stamp"; then
    fail "post-sync: stampless hint fired with no assets on disk (should not)"
else
    pass "post-sync: no manifest and no assets -> no stampless hint"
fi

# --- post-sync stampless hint (ticket 0283) ---
# No manifest, but an on-disk asset that differs from the swapped-in binary's
# embedded copy. The pre-0283 os.Stat(manifestName) gate skipped the re-exec'd
# check entirely here, so this condition could not be reported however loudly
# `erg check` shouted. This fixture is the only test that reaches that call
# site: it re-execs the real swapped binary and cannot be driven from a Go
# unit test.
WORKSL="$WORKROOT/work-stampless"
git clone -q "$REMOTE" "$WORKSL"
cp "$ERG_ABS" "$WORKSL/tickets/erg"
printf '# an .ergrc that is not what this binary embeds\nlabels = whatever\n' > "$WORKSL/tickets/.ergrc"
# Guard: a stamp here would reroute the run into the drift branch.
if [ -f "$WORKSL/tickets/.erg-assets" ]; then
    fail "post-sync: stampless fixture carries a manifest (test would not exercise 0283)"
else
    OUTSL=$(cd "$WORKSL" && ERG_TICKET_DIR="$WORKSL/tickets" ./tickets/erg sync 2>&1 || true)
    if echo "$OUTSL" | grep -q "erg: synchronized" && echo "$OUTSL" | grep -qF "carry no .erg-assets stamp"; then
        pass "post-sync: stampless hint fires when a diverged asset has no stamp"
    else
        fail "post-sync: expected the stampless provenance hint (got: $OUTSL)"
    fi
    if echo "$OUTSL" | grep -q "init to refresh"; then
        fail "post-sync: no stamp exists, yet the drift hint fired (got: $OUTSL)"
    else
        pass "post-sync: stampless store makes no stamp-relative claim"
    fi
fi

# Manifest up to date -> no drift hint (exit criterion: no hint when assets
# are current). The remote binary differs only by a trailing byte, so a swap
# happens but its embedded assets are unchanged; `erg init` stamps exactly
# those assets, so the re-exec'd check finds matching stamps and stays quiet.
WORKM="$WORKROOT/work-match"
git clone -q "$REMOTE" "$WORKM"
cp "$ERG_ABS" "$WORKM/tickets/erg"
$ERG init "$WORKM" >/dev/null 2>&1
# Guard: this case is distinct from "no manifest -> no hint" only if init
# actually stamped a matching manifest. Without the guard a silently-missing
# manifest would let the test pass via the os.Stat gate, not the matching path.
if ! grep -q "sha256:[0-9a-f]" "$WORKM/tickets/.erg-assets" 2>/dev/null; then
    fail "post-sync: matching fixture: init wrote no stamped manifest (test would be vacuous)"
else
    OUTM=$(cd "$WORKM" && ERG_TICKET_DIR="$WORKM/tickets" ./tickets/erg sync 2>&1 || true)
    if echo "$OUTM" | grep -q "erg: synchronized" && ! echo "$OUTM" | grep -q "init to refresh"; then
        pass "post-sync: up-to-date manifest -> swap happens but no drift hint"
    else
        fail "post-sync: matching manifest should swap without a drift hint (got: $OUTM)"
    fi
fi

# unknown flag rejection (ticket 0185)
out=$($ERG sync --bogus 2>&1) || rc=$?
if [ "${rc:-0}" -ne 0 ] && echo "$out" | grep -q "unknown flag"; then
    pass "unknown flag rejected with usage message"
else
    fail "unknown flag not rejected (rc=${rc:-0}, got: $out)"
fi

echo "sync: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
