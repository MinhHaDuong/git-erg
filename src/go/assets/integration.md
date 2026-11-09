# Integration

After running `erg init`, follow these two steps to integrate git-erg
with your project. The automated path is `erg install` (see below);
the manual steps that follow describe exactly what it writes.

`erg init` also writes `tickets/.erg-assets`, a provenance manifest (the
binary's rev/date and the SHA-256 of each embedded asset). Commit it -- it
is lightweight durable state that lets erg tell a clean asset upgrade from a
local edit. It is not a `.erg` ticket, so `erg check` ignores it.

## 1. Pre-commit hook

The hook prevents committing `tickets/erg` on feature branches -- CI
rebuilds the binary after merge to main -- validates staged tickets, and
rejects a commit whose index holds a ticket without its ID claim (the index,
not the working tree: an untracked claim on disk would let `erg check` pass
while the commit lacks it). See the `.gitignore` section below for the full
commit policy.

Install the hooks after the store is migrated and that commit is made
(`erg migrate --claims`, see "ID claims" below): the claim check rejects
every commit in a store whose tickets are not claimed yet.

**Automated:** run `erg install --hooks`. It inserts the block below into
`.git/hooks/pre-commit` between sentinel markers, right after the shebang
so it runs before any other hook content, makes the file executable, and
on rerun replaces only the marked region (your other hook content is left
untouched). It honours linked worktrees and `core.hooksPath`.

**Manual:** create `.git/hooks/pre-commit` (and `chmod +x` it), then paste
the marked block below. Keep the markers verbatim so a later
`erg install --hooks` recognises and upgrades the block in place. Put any
custom hook logic OUTSIDE the markers -- erg overwrites the inside on
upgrade.

```sh
# >>> erg managed >>>
# Reject tickets/erg commit on non-default branches.
# CI rebuilds the binary after merge; feature PRs must not include it.
if git diff --cached --name-only | grep -q '^tickets/erg$'; then
    default_branch=$(git symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null | sed 's|^origin/||')
    default_branch=${default_branch:-main}
    branch=$(git branch --show-current)
    if [ "$branch" != "$default_branch" ]; then
        echo "pre-commit: do not commit tickets/erg in feature branches." >&2
        echo " CI rebuilds the binary after merge. To override: git commit --no-verify" >&2
        exit 1
    fi
fi

# Validate staged .erg files and the corpus.
erg_files=$(git diff --cached --name-only | grep '\.erg$' || true)
if [ -n "$erg_files" ]; then
    if [ -x tickets/erg ]; then
        # shellcheck disable=SC2086
        if ! tickets/erg validate $erg_files; then
            echo "ERROR: ticket validation failed." >&2
            exit 1
        fi
        if ! tickets/erg check tickets/; then
            echo "ERROR: ticket corpus check failed." >&2
            exit 1
        fi
    else
        echo "ERROR: tickets/erg not found. Build it from the git-erg source" >&2
        echo "  (make build there), or 'git pull' if your project vendors the committed binary." >&2
        exit 1
    fi
fi

erg_hook=pre-commit
# Every ticket in the index has its ID claim in the index. The index, not
# the working tree: an untracked claim on disk lets erg check pass while
# the commit lacks it.
erg_unclaimed=$(git -c core.quotePath=false ls-files -- tickets | awk '
    /^tickets\/\.claims\/[0-9][0-9][0-9][0-9]$/ { claimed[substr($0, length($0) - 3)] = 1; next }
    /^tickets\/(.*\/)?[0-9][0-9][0-9][0-9]-[^\/]*\.erg$/ {
        n = split($0, part, "/")
        ticket[$0] = substr(part[n], 1, 4)
    }
    END { for (t in ticket) if (!(ticket[t] in claimed)) print t }' | sort)
if [ -n "$erg_unclaimed" ]; then
    echo "$erg_hook: tickets without a claim in the index:" >&2
    echo "$erg_unclaimed" | sed 's/^/  /' >&2
    echo " Stage each claim: git add tickets/.claims/NNNN (none yet? run: tickets/erg migrate --claims tickets)" >&2
    exit 1
fi
# <<< erg managed <<<
```

### Pre-merge-commit hook

A clean merge runs `pre-merge-commit`, not `pre-commit`, so `erg install
--hooks` installs this block too, in `.git/hooks/pre-merge-commit`, with the
same markers and the same in-place upgrade. When the merge changes
`tickets/`, it runs `erg check` on the merge result, then the same index
check: a merge that brings in an unclaimed ticket or a duplicate number is
refused before git records it, and `MERGE_HEAD` stays for you to fix and
conclude with `git commit` (whose pre-commit check runs again) or to
`git merge --abort`. It runs this clone's committed `tickets/erg` (the blob
at `HEAD`, copied into the git directory), never the working tree's, which a
merge that changes the binary would make the other side's, unreviewed; with
no trusted binary (an untracked one the merge changes, or a symlink) it says
so and only the index check (plain `sh` and `awk`) runs. If the copy itself
fails, the merge is refused. A fast-forward and
`git merge --no-verify` run no hook; CI is the backstop for those.

