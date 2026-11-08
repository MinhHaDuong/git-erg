#!/bin/sh
# Integration tests for: ID claims, tickets/.claims/NNNN (ticket 0306)
#
# Two clones of one bare origin take ticket numbers independently, as two
# sessions on two machines would. A claim turns "same number, different
# ticket" into a git add/add conflict on merge and on rebase; the controls
# show different numbers merge clean, and that the silenced merges (-X ours,
# merge=union) are caught by erg check.
set -eu

ERG="${ERG_BIN:-build/erg}"
ERG_ABS=$(CDPATH= cd "$(dirname "$ERG")" && pwd)/$(basename "$ERG")
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  FAIL: $1"; }

echo "=== ID claims ==="

TDIR=$(mktemp -d)
trap 'rm -rf "$TDIR"' EXIT

# Hermetic git: no host hooks, signing or templates leak into the sandbox.
export GIT_CONFIG_NOSYSTEM=1
export HOME="$TDIR/home"
mkdir -p "$HOME"
git config --global user.email "test@example.com"
git config --global user.name "Test"
git config --global commit.gpgsign false
git config --global init.defaultBranch main
git config --global advice.detachedHead false
export ERG_AUTHOR=tester

# created FILE-OF-STDOUT -> the NNNN of the CREATED line
created_id() { sed -n 's/^CREATED \([0-9][0-9][0-9][0-9]\)-.*/\1/p' "$1"; }

# conflicted PATH: is PATH an unmerged add/add ("AA") entry right now?
conflicted() { git status --porcelain -- "$1" | grep -q "^AA "; }

ORIGIN="$TDIR/origin.git"
A="$TDIR/a"
B="$TDIR/b"
git init -q --bare "$ORIGIN"
git clone -q "$ORIGIN" "$A" 2>/dev/null
mkdir -p "$A/tickets/.claims"
: > "$A/tickets/.claims/.keep"
(cd "$A" && "$ERG_ABS" new "seed" tickets >/dev/null 2>&1 &&
    git add tickets && git commit -q -m seed && git push -q origin main 2>/dev/null)
git clone -q "$ORIGIN" "$B" 2>/dev/null

# --- erg new: claim content, stdout unchanged, git add line on stderr ---
cd "$B"
git checkout -q -b b
"$ERG_ABS" new "beta" tickets > "$TDIR/b.out" 2> "$TDIR/b.err"
NB=$(created_id "$TDIR/b.out")
if [ "$(cat "$TDIR/b.out")" = "CREATED $NB-beta.erg" ]; then
    pass "erg new: stdout is the CREATED line alone"
else
    fail "erg new: stdout changed (got: $(cat "$TDIR/b.out"))"
fi
if grep -qE '^beta [0-9a-f]{8}$' "tickets/.claims/$NB" && [ "$(wc -l < "tickets/.claims/$NB")" -eq 1 ]; then
    pass "erg new: claim $NB is one line, slug and 8-hex nonce"
else
    fail "erg new: claim $NB malformed (got: $(cat "tickets/.claims/$NB" 2>&1))"
fi
if grep -qF "git add tickets/$NB-beta.erg tickets/.claims/$NB" "$TDIR/b.err"; then
    pass "erg new: stderr prints the git add line naming both files"
else
    fail "erg new: git add line missing (stderr: $(cat "$TDIR/b.err"))"
fi
git add "tickets/$NB-beta.erg" "tickets/.claims/$NB"
git commit -q -m "beta"

cd "$A"
git checkout -q -b a
"$ERG_ABS" new "alpha" tickets > "$TDIR/a.out" 2>/dev/null
NA=$(created_id "$TDIR/a.out")
git add tickets && git commit -q -m alpha
if [ "$NA" = "$NB" ]; then
    pass "setup: two clones took the same number ($NA) independently"
else
    fail "setup: the clones took $NA and $NB; the collision tests below are vacuous"
fi
git checkout -q main && git merge -q --no-edit a && git push -q origin main 2>/dev/null

# --- positive control: merge and rebase stop on the claim ---
cd "$B"
git fetch -q origin
if git merge --no-edit origin/main >/dev/null 2>&1; then
    fail "merge: same number, different slugs merged clean"
else
    if conflicted "tickets/.claims/$NB"; then
        pass "merge: add/add conflict on tickets/.claims/$NB"
    else
        fail "merge: failed, but not on the claim ($(git status --porcelain | tr '\n' ' '))"
    fi
    git merge --abort
fi
if git rebase origin/main >/dev/null 2>&1; then
    fail "rebase: same number, different slugs rebased clean"
