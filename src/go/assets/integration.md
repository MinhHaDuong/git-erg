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
rebuilds the binary after merge to main -- and validates staged tickets.
See the `.gitignore` section below for the full commit policy.

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

`erg new` scans only the local checkout, so parallel sessions on different
branches or in different checkouts can hand out the same ID. No reservation
machinery exists or is planned. Fetch before allocating, and run `erg check`
after every fetch in ticket-heavy sessions.

Re-run the collision scan **at the merge gate too**, not only at allocation: a
sibling PR can renumber onto your ID after you allocated, and once that sibling
has merged, a cross-PR CI gate no longer sees the collision -- such gates
compare open PRs against each other, never a PR against the default branch.

### Renumber clear of the frontier, never to the next free ID

On collision, renumber (`git mv` plus fixing cross-references) to a number well
clear of the high-water mark -- not to the next free ID. The next-free ID is
the most contended seat in the repo: every parallel session computes the same
value and races for it, so renumbering to it is exactly as collision-prone as
the allocation that just collided, and chasing the frontier cannot converge
while siblings are still filing. IDs are free and a gap costs nothing. One
filing has collided three times in a single session this way, leaving the
default branch red on a duplicate ID twice; it settled on the first try once it
jumped a dozen clear of the frontier.

This rule governs **collision recovery only**. Initial allocation stays dense:
`erg new`'s next-free ID is the correct first try. Jumping clear of the
frontier at creation time is over-application, and its cost is real -- a store
that allocates in round decades wastes most of its ID space and makes the
sequence unreadable.

### Scan for collisions with `gh pr view`, never `gh pr list --json files`

`gh pr list` does not populate `files`, so a one-shot list-plus-filter returns
empty regardless of content -- "no collision found" and "I never looked" are
the same output. Enumerate, then query each PR:

```sh
for n in $(gh pr list --state open --limit 60 --json number --jq '.[].number'); do
  gh pr view "$n" --json files --jq '.files[].path' | grep -q "tickets/$ID" && echo "PR $n uses $ID"
done
```

General form of the trap: a check whose "all clear" is indistinguishable from
its "I could not look" is not a check. Before trusting a scan that returns
nothing, run it against a case known to be positive.

(This is documentation, printed by `erg integration`. `erg` itself never shells
out to a forge -- the core stays offline.)

### After a ticket PR merges, run `erg check` against `origin/main`, not the branch

A branch-local `erg check` passes by construction -- each branch's IDs are
unique within itself -- so it structurally cannot see a duplicate created by
another PR. Checking the merged `origin/main` is the only check that catches a
duplicate that has already landed, and in a repo with no CI it is the only
collision check there is.

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

For the pre-commit hook, delete only the lines between the
`# >>> erg managed >>>` and `# <<< erg managed <<<` markers -- this
preserves any other hook content you (or another tool) added. Only if the
hook contains nothing but the erg managed block is it safe to remove the
whole file:

```sh
rm .git/hooks/pre-commit   # ONLY if it holds nothing but the erg block
```

The pre-push hook (if you ran `--push-hook`) uses the same markers in
`.git/hooks/pre-push`; remove its managed block the same way.

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

After upgrading the erg binary, run both commands to absorb embedded-asset
changes and updated default label vocabulary:

  erg update && erg init

What each command does (and does not) touch:

- `erg update`: replaces the binary only. Never writes .ergrc, AGENTS.md, or
  any store file. Asset and default-vocabulary changes in the new binary are
  NOT yet visible to the store -- they require a follow-up `erg init`.

- `erg init`: delivers embedded-asset changes via the dpkg 3-state rule
  (byte-identical: skip; untouched stock matching the .erg-assets stamp: clean
  upgrade, overwritten; locally edited: preserved, exit 2; `--force` to
  override). A file the stamp says was written by a NEWER erg than the one
  running is preserved too, and init points you at `erg update` -- which is
  what makes the pair above an order and not a habit: running a stale binary's
  init against a current store would otherwise revert its assets. The default
  label vocabulary is frozen-by-copy into .ergrc at
  init time -- a new default added later to the binary is shadowed by the
  existing file and never takes effect until `erg init` overwrites it. Running
  `erg update` alone cannot un-shadow a frozen vocabulary.

- `erg migrate`: ticket-format conversion (Status: -> Closed:, Tag: -> Label:,
  etc.) and project layout upgrade (archive/ -> closed/, stale hook rewrites).
  It rewrites the .ergrc `[tags]` section header to `[labels]` as a one-time
  format migration, but does NOT deliver or refresh configuration content --
  the default label vocabulary and other config is `erg init`'s job, not
  migrate's. Run `erg migrate` after `erg update && erg init` when the new
  binary introduced ticket-format changes.

Canonical full sequence: `erg update && erg init`, then `erg migrate DIR` when
the release notes mention format changes.

## Optional: .gitignore

Add `tickets/erg` to `.gitignore` if you do not want to commit the
bootstrap binary. If you *do* commit it (recommended for offline
environments), skip this step.
