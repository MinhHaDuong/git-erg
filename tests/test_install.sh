#!/bin/sh
# Integration tests for: erg install (hooks + inject-agents wiring, ticket 0208)
set -eu

ERG="${ERG_BIN:-build/erg}"
ERG_ABS=$(CDPATH= cd "$(dirname "$ERG")" && pwd)/$(basename "$ERG")
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== erg install ==="

TDIR=$(mktemp -d)
trap 'rm -rf "$TDIR"' EXIT

# new_repo <name>: make a fresh git repo with a real (executable) tickets/erg.
# Echoes the repo path.
new_repo() {
    r="$TDIR/$1"
    mkdir -p "$r/tickets"
    cp "$ERG_ABS" "$r/tickets/erg"
    (cd "$r" && git init -q -b main >/dev/null 2>&1)
    echo "$r"
}

# --- install exists as a subcommand ---
if $ERG install --help >/dev/null 2>&1; then
    pass "install subcommand exists"
else
    fail "install subcommand does not exist"
fi

# --- install help starts with ## erg install ---
help_out=$($ERG install --help 2>/dev/null)
if echo "$help_out" | grep -q "^## erg install"; then
    pass "install help starts with '## erg install'"
else
    fail "install help header (got: $(echo "$help_out" | head -1))"
fi

# --- install with no flags is a no-op (no mutation outside tickets/) ---
REPO=$(new_repo norepoflags)
$ERG install "$REPO" >/dev/null 2>&1
if [ ! -f "$REPO/.git/hooks/pre-commit" ] && [ ! -f "$REPO/AGENTS.md" ]; then
    pass "install with no flags: no mutation outside tickets/"
else
    fail "install with no flags: created files outside tickets/"
fi