else
    if conflicted "tickets/.claims/$NB"; then
        pass "rebase: add/add conflict on tickets/.claims/$NB"
    else
        fail "rebase: failed, but not on the claim ($(git status --porcelain | tr '\n' ' '))"
    fi
    git rebase --abort
fi

# --- silenced conflicts are caught by erg check ---
git checkout -q -b b-ours b
if git merge -q --no-edit -X ours origin/main >/dev/null 2>&1; then
    out=$("$ERG_ABS" check tickets 2>&1) && rc=0 || rc=$?
    if [ "$rc" -ne 0 ] && echo "$out" | grep -qF "names slug \"beta\", not \"alpha\""; then
        pass "-X ours: two tickets under one claim fail erg check on the slug mismatch"
    else
        fail "-X ours: erg check did not report the slug mismatch (rc=$rc: $out)"
    fi
else
    fail "-X ours: the merge did not go through, so the silencing is untested"
    git merge --abort 2>/dev/null || true
fi

git checkout -q -b b-union b
echo 'tickets/.claims/* merge=union' > .git/info/attributes
if git merge -q --no-edit origin/main >/dev/null 2>&1; then
    out=$("$ERG_ABS" check tickets 2>&1) && rc=0 || rc=$?
    if [ "$rc" -ne 0 ] && echo "$out" | grep -qF "claim must be exactly one line"; then
        pass "merge=union: a two-line claim fails erg check"
    else
        fail "merge=union: erg check did not report the two-line claim (rc=$rc: $out)"
    fi
else
    fail "merge=union: the merge did not go through, so the silencing is untested"
    git merge --abort 2>/dev/null || true
fi
rm -f .git/info/attributes

# --- negative control: different numbers merge clean ---
# A files two tickets on branch c and publishes the branch, so B's scan of
# origin/c moves past both: the numbers differ by construction.
cd "$A"
git checkout -q -b c main
"$ERG_ABS" new "c one" tickets > "$TDIR/c.out" 2>/dev/null
"$ERG_ABS" new "c two" tickets >/dev/null 2>&1
git add tickets && git commit -q -m "c" && git push -q origin c 2>/dev/null
cd "$B"
git fetch -q origin
git checkout -q -b d origin/main
"$ERG_ABS" new "delta" tickets > "$TDIR/d.out" 2>/dev/null
ND=$(created_id "$TDIR/d.out")
NC=$(created_id "$TDIR/c.out")
git add tickets && git commit -q -m "delta"
if [ "$ND" != "$NC" ] && git merge -q --no-edit origin/c >/dev/null 2>&1; then
    pass "negative control: claims $NC and $ND (different numbers) merge clean"
else
    fail "negative control: different numbers ($NC, $ND) did not merge clean"
    git merge --abort 2>/dev/null || true
fi
if "$ERG_ABS" check tickets >/dev/null 2>&1; then
    pass "negative control: the merged store passes erg check"
else
    fail "negative control: the merged store fails erg check ($("$ERG_ABS" check tickets 2>&1))"
fi

# --- same number, same slug, one copy closed: still a conflict ---
cd "$A"
git checkout -q main && git merge -q --no-edit c
git push -q origin main 2>/dev/null
cd "$B"
git fetch -q origin
git checkout -q -b f origin/main
# B's local branch d still holds its own number; drop it so both clones see
# the same frontier and take the same number below.
git branch -q -D d
"$ERG_ABS" new "same incident" tickets > "$TDIR/f.out" 2>/dev/null
NF=$(created_id "$TDIR/f.out")
git add tickets && git commit -q -m "f"
cd "$A"
git checkout -q -b e main
"$ERG_ABS" new "same incident" tickets > "$TDIR/e.out" 2>/dev/null
NE=$(created_id "$TDIR/e.out")
"$ERG_ABS" close "$NE" "done" tickets >/dev/null 2>&1
git add -A tickets && git commit -q -m "e"
git checkout -q main && git merge -q --no-edit e && git push -q origin main 2>/dev/null
if [ -f "tickets/closed/$NE-same-incident.erg" ] && [ "$NE" = "$NF" ]; then
    pass "setup: same number $NE and slug, closed on one side, open on the other"
else
    fail "setup: expected the same number (got $NE, $NF) with one copy closed"
fi
cd "$B"
git fetch -q origin
if git merge --no-edit origin/main >/dev/null 2>&1; then
    fail "same slug, one closed: merged clean (the ticket paths differ; only the nonce can conflict)"