```sh
# >>> erg managed >>>
# Check the merge result before git records it: a clean merge runs this
# hook, not pre-commit. A fast-forward or --no-verify runs neither.
if [ -n "$(git diff --cached --name-only HEAD -- tickets)" ]; then
    # Run this clone's committed binary (HEAD's blob), never the working
    # tree's: a merge that changes tickets/erg would otherwise run the
    # incoming side's binary, unreviewed. An untracked local build is used
    # only when the merge leaves tickets/erg alone and it is no symlink.
    erg_bin=
    if git ls-tree HEAD -- tickets/erg | grep -q '^100755 blob '; then
        erg_bin=$(git rev-parse --git-path erg-merge-check)
        erg_from="this clone's committed tickets/erg"
        if ! { git cat-file blob HEAD:tickets/erg > "$erg_bin" && chmod +x "$erg_bin"; }; then
            rm -f "$erg_bin" 2>/dev/null || :
            echo "pre-merge-commit: cannot copy this clone's tickets/erg to $erg_bin; refusing the merge." >&2
            echo " Free space or fix permissions in the git directory, then retry (bypass: git merge --no-verify)." >&2
            exit 1
        fi
    elif [ -x tickets/erg ] && [ ! -L tickets/erg ] && [ -z "$(git diff --cached --name-only HEAD -- tickets/erg)" ]; then
        erg_bin=tickets/erg
        erg_from="the local tickets/erg build"
    fi
    if [ -n "$erg_bin" ]; then
        erg_rc=0
        "$erg_bin" check tickets/ || erg_rc=$?
        [ "$erg_bin" = tickets/erg ] || rm -f "$erg_bin"
        if [ "$erg_rc" -ne 0 ]; then
            echo "pre-merge-commit: ticket corpus check failed on the merge result (checked by $erg_from)." >&2
            echo " Fix it and conclude with 'git commit', or abort: git merge --abort." >&2
            echo " If the merge brings a newer erg whose format this binary rejects, review it, then conclude with 'git commit'." >&2
            exit 1
        fi
    elif [ -e tickets/erg ]; then
        echo "pre-merge-commit: no trusted tickets/erg to run (only one this merge cannot change runs: HEAD's committed executable, or an untracked build the merge leaves alone, never a symlink);" >&2
        echo " the claim index check below still runs. Review the new binary before your next commit." >&2
    else
        echo "ERROR: tickets/erg not found. Build it from the git-erg source" >&2
        echo "  (make build there), or 'git pull' if your project vendors the committed binary." >&2
        exit 1
    fi
fi

erg_hook=pre-merge-commit
# Every ticket in the index has its ID claim in the index. The index, not
# the working tree: an untracked claim on disk lets erg check pass while
# the commit lacks it.
erg_unclaimed=$(git -c core.quotePath=false ls-files -- tickets | awk '
    /^tickets\/\.claims\/[0-9][0-9][0-9][0-9]$/ { claimed[substr($0, length($0) - 3)] = 1; next }
    /^tickets\/(.*\/)?[0-9][0-9][0-9][0-9]-[^\/]*\.erg$/ {
        n = split($0, part, "/")
        ticket[$0] = substr(part[n], 1, 4)
    }
    END { for (t in ticket) if (!(ticket[t] in claimed)) print t }' | sort)
if [ -n "$erg_unclaimed" ]; then
    echo "$erg_hook: tickets without a claim in the index:" >&2
    echo "$erg_unclaimed" | sed 's/^/  /' >&2
    echo " Stage each claim: git add tickets/.claims/NNNN (none yet? run: tickets/erg migrate --claims tickets)" >&2
    exit 1
fi
# <<< erg managed <<<
```

## 2. Agent instructions

*(Skip if you are not using AI coding agents.)*

**Automated:** run `erg install --inject-agents`. It adds the pointer line
inside a sentinel-marked block in your root `AGENTS.md`. If you have no
`AGENTS.md`, it refuses unless you also pass `--create-agents-md` (so it
never creates a root file you did not ask for).