# --- unknown flag rejected ---
out=$($ERG install --bogus 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "unknown flag"; then
    pass "unknown flag rejected with message"
else
    fail "unknown flag not rejected (rc=$rc, got: $out)"
fi

# --- missing binary refused ---
NOBIN="$TDIR/nobin"; mkdir -p "$NOBIN/tickets"; (cd "$NOBIN" && git init -q -b main)
out=$($ERG install "$NOBIN" --hooks 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "binary not found"; then
    pass "install without tickets/erg refused"
else
    fail "install without binary should be refused (rc=$rc, out: $out)"
fi

# --- --hooks: fresh install creates an executable managed hook ---
REPO=$(new_repo hooksfresh)
out=$($ERG install "$REPO" --hooks 2>&1) && rc=0 || rc=$?
HOOK="$REPO/.git/hooks/pre-commit"
if [ "$rc" -eq 0 ] && [ -f "$HOOK" ]; then
    pass "--hooks: creates pre-commit"
else
    fail "--hooks: pre-commit not created (rc=$rc, out: $out)"
fi
if [ -x "$HOOK" ]; then
    pass "--hooks: hook is executable"
else
    fail "--hooks: hook not executable"
fi
if grep -q "tickets/erg validate" "$HOOK" && grep -q "tickets/erg check" "$HOOK"; then
    pass "--hooks: hook runs validate + check"
else
    fail "--hooks: hook missing validate/check"
fi
if grep -q "tickets/erg" "$HOOK" && grep -q 'branch=' "$HOOK"; then
    pass "--hooks: hook rejects tickets/erg outside main"
else
    fail "--hooks: hook missing binary-reject logic"
fi
if grep -q "erg archive" "$HOOK"; then
    fail "--hooks: hook must NOT run erg archive (blocker #2)"
else
    pass "--hooks: hook does not run erg archive (blocker #2)"
fi

# --- --hooks: rerun is idempotent (single block) ---
$ERG install "$REPO" --hooks >/dev/null 2>&1
n=$(grep -c '>>> erg managed >>>' "$HOOK" || true)
if [ "$n" -eq 1 ]; then
    pass "--hooks: rerun idempotent (one managed block)"
else
    fail "--hooks: rerun produced $n managed blocks (expected 1)"
fi

# --- --hooks: rerun is BYTE-stable (no blank-line accumulation) ---
REPO=$(new_repo hooksbytes)
printf '#!/bin/sh\necho THIRD\nexit 0\n' > "$REPO/.git/hooks/pre-commit"
chmod +x "$REPO/.git/hooks/pre-commit"
$ERG install "$REPO" --hooks >/dev/null 2>&1
after1=$(cat "$REPO/.git/hooks/pre-commit")
$ERG install "$REPO" --hooks >/dev/null 2>&1
after2=$(cat "$REPO/.git/hooks/pre-commit")
if [ "$after1" = "$after2" ]; then
    pass "--hooks: rerun is byte-stable (no separator growth)"
else
    fail "--hooks: rerun mutated the file (blank-line accumulation)"
fi

# --- --hooks: prepend runs BEFORE a third-party 'exit 0' (real commit) ---
REPO=$(new_repo prepend)
printf '#!/bin/sh\necho THIRD\nexit 0\n' > "$REPO/.git/hooks/pre-commit"
chmod 0644 "$REPO/.git/hooks/pre-commit"   # also exercises the 0644->exec fix
$ERG install "$REPO" --hooks >/dev/null 2>&1
if [ -x "$REPO/.git/hooks/pre-commit" ]; then
    pass "--hooks: pre-existing 0644 hook is made executable"
else
    fail "--hooks: pre-existing hook left non-executable (git would skip it)"
fi
# Stage an invalid .erg; our validate must reject despite the trailing exit 0.
printf 'not valid\n' > "$REPO/tickets/9001-bad.erg"
( cd "$REPO" && git add tickets/9001-bad.erg && git commit -q -m x ) >/dev/null 2>&1 && crc=0 || crc=$?
if [ "$crc" -ne 0 ]; then
    pass "--hooks: managed checks run before third-party 'exit 0' (commit blocked)"
else
    fail "--hooks: commit succeeded -- our checks were skipped by third-party exit 0"
fi
if grep -q "THIRD" "$REPO/.git/hooks/pre-commit"; then
    pass "--hooks: third-party content preserved"
else
    fail "--hooks: third-party content lost"
fi

# --- --hooks: legacy marker upgrade collapses to one new-format block ---
REPO=$(new_repo legacy)
cat > "$REPO/.git/hooks/pre-commit" <<'EOF'
#!/bin/sh
echo before
# --- git-erg: begin managed block ---
echo OLD ERG BODY
# --- git-erg: end managed block ---
echo after
EOF
chmod +x "$REPO/.git/hooks/pre-commit"
$ERG install "$REPO" --hooks >/dev/null 2>&1
HOOK="$REPO/.git/hooks/pre-commit"
nnew=$(grep -c '>>> erg managed >>>' "$HOOK" || true)
nleg=$(grep -c 'git-erg: begin managed' "$HOOK" || true)
nold=$(grep -c 'OLD ERG BODY' "$HOOK" || true)
nthird=$(grep -cE '^echo (before|after)' "$HOOK" || true)
if [ "$nnew" -eq 1 ] && [ "$nleg" -eq 0 ] && [ "$nold" -eq 0 ] && [ "$nthird" -eq 2 ]; then
    pass "--hooks: legacy block upgraded in place (1 new, 0 legacy, third-party intact)"
else
    fail "--hooks: legacy upgrade wrong (new=$nnew legacy=$nleg old=$nold third=$nthird)"
fi

# --- --hooks: unbalanced markers refuse and change nothing ---
REPO=$(new_repo unbalanced)
printf '#!/bin/sh\n# >>> erg managed >>>\necho dangling\necho keep\n' > "$REPO/.git/hooks/pre-commit"
chmod +x "$REPO/.git/hooks/pre-commit"
before=$(cat "$REPO/.git/hooks/pre-commit")
out=$($ERG install "$REPO" --hooks 2>&1) && rc=0 || rc=$?
after=$(cat "$REPO/.git/hooks/pre-commit")
if [ "$rc" -ne 0 ] && [ "$before" = "$after" ]; then
    pass "--hooks: unbalanced markers refused, file unchanged"
else
    fail "--hooks: unbalanced markers not safely handled (rc=$rc, changed=$([ "$before" = "$after" ] && echo no || echo yes))"
fi

# --- --hooks: not a git repository is refused ---
NOGIT="$TDIR/nogit"; mkdir -p "$NOGIT/tickets"; cp "$ERG_ABS" "$NOGIT/tickets/erg"
out=$($ERG install "$NOGIT" --hooks 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "not a git repository"; then
    pass "--hooks: refuses outside a git repository"
else
    fail "--hooks: should refuse without a git repo (rc=$rc, out: $out)"
fi

# --- --hooks: works in a linked worktree (hook lands in common .git/hooks) ---
REPO=$(new_repo wtmain)
( cd "$REPO" && git add tickets/erg && git -c user.email=t@t -c user.name=t commit -q -m init ) >/dev/null 2>&1
WT="$TDIR/wt-linked"
( cd "$REPO" && git worktree add -q "$WT" -b wtbranch ) >/dev/null 2>&1
if [ -d "$WT/tickets" ] || mkdir -p "$WT/tickets"; then :; fi
cp "$ERG_ABS" "$WT/tickets/erg"
out=$($ERG install "$WT" --hooks 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 0 ] && [ -f "$REPO/.git/hooks/pre-commit" ]; then
    pass "--hooks: worktree install lands in the common .git/hooks"
else
    fail "--hooks: worktree hook not in common dir (rc=$rc, out: $out)"
fi

# --- --inject-agents: absent AGENTS.md without consent is refused ---
REPO=$(new_repo noagents)
out=$($ERG install "$REPO" --inject-agents 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && [ ! -f "$REPO/AGENTS.md" ]; then
    pass "--inject-agents: absent AGENTS.md refused, nothing created"
else
    fail "--inject-agents: should refuse on absent AGENTS.md (rc=$rc)"
fi

# --- pre-flight: a refused --inject-agents writes NEITHER file ---
REPO=$(new_repo preflight)
$ERG install "$REPO" --hooks --inject-agents >/dev/null 2>&1 && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && [ ! -f "$REPO/.git/hooks/pre-commit" ] && [ ! -f "$REPO/AGENTS.md" ]; then
    pass "pre-flight: a failed precondition writes nothing (hook NOT created)"
else
    fail "pre-flight: partial mutation occurred (rc=$rc, hook=$([ -f "$REPO/.git/hooks/pre-commit" ] && echo yes || echo no))"
fi

# --- --create-agents-md: creates the file with the pointer ---
REPO=$(new_repo createagents)
out=$($ERG install "$REPO" --inject-agents --create-agents-md 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 0 ] && grep -q "tickets/AGENTS.md" "$REPO/AGENTS.md"; then
    pass "--inject-agents --create-agents-md: creates AGENTS.md with pointer"
else
    fail "--create-agents-md: did not create with pointer (rc=$rc)"
fi
# idempotent rerun
$ERG install "$REPO" --inject-agents >/dev/null 2>&1
n=$(grep -c '>>> erg managed >>>' "$REPO/AGENTS.md" || true)
if [ "$n" -eq 1 ]; then
    pass "--inject-agents: rerun idempotent (one managed block)"
else
    fail "--inject-agents: rerun produced $n blocks (expected 1)"
fi

# --- --inject-agents: rerun is BYTE-stable (no blank-line accumulation) ---
REPO=$(new_repo agentsbytes)
printf '# P\n\nrules\n' > "$REPO/AGENTS.md"
$ERG install "$REPO" --inject-agents >/dev/null 2>&1
a1=$(cat "$REPO/AGENTS.md")
$ERG install "$REPO" --inject-agents >/dev/null 2>&1
a2=$(cat "$REPO/AGENTS.md")
if [ "$a1" = "$a2" ]; then
    pass "--inject-agents: rerun is byte-stable (no separator growth)"
else
    fail "--inject-agents: rerun mutated the file (blank-line accumulation)"
fi

# --- --inject-agents into existing AGENTS.md preserves third-party content ---
REPO=$(new_repo existingagents)
printf '# My Project\n\nExisting agent rules.\n' > "$REPO/AGENTS.md"
$ERG install "$REPO" --inject-agents >/dev/null 2>&1
if grep -q "Existing agent rules" "$REPO/AGENTS.md" && grep -q "tickets/AGENTS.md" "$REPO/AGENTS.md"; then
    pass "--inject-agents: existing AGENTS.md content preserved + pointer added"
else
    fail "--inject-agents: existing content not preserved"
fi

# --- both flags together succeed when AGENTS.md exists ---
REPO=$(new_repo bothok)
printf '# P\n' > "$REPO/AGENTS.md"
out=$($ERG install "$REPO" --hooks --inject-agents 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 0 ] && [ -f "$REPO/.git/hooks/pre-commit" ] && grep -q "tickets/AGENTS.md" "$REPO/AGENTS.md"; then
    pass "both flags: hook + agents both wired (exit 0)"
else
    fail "both flags: not both wired (rc=$rc)"
fi

# --- pre-commit managed block must NEVER run archive (blocker #2) ---
# A pre-commit os.Rename would record a deletion without the matching add.
REPO=$(new_repo noarchive)
$ERG install "$REPO" --hooks >/dev/null 2>&1
if grep -q "archive" "$REPO/.git/hooks/pre-commit"; then
    fail "pre-commit hook contains 'archive' (blocker #2: archive must be pre-push only)"
else
    pass "pre-commit hook never runs archive (blocker #2)"
fi

# --- --push-hook installs a warn-only, non-blocking pre-push hook (0209) ---
REPO=$(new_repo pushhook)
out=$($ERG install "$REPO" --push-hook 2>&1) && rc=0 || rc=$?
PUSH="$REPO/.git/hooks/pre-push"
if [ "$rc" -eq 0 ] && [ -x "$PUSH" ]; then
    pass "--push-hook: creates an executable pre-push hook"
else
    fail "--push-hook: pre-push not created (rc=$rc, out: $out)"
fi
# It must not be installed by --hooks alone (opt-in separation).
REPO2=$(new_repo hooksonly)
$ERG install "$REPO2" --hooks >/dev/null 2>&1
if [ ! -f "$REPO2/.git/hooks/pre-push" ]; then
    pass "--hooks alone does NOT install the pre-push hook (opt-in separation)"
else
    fail "--hooks alone installed a pre-push hook (scope creep)"
fi

# --- pre-push hook: warn-only, never blocks, mutates nothing ---
REPO=$(new_repo pushwarn)
( cd "$REPO" && git config user.email t@t && git config user.name t )
$ERG install "$REPO" --push-hook >/dev/null 2>&1
# A closed-but-unarchived ticket, committed.
cat > "$REPO/tickets/9002-closed.erg" <<'EOF'
%erg 0.1
Title: Closed
Created: 2026-06-02
Author: t
Closed: 2026-06-02

--- log ---
--- body ---
EOF
( cd "$REPO" && git add -A && git commit -q -m "closed ticket" )
out=$( cd "$REPO" && sh .git/hooks/pre-push origin /dev/null </dev/null 2>&1 ) && rc=0 || rc=$?
if [ "$rc" -eq 0 ]; then
    pass "pre-push hook exits 0 (never blocks the push)"
else
    fail "pre-push hook blocked the push (rc=$rc)"
fi
if echo "$out" | grep -q "9002-closed.erg"; then
    pass "pre-push hook warns about the closed-but-unarchived ticket"
else
    fail "pre-push hook did not warn (out: $out)"
fi
dirty=$( cd "$REPO" && git status --porcelain | wc -l )
if [ "$dirty" -eq 0 ]; then
    pass "pre-push hook mutates nothing (clean tree)"
else
    fail "pre-push hook left the tree dirty ($dirty changes)"
fi

# --- pre-push hook: prepended block does NOT shadow third-party content ---
# The managed block is inserted before any pre-existing hook content; it must
# not contain a trailing 'exit 0' that would terminate before that content runs
# (the same hazard the pre-commit block deliberately avoids).
REPO=$(new_repo pushprepend)
( cd "$REPO" && git config user.email t@t && git config user.name t )
printf '#!/bin/sh\necho "THIRD-PARTY PRE-PUSH RAN"\n' > "$REPO/.git/hooks/pre-push"
chmod +x "$REPO/.git/hooks/pre-push"
$ERG install "$REPO" --push-hook >/dev/null 2>&1
out=$( cd "$REPO" && sh .git/hooks/pre-push origin /dev/null </dev/null 2>&1 ) && rc=0 || rc=$?
if echo "$out" | grep -q "THIRD-PARTY PRE-PUSH RAN"; then
    pass "--push-hook: third-party pre-push content still runs (not shadowed by exit 0)"
else
    fail "--push-hook: managed block shadowed third-party content (out: $out)"
fi
if grep -q "THIRD-PARTY PRE-PUSH RAN" "$REPO/.git/hooks/pre-push"; then
    pass "--push-hook: third-party pre-push content preserved on disk"
else
    fail "--push-hook: third-party pre-push content lost"
fi

# --- pre-push hook: silent + exit 0 when binary absent and nothing pending ---
REPO=$(new_repo pushquiet)
( cd "$REPO" && git config user.email t@t && git config user.name t )
$ERG install "$REPO" --push-hook >/dev/null 2>&1
rm -f "$REPO/tickets/erg"   # binary gitignored / absent
out=$( cd "$REPO" && sh .git/hooks/pre-push origin /dev/null </dev/null 2>&1 ) && rc=0 || rc=$?
if [ "$rc" -eq 0 ]; then
    pass "pre-push hook exits 0 even when tickets/erg is absent"
else
    fail "pre-push hook failed with binary absent (rc=$rc, out: $out)"
fi

# --- erg-github (a text script) is committable on a feature branch (Part E) ---
# The binary-reject rule anchors ^tickets/erg$ exactly; erg-github must pass.
REPO=$(new_repo ergghubcommit)
( cd "$REPO" && git config user.email t@t && git config user.name t )
cp "$ERG_ABS" "$REPO/tickets/erg"   # ensure present for hook
$ERG install "$REPO" --hooks >/dev/null 2>&1
cp "$(CDPATH= cd "$(dirname "$0")/.." && pwd)/tickets/erg-github" "$REPO/tickets/erg-github"
( cd "$REPO" && git switch -q -c feature && git add tickets/erg-github ) >/dev/null 2>&1
out=$( cd "$REPO" && git commit -q -m "add erg-github" 2>&1 ) && rc=0 || rc=$?
if [ "$rc" -eq 0 ]; then
    pass "committing tickets/erg-github on a feature branch is NOT rejected"
else
    fail "tickets/erg-github commit was wrongly rejected (rc=$rc, out: $out)"
fi

# --- master-default repo: tickets/erg commit allowed on master (ticket 0219) ---
# Simulate a repo whose origin/HEAD resolves to master using git symbolic-ref.
# The hook must allow tickets/erg on master and reject it on a feature branch.
REPO=$(new_repo masterdefault)
( cd "$REPO" && git config user.email t@t && git config user.name t )
# Set origin/HEAD to point at master (no real remote needed; the hook only reads
# refs/remotes/origin/HEAD via git symbolic-ref --short).
git -C "$REPO" symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/master
# Install the hook.
$ERG install "$REPO" --hooks >/dev/null 2>&1
# Create and switch to master branch.
git -C "$REPO" checkout -q -b master 2>/dev/null || true
# Stage tickets/erg on master -> commit must SUCCEED.
( cd "$REPO" && git add tickets/erg && git commit -q -m "erg refresh on master" ) && rc=0 || rc=$?
if [ "$rc" -eq 0 ]; then
    pass "master-default repo: tickets/erg commit allowed on master"
else
    fail "master-default repo: tickets/erg commit wrongly rejected on master (rc=$rc)"
fi

# --- master-default repo: tickets/erg commit rejected on feature branch ---
# Reuse the same master-default repo; switch to a feature branch.
REPO2=$(new_repo masterdefault_feature)
( cd "$REPO2" && git config user.email t@t && git config user.name t )
git -C "$REPO2" symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/master
$ERG install "$REPO2" --hooks >/dev/null 2>&1
# Stay on the initial 'main' branch (which is NOT the default 'master').
( cd "$REPO2" && git add tickets/erg && git commit -q -m "erg on non-default branch" ) && rc=0 || rc=$?
if [ "$rc" -ne 0 ]; then
    pass "master-default repo: tickets/erg commit rejected on non-default branch (main)"
else
    fail "master-default repo: tickets/erg commit was NOT rejected on non-default branch (expected rejection)"
fi

echo ""
echo "install: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