else
    if conflicted "tickets/.claims/$NF"; then
        pass "same slug, one closed: add/add conflict on tickets/.claims/$NF"
    else
        fail "same slug, one closed: failed, but not on the claim"
    fi
    git merge --abort
fi

# --- pre-commit hook: a ticket staged without its claim is rejected ---
git checkout -q -b g origin/main
git config core.hooksPath "$B/.git/hooks"
cp "$ERG_ABS" tickets/erg
echo "tickets/erg" >> .git/info/exclude
"$ERG_ABS" install . --hooks >/dev/null 2>&1
"$ERG_ABS" new "hooked" tickets > "$TDIR/g.out" 2>/dev/null
NG=$(created_id "$TDIR/g.out")
git add "tickets/$NG-hooked.erg"
msg=$(git commit -m "hooked" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$msg" | grep -qF "has no staged claim tickets/.claims/$NG"; then
    pass "hook: a ticket staged without its claim is rejected"
else
    fail "hook: ticket without staged claim not rejected (rc=$rc: $msg)"
fi
git add "tickets/.claims/$NG"
if git commit -q -m "hooked" >/dev/null 2>&1; then
    pass "hook: the same commit passes once the claim is staged"
else
    fail "hook: commit with ticket and claim staged was rejected"
fi

# --- close and rm leave the claim; its number is never reused ---
"$ERG_ABS" new "to remove" tickets > "$TDIR/r.out" 2>/dev/null
NR=$(created_id "$TDIR/r.out")
"$ERG_ABS" close "$NG" "done" tickets >/dev/null 2>&1
"$ERG_ABS" rm "$NR" tickets >/dev/null 2>&1
if [ -f "tickets/.claims/$NG" ] && [ -f "tickets/.claims/$NR" ] && [ ! -e "tickets/$NR-to-remove.erg" ]; then
    pass "close and rm leave the claims in place"
else
    fail "close or rm removed a claim (or rm did not remove the ticket)"
fi
if "$ERG_ABS" check tickets >/dev/null 2>&1; then
    pass "a claim without a ticket passes erg check"
else
    fail "a claim without a ticket fails erg check: $("$ERG_ABS" check tickets 2>&1)"
fi
NEXT=$("$ERG_ABS" next-id tickets 2>/dev/null)
if [ "$NEXT" -gt "$NR" ]; then
    pass "next-id ($NEXT) allocates above the removed ticket's claim ($NR)"
else
    fail "next-id ($NEXT) would reuse the removed ticket's number ($NR)"
fi

# --- a ticket without a claim is rejected, with the plain-file fix ---
sed "s/^Title: .*/Title: hand made/" "tickets/closed/$NG-hooked.erg" | grep -v '^Closed:' > "tickets/0999-hand-made.erg"
out=$("$ERG_ABS" check tickets 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -qF 'echo "hand-made $(date +%s)" >'; then
    pass "claimless ticket: erg check fails and prints the plain-file fix"
else
    fail "claimless ticket: no failure or no fix (rc=$rc: $out)"
fi
echo "hand-made $(date +%s)" > tickets/.claims/0999
if "$ERG_ABS" check tickets >/dev/null 2>&1; then
    pass "claimless ticket: the plain-file fix makes erg check pass, no binary needed"
else
    fail "claimless ticket: the plain-file fix did not satisfy erg check"
fi

# --- an ignored claim path is refused ---
IGN="$TDIR/ign"
git init -q "$IGN"
mkdir -p "$IGN/tickets/.claims"
printf '*\n!tickets/\n!tickets/*.erg\n' > "$IGN/.gitignore"
out=$(cd "$IGN" && "$ERG_ABS" new "ignored" tickets 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -qF "git ignores" && ! ls "$IGN"/tickets/*.erg >/dev/null 2>&1; then
    pass "ignored claim path: erg new refuses and writes nothing"
else
    fail "ignored claim path: erg new did not refuse (rc=$rc: $out)"
fi
# The same ignore rules make migrate --claims refuse (before .claims/ exists,
# which is where a probe run from inside .claims/ would see nothing) and make
# init leave claims off rather than create a switch git would never carry.
IGM="$TDIR/igm"
git init -q "$IGM"
cp "$IGN/.gitignore" "$IGM/.gitignore"
mkdir -p "$IGM/tickets"
"$ERG_ABS" new "existing" "$IGM/tickets" >/dev/null 2>&1
out=$(cd "$IGM" && "$ERG_ABS" migrate --claims tickets 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -qF "git ignores" && [ ! -e "$IGM/tickets/.claims" ]; then
    pass "ignored claim path: migrate --claims refuses and creates nothing"
else
    fail "ignored claim path: migrate --claims did not refuse (rc=$rc: $out)"
fi
IGI="$TDIR/igi"
git init -q "$IGI"
cp "$IGN/.gitignore" "$IGI/.gitignore"
mkdir -p "$IGI/tickets"
cp "$ERG_ABS" "$IGI/tickets/erg"
out=$("$ERG_ABS" init "$IGI" 2>&1) || true
if [ ! -e "$IGI/tickets/.claims" ] && echo "$out" | grep -qF "not creating tickets/.claims/"; then
    pass "ignored claim path: init leaves claims off and says why"
else
    fail "ignored claim path: init created .claims/ or stayed silent ($out)"
fi

printf '!tickets/.claims/\n!tickets/.claims/*\n' >> "$IGN/.gitignore"
if (cd "$IGN" && "$ERG_ABS" new "allowed" tickets >/dev/null 2>&1); then
    pass "ignored claim path: allowlisting .claims/ lets erg new proceed"
else
    fail "ignored claim path: still refused after allowlisting .claims/"
fi

# --- without .claims/: no claim, no requirement, a hint ---
OFF="$TDIR/off"
mkdir -p "$OFF"
out=$("$ERG_ABS" new "plain" "$OFF" 2>&1 >/dev/null)
if [ ! -e "$OFF/.claims" ] && echo "$out" | grep -qF "erg migrate --claims"; then
    pass "claims off: erg new writes no claim and hints at the migration"
else
    fail "claims off: unexpected claim or missing hint ($out)"
fi
if "$ERG_ABS" check "$OFF" >/dev/null 2>&1; then
    pass "claims off: erg check requires no claim"
else
    fail "claims off: erg check failed on a claimless store"
fi

# --- migrate --claims: populates open and closed, idempotent, refuses shared ---
mkdir -p "$OFF/closed"
"$ERG_ABS" new "second" "$OFF" >/dev/null 2>&1
"$ERG_ABS" close 0002 "done" "$OFF" >/dev/null 2>&1
"$ERG_ABS" migrate --claims "$OFF" >/dev/null 2>&1 && rc=0 || rc=$?
if [ "$rc" -eq 0 ] && grep -q '^plain ' "$OFF/.claims/0001" && grep -q '^second ' "$OFF/.claims/0002" &&
    "$ERG_ABS" check "$OFF" >/dev/null 2>&1; then
    pass "migrate --claims: claims open and closed tickets; the store passes check"
else
    fail "migrate --claims: claims missing or store fails check (rc=$rc)"
fi
snap() { (cd "$OFF/.claims" && ls -A | sort | while read -r f; do printf '%s:' "$f"; cat "$f"; done) | cksum; }
S1=$(snap)
"$ERG_ABS" migrate --claims "$OFF" >/dev/null 2>&1
if [ "$S1" = "$(snap)" ]; then
    pass "migrate --claims: a second run is byte-identical"
else
    fail "migrate --claims: a second run changed .claims/"
fi
DUP="$TDIR/dup"
mkdir -p "$DUP"
"$ERG_ABS" new "one" "$DUP" >/dev/null 2>&1
cp "$DUP/0001-one.erg" "$DUP/0001-other.erg"
out=$("$ERG_ABS" migrate --claims "$DUP" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && [ ! -e "$DUP/.claims/0001" ] && echo "$out" | grep -qF "0001-other.erg"; then
    pass "migrate --claims: refuses a number two tickets share, names both"
else
    fail "migrate --claims: shared number not refused (rc=$rc: $out)"
fi

# --- erg init switches claims on only in an empty store ---
NEWP="$TDIR/newp"
mkdir -p "$NEWP/tickets"
cp "$ERG_ABS" "$NEWP/tickets/erg"
"$ERG_ABS" init "$NEWP" >/dev/null 2>&1 || true
OLDP="$TDIR/oldp"
mkdir -p "$OLDP/tickets"
cp "$ERG_ABS" "$OLDP/tickets/erg"
"$ERG_ABS" new "existing" "$OLDP/tickets" >/dev/null 2>&1
"$ERG_ABS" init "$OLDP" >/dev/null 2>&1 || true
if [ -f "$NEWP/tickets/.claims/.keep" ] && [ ! -e "$OLDP/tickets/.claims" ]; then
    pass "erg init: creates .claims/.keep in an empty store, not in one with tickets"
else
    fail "erg init: .claims/ creation wrong (new: $(ls -A "$NEWP/tickets"), old: $(ls -A "$OLDP/tickets"))"
fi

echo ""
echo "claims: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