**Manual:** add this block to your `AGENTS.md` (or `CLAUDE.md`,
`.cursorrules`, or whichever file your agent reads at session start):

```
<!-- >>> erg managed >>> -->
git-erg local tickets: see tickets/AGENTS.md
<!-- <<< erg managed <<< -->
```

## 3. Pre-push warning

`erg install --hooks` (step 1) also installs a pre-push hook that WARNS when a
ticket is closed but not yet filed under `closed/` (it prints the exact `erg
archive` + commit + push recipe). Since `erg close` now files the ticket into
`closed/` in one step, this normally only catches a ticket closed by a
hand-edited `Closed:` header. It mutates nothing and never blocks the push: a
pre-push hook cannot get a file move into the push it gates, and a mutating
hook would leave a dirty tree. Filing a hand-closed ticket stays a deliberate
step (`erg archive`, or automatic at merge).

To install only the pre-push hook without the pre-commit hook, use
`erg install --push-hook`.

Never put `erg archive` in a pre-commit hook: archive renames files, and a
pre-commit rename is not re-staged, so the commit would record a deletion
without the matching add. The pre-commit block above intentionally omits it.

## 4. GitHub forge layer (optional)

`tickets/erg-github` is a separate committed helper (not an `erg` subcommand).
`erg-github install` writes a required CI check (`.github/workflows/erg-verify.yml`);
`erg-github verify` fails a PR that references a still-open ticket. Run it
directly: `./tickets/erg-github verify`.

## 5. Working conventions for agents

The shipped `tickets/AGENTS.md` is *resident* context: an agent re-reads every
byte of it at the start of every session. So it stays short, and the long-form
conventions are served here, on demand: this section, then the
handoff-document template and the note on where project-specific lore belongs.
Read them once when you start working with tickets in a repo; re-read them when
a collision or a handoff actually happens.

### ID allocation is optimistic

`erg new` returns one past the highest ID it can see, and it looks in three
places: the store itself, every sibling git worktree at the same relative path,
and every `refs/heads` and `refs/remotes` tip. A sibling session holding an ID
on an unmerged branch is therefore already avoided, without a fetch. A claim
name counts like a ticket, so a removed ticket's number is never handed out
again.

What remains is narrower, and still real. A ref you have not fetched is
invisible, so a collision with another clone is possible until you fetch. The
branch scan runs under a 200 ms deadline and prints a warning on stderr when it
trips -- a result that arrives with that warning may have missed IDs on
un-checked-out branches. And two allocations in different checkouts racing
between the scan and the commit can still land on the same number. So: fetch
before allocating, and read that deadline warning when it appears rather than
past it. The scans make a collision rare; the claims below make it loud.

### ID claims turn a collision into a git conflict

Every store has `tickets/.claims/`, and `erg new` writes `tickets/.claims/NNNN`
before the ticket: one line, the ticket's slug, a space and a random nonce. Two branches that took
the same number add that same path with different content, so the second one
to be merged or rebased meets an add/add conflict -- on the forge, which
recomputes mergeability when the base moves, and locally, offline, with no CI
and no forge call. The nonce makes every same-number pair conflict, even two
sessions filing the same incident under the same slug.

- **Stage the claim with the ticket.** `git add tickets/NNNN-*` misses it;
  `erg new` prints the `git add` line naming both files, and the pre-commit
  hook rejects a commit whose index holds a ticket without its claim.
- **A claim conflict means renumber.** Never resolve it by keeping one side or
  both (`-X ours`, a `merge=union` attribute): that silences the detection,
  and `erg check` then fails the merged tree on a duplicate ID or a
  multi-line claim.
- **Claims are never deleted.** `erg close`, `erg rm` and `erg archive` leave
  them, so the directory records every number ever used; a claim without a
  ticket is legitimate.
- **Picking a number by hand.** `erg next-id` prints the next number. Without
  the binary, fetch and rebase onto the base branch, then take one past the
  highest claim, `ls tickets/.claims | sort -n | tail -1`. That reads this
  checkout only: unmerged branches and sibling worktrees, which
  `erg next-id` scans, stay invisible, and the claim is what turns such a
  collision into a conflict. Write it with the ticket's slug and a fresh
  nonce, `echo "SLUG $(date +%s)" > tickets/.claims/NNNN`, and stage both.
