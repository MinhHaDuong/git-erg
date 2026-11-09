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

# plain_ticket DIR NNNN-slug [DATE [closed]]: a hand-written ticket, so a
# store without .claims/ can be populated without the binary (which refuses
# there, ticket 0307).
plain_ticket() {
    d=${3:-2026-11-08}
    if [ "${4:-}" = closed ]; then
        mkdir -p "$1/closed"
        printf '%%erg 0.1\nTitle: %s\nCreated: %s\nAuthor: a\nClosed: done\n\n--- log ---\n%sT00:00Z a created\n\n--- body ---\n' "$2" "$d" "$d" > "$1/closed/$2.erg"
    else
        mkdir -p "$1"
        printf '%%erg 0.1\nTitle: %s\nCreated: %s\nAuthor: a\n\n--- log ---\n%sT00:00Z a created\n\n--- body ---\n' "$2" "$d" "$d" > "$1/$2.erg"
    fi
}

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
if [ "$rc" -ne 0 ] && echo "$out" | grep -qF "git ignores tickets/.claims/0001," && ! ls "$IGN"/tickets/*.erg >/dev/null 2>&1; then
    pass "ignored claim path: erg new refuses, names the claim it would write, writes nothing"
else
    fail "ignored claim path: erg new did not refuse (rc=$rc: $out)"
fi
# The same ignore rules make migrate --claims refuse (before .claims/ exists,
# which is where a probe run from inside .claims/ would see nothing) and make
# init leave claims off rather than create a switch git would never carry.
IGM="$TDIR/igm"
git init -q "$IGM"
cp "$IGN/.gitignore" "$IGM/.gitignore"
plain_ticket "$IGM/tickets" 0001-existing
out=$(cd "$IGM" && "$ERG_ABS" migrate --claims tickets 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -qF "git ignores tickets/.claims/.keep," && [ ! -e "$IGM/tickets/.claims" ]; then
    pass "ignored claim path: migrate --claims refuses, names a file it would write, creates nothing"
else
    fail "ignored claim path: migrate --claims did not refuse (rc=$rc: $out)"
fi
IGI="$TDIR/igi"
git init -q "$IGI"
cp "$IGN/.gitignore" "$IGI/.gitignore"
mkdir -p "$IGI/tickets"
cp "$ERG_ABS" "$IGI/tickets/erg"
out=$("$ERG_ABS" init "$IGI" 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 1 ] && [ ! -e "$IGI/tickets/.claims" ] && echo "$out" | grep -qF "not creating tickets/.claims/" &&
    echo "$out" | grep -qF "tickets/.claims/.keep,"; then
    pass "ignored claim path: init exits 1, creates no .claims/, names the .keep it would write"
else
    fail "ignored claim path: init created .claims/, exited 0 or stayed silent (rc=$rc: $out)"
fi

printf '!tickets/.claims/\n!tickets/.claims/*\n' >> "$IGN/.gitignore"
if (cd "$IGN" && "$ERG_ABS" new "allowed" tickets >/dev/null 2>&1); then
    pass "ignored claim path: allowlisting .claims/ lets erg new proceed"
else
    fail "ignored claim path: still refused after allowlisting .claims/"
fi

# --- without .claims/: the store is noncompliant; new and check name the repair ---
OFF="$TDIR/off"
plain_ticket "$OFF" 0001-plain
out=$("$ERG_ABS" new "refused" "$OFF" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && [ ! -e "$OFF/.claims" ] && [ "$(ls "$OFF")" = "0001-plain.erg" ] &&
    echo "$out" | grep -qF "run: erg migrate --claims $OFF"; then
    pass "no .claims/: erg new refuses, writes nothing, names erg migrate --claims"
else
    fail "no .claims/: erg new did not refuse cleanly (rc=$rc: $out)"
fi
out=$("$ERG_ABS" check "$OFF" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -qF "has no .claims/: ID claims are required -- run: erg migrate --claims $OFF"; then
    pass "no .claims/: erg check fails, naming erg migrate --claims"
else
    fail "no .claims/: erg check did not fail with the repair (rc=$rc: $out)"
fi

# --- migrate --claims: populates open and closed, idempotent, refuses shared ---
plain_ticket "$OFF" 0002-second 2026-11-08 closed
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
plain_ticket "$DUP" 0001-one
cp "$DUP/0001-one.erg" "$DUP/0001-other.erg"
out=$("$ERG_ABS" migrate --claims "$DUP" 2>&1) && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && [ ! -e "$DUP/.claims/0001" ] && echo "$out" | grep -qF "0001-other.erg"; then
    pass "migrate --claims: refuses a number two tickets share, names both"
else
    fail "migrate --claims: shared number not refused (rc=$rc: $out)"
fi

# --- migrate --claims prechecks: every blocker in one run, nothing written ---
# Two shared numbers (0002, 0004) and a root rule ignoring every dot path:
# exit-code-only would be tautological (the 0306 binary exits 1 here too), so
# the test reads every blocker off stderr and checks .claims/ was not created.
PRE="$TDIR/pre"
git init -q "$PRE"
printf '.*\n!.gitignore\n' > "$PRE/.gitignore"
plain_ticket "$PRE/tickets" 0001-alone
plain_ticket "$PRE/tickets" 0002-left
plain_ticket "$PRE/tickets" 0002-right
plain_ticket "$PRE/tickets" 0003-other
plain_ticket "$PRE/tickets" 0004-open
plain_ticket "$PRE/tickets" 0004-shut 2026-11-08 closed
out=$(cd "$PRE" && "$ERG_ABS" migrate --claims tickets 2>&1) && rc=0 || rc=$?
missing=""
for want in tickets/0002-left.erg tickets/0002-right.erg tickets/0004-open.erg tickets/closed/0004-shut.erg \
    "tickets/.claims/" "nothing written"; do
    echo "$out" | grep -qF "$want" || missing="$missing [$want]"
done
if [ "$rc" -eq 1 ] && [ -z "$missing" ] && [ ! -e "$PRE/tickets/.claims" ]; then
    pass "migrate --claims precheck: both shared pairs and the ignored path reported in one run; no .claims/ created"
else
    fail "migrate --claims precheck: rc=$rc, missing$missing, .claims exists: $([ -e "$PRE/tickets/.claims" ] && echo yes || echo no) ($out)"
fi
# A claim migrate cannot fix (another ticket's slug) is a blocker too, and a
# partial claim left by an interrupted write names its safe recovery.
FIX="$TDIR/fix"
git init -q "$FIX"
plain_ticket "$FIX/tickets" 0001-mine
plain_ticket "$FIX/tickets" 0002-ours
plain_ticket "$FIX/tickets" 0003-new
mkdir -p "$FIX/tickets/.claims"
echo "theirs 0123abcd" > "$FIX/tickets/.claims/0001"
printf 'ours' > "$FIX/tickets/.claims/0002"
out=$(cd "$FIX" && "$ERG_ABS" migrate --claims tickets 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 1 ] && echo "$out" | grep -qF 'names slug "theirs", not "mine"' &&
    echo "$out" | grep -qF "rm tickets/.claims/0002" && [ ! -e "$FIX/tickets/.claims/0003" ]; then
    pass "migrate --claims precheck: a foreign claim and a partial claim block, with the recovery; nothing written"
else
    fail "migrate --claims precheck: foreign/partial claim not reported, or 0003 written (rc=$rc: $out)"
fi
rm "$FIX/tickets/.claims/0002"
echo "mine 0123abcd" > "$FIX/tickets/.claims/0001"
out=$(cd "$FIX" && "$ERG_ABS" migrate --claims tickets 2>&1) && rc=0 || rc=$?
if [ "$rc" -eq 0 ] && echo "$out" | grep -qF "claims: 2 written, 1 already present" &&
    echo "$out" | grep -qF "erg install --hooks"; then
    pass "migrate --claims: repairs a compliant store's unclaimed tickets; the success line names the hook step"
else
    fail "migrate --claims: repair run (rc=$rc: $out)"
fi

# --- erg init: creates .claims/ in an empty store; refuses a populated one ---
NEWP="$TDIR/newp"
mkdir -p "$NEWP/tickets"
cp "$ERG_ABS" "$NEWP/tickets/erg"
"$ERG_ABS" init "$NEWP" >/dev/null 2>&1 || true
OLDP="$TDIR/oldp"
mkdir -p "$OLDP/tickets"
cp "$ERG_ABS" "$OLDP/tickets/erg"
plain_ticket "$OLDP/tickets" 0001-existing
out=$(cd "$OLDP" && "$ERG_ABS" init 2>&1) && rc=0 || rc=$?
if [ -f "$NEWP/tickets/.claims/.keep" ]; then
    pass "erg init: creates .claims/.keep in an empty store"
else
    fail "erg init: no .claims/.keep in an empty store ($(ls -A "$NEWP/tickets"))"
fi
if [ "$rc" -eq 1 ] && [ ! -e "$OLDP/tickets/.claims" ] && [ -f "$OLDP/tickets/AGENTS.md" ] &&
    echo "$out" | grep -qF "run: erg migrate --claims tickets"; then
    pass "erg init: a populated store without .claims/ gets its assets, then exit 1 naming migrate"
else
    fail "erg init: populated store without .claims/ (rc=$rc, assets: $(ls -A "$OLDP/tickets"): $out)"
fi

# --- two branches migrate one base: the claims are identical, no conflict ---
# A migration claim's nonce is derived from the ticket (number, slug,
# Created), so the same base migrated twice writes the same bytes.
TWO="$TDIR/two"
git init -q "$TWO"
plain_ticket "$TWO/tickets" 0001-first
plain_ticket "$TWO/tickets" 0002-second
plain_ticket "$TWO/tickets" 0003-third 2026-11-07 closed
(cd "$TWO" && git add tickets && git commit -q -m base &&
    git checkout -q -b x && "$ERG_ABS" migrate --claims tickets >/dev/null 2>&1 &&
    git add tickets && git commit -q -m "migrate on x" &&
    git checkout -q -b y main && "$ERG_ABS" migrate --claims tickets >/dev/null 2>&1 &&
    git add tickets && git commit -q -m "migrate on y" && git checkout -q x)
if (cd "$TWO" && git merge -q --no-edit y >/dev/null 2>&1) &&
    [ -z "$(cd "$TWO" && git diff x@{1} HEAD -- tickets/.claims)" ] &&
    [ -f "$TWO/tickets/.claims/0003" ]; then
    pass "two independent migrations of one base merge clean, claims identical"
else
    fail "two independent migrations of one base conflict ($(cd "$TWO" && git status --porcelain | tr '\n' ' '))"
    (cd "$TWO" && git merge --abort 2>/dev/null) || true
fi

echo ""
echo "claims: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