- **Claims are required.** `erg init` creates `tickets/.claims/` in a new
  store. A store without it is noncompliant: `erg check` fails it and
  `erg new` refuses there, each naming the repair. `erg new` cannot produce
  an unclaimed ticket; anything else (a hand-made ticket, one from an older
  erg, a branch committed without hooks) is reported by the next erg command,
  commit or merge in that clone: `new`, `close`, `rm` and `archive` warn,
  the pre-commit and pre-merge-commit hooks refuse. CI is a backstop.
- **One repair: `erg migrate --claims tickets`.** It brings a store without
  `.claims/` to compliance and claims a compliant store's unclaimed tickets.
  It first lists every blocker (a number two tickets share, a claim it cannot
  fix, an ignored claim path) and writes nothing until there is none. Its
  claims are derived from each ticket (number, slug, `Created:`), so two
  clones that migrate the same tickets write the same bytes; still, migrate
  once on the shared base (main), commit, then integrate branches. Upgrading
  an existing project: `erg sync`, `erg init`, `erg migrate --claims
  tickets`, commit, then `erg install --hooks` -- the hooks reject every
  commit in a store whose tickets are not claimed yet.

### Renumber clear of the frontier, never to the next free ID

On collision, the later claimant renumbers to a number well clear of the
high-water mark -- not to the next free ID. The next-free ID is
the most contended seat in the repo: every parallel session computes the same
value and races for it, so renumbering to it is exactly as collision-prone as
the allocation that just collided, and chasing the frontier cannot converge
while siblings are still filing. One filing has collided three times in a
single session this way, leaving the default branch red on a duplicate ID
twice; it settled on the first try once it jumped a dozen clear of the
frontier.

The rename covers everything that carries the number:

- the ticket file (`git mv`) and its log;
- its claim: in the conflicted `tickets/.claims/NNNN` keep the base's version
  (it belongs to the ticket that landed first), and write a new claim for the
  new number, `echo "SLUG $(date +%s)" > tickets/.claims/MMMM`;
- references from other tickets on the branch (`Blocked-by:`, body text);
- code comments and configuration that cite the number;
- the branch name, and the pull request's title and close claim -- a stale
  close claim on the old number would close the winner's ticket at merge.

A slug rename rewrites the claim's first field too; `erg check` rejects a
claim whose slug no longer matches its ticket.

A jumped ID is gone for good, so weigh the jump rather than treating it as
free. `erg new` returns max-plus-one, never the first gap, so the numbers you
skip are never handed out later. The trade is a permanent hole against a
repeated collision, and the hole is cheap only while the repo is far from
`9999`.

This rule governs **collision recovery only**. Initial allocation stays dense:
`erg new`'s next-free ID is the correct first try. Jumping clear of the
frontier at creation time is over-application, and its cost is real -- a store
that allocates in round decades wastes most of its ID space and makes the
sequence unreadable.

### Decision records versus artifacts

These are not the same thing, and only one of them belongs in the ticket file.

A ticket's body may hold the *decisions themselves* -- a kickoff note's settled
options, an arbitration verdict, the reasoning behind a choice. That is the
ticket's own process record, load-bearing for the log and the exit-criteria
trail, and it stays in the `.erg` file.

It must not hold the *material the decision was made about* -- a calibration
corpus, a few-shot set, mined training pairs, generated samples. That is an
artifact, and the shipped rule applies: artifacts live in their natural
location in the project tree and are referenced from the ticket body by path.

## Handoff-document sections

When a ticket is created as a handoff document (a new agent will pick it up
cold), the body should include these sections so that agent has complete
context:

```markdown
## Context
What problem or need this addresses. Why now.

## Relevant files
- `path/to/file.py` -- role in this task

## Actions
1. Concrete step
2. Concrete step

## Test
- What test to write first (red step of TDD)

## Verification
- [ ] How to confirm each action worked

## Invariants
- What must not break (tests, build, existing behavior)

## Exit criteria
- Definition of done -- when is this ticket complete?
```

## Project-specific ticket lore: `tickets/LOCAL.md`

The conventions above are generic: they hold in any repo that uses erg. Your
own are not. The names of your CI jobs and helper scripts, your merge-gate
script and its PR-body conventions, the incidents that taught you a rule, the
extension points your forge wrapper hooks into -- that knowledge is worth
writing down, and it must not be written into `tickets/AGENTS.md`.

`tickets/AGENTS.md` is an erg asset: `erg init` upgrades it in place when it is
untouched stock, but once you have edited it, init preserves your copy, skips
the upgrade and exits 2. You keep the edit and stop receiving improvements to
the file, until you merge the two by hand. Put project-specific ticket lore in
`tickets/LOCAL.md` instead. erg never writes, reads, upgrades or deletes that file -- it is yours,
and `erg check` ignores it like any other non-`.erg` file. The shipped
`tickets/AGENTS.md` names it, so an agent reading its resident context knows
where your local rules live.

## Uninstall

To remove erg from your project, delete the binary and the two files
`erg init` placed in `tickets/`:

```sh
rm tickets/.ergrc tickets/AGENTS.md tickets/.erg-assets tickets/erg
```

`tickets/.claims/` is ticket data like the tickets themselves: the record of
every number the store has used. Keep it with them; a store without it fails
`erg check`.

For the pre-commit hook, delete only the lines between the
`# >>> erg managed >>>` and `# <<< erg managed <<<` markers -- this
preserves any other hook content you (or another tool) added. Only if the
hook contains nothing but the erg managed block is it safe to remove the
whole file:

```sh
rm .git/hooks/pre-commit   # ONLY if it holds nothing but the erg block
```

The pre-merge-commit and pre-push hooks use the same markers in
`.git/hooks/pre-merge-commit` and `.git/hooks/pre-push`; remove their managed
blocks the same way.

Likewise for the `AGENTS.md` pointer: delete only the lines between the
`<!-- >>> erg managed >>> -->` and `<!-- <<< erg managed <<< -->` markers.

If you also copied erg to `~/.local/bin` (contributors: `make
install-erg-binary`), remove that copy too:

```sh
rm ~/.local/bin/erg
```

**Your tickets are not removed.** Files you created (`tickets/*.erg`,
`tickets/closed/`) are yours -- erg never deletes them. Remove them
yourself if you no longer need them.

## Keeping a store current

Two workflows have deliberately different trust boundaries.

To align a Linux x86-64 clone with the binary already reviewed and vendored by
its project, then deliver that binary's embedded assets:

  erg sync
  tickets/erg init

By default `erg sync` reads `tickets/erg` from the current project's `origin`.
It does not ask the git-erg project whether a newer binary exists.

To import a new binary explicitly from the git-erg project, review it before
first execution, and make it the version other clones of this project receive:

  erg sync --upstream
  git diff -- tickets/erg
  # review or verify the imported binary here
  tickets/erg init
  git diff -- tickets/
  git commit

What each command does (and does not) touch:

- `erg sync`: replaces `<ticket store>/erg` only, never the system/PATH binary
  used to invoke it. Never writes .ergrc, AGENTS.md, or
  any store file. Asset and default-vocabulary changes in the new binary are
  NOT yet visible to the store -- they require a follow-up `erg init`.

  The vendored binary is always Linux x86-64. On macOS, Windows, or another
  architecture, `erg sync` still refreshes that project/CI artifact, but do not
  execute it locally. Update or rebuild the native system `erg` from the same
  reviewed git-erg revision, then run the native `erg init`.

- `erg init`: delivers embedded-asset changes via the dpkg 3-state rule
  (byte-identical: skip; untouched stock matching the .erg-assets stamp: clean
  upgrade, overwritten; locally edited: preserved, exit 2; `--force` to
  override). A file the stamp says was written by a NEWER erg than the one
  running is preserved too, and init points you at `erg sync` -- which is
  what makes the pair above an order and not a habit: running a stale binary's
  init against a current store would otherwise revert its assets. The default
  label vocabulary is frozen-by-copy into .ergrc at
  init time -- a new default added later to the binary is shadowed by the
  existing file and never takes effect until `erg init` overwrites it. Running
  `erg sync` alone cannot un-shadow a frozen vocabulary.

- `erg migrate`: ticket-format conversion (Status: -> Closed:, Tag: -> Label:,
  etc.) and project layout upgrade (archive/ -> closed/, stale hook rewrites).
  It rewrites the .ergrc `[tags]` section header to `[labels]` as a one-time
  format migration, but does NOT deliver or refresh configuration content --
  the default label vocabulary and other config is `erg init`'s job, not
  migrate's. Run `erg migrate` after sync and the platform-appropriate init when the new
  binary introduced ticket-format changes.

On Linux x86-64 the canonical full sequence is `erg sync`, then
`tickets/erg init`, then `tickets/erg migrate DIR` when the release notes
mention format changes. Other platforms use their correspondingly updated
native system binary for init and migrate.

## Optional: .gitignore

Add `tickets/erg` to `.gitignore` if you do not want to commit the
bootstrap binary. If you *do* commit it (recommended for offline
environments), skip this step.
