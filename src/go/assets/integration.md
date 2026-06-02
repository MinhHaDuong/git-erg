# Integration

After running `erg init`, follow these two steps to integrate git-erg
with your project. The automated path is `erg install` (see below);
the manual steps that follow describe exactly what it writes.

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
# Reject tickets/erg commit on non-main branches.
# CI rebuilds the binary after merge; feature PRs must not include it.
if git diff --cached --name-only | grep -q '^tickets/erg$'; then
    branch=$(git branch --show-current)
    if [ "$branch" != "main" ]; then
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
        echo "ERROR: tickets/erg not found. Run 'make build' first." >&2
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

## 3. Pre-push warning (optional)

`erg install --push-hook` adds a pre-push hook that WARNS when tickets are
closed but not yet archived (it prints the exact `erg archive` + commit + push
recipe). It mutates nothing and never blocks the push: a pre-push hook cannot
get a file move into the push it gates, and a mutating hook would leave a dirty
tree. Archiving stays a deliberate step (`erg archive`, or automatic at merge).

Never put `erg archive` in a pre-commit hook: archive renames files, and a
pre-commit rename is not re-staged, so the commit would record a deletion
without the matching add. The pre-commit block above intentionally omits it.

## 4. GitHub forge layer (optional)

`tickets/erg-github` is a separate committed helper (not an `erg` subcommand).
`erg-github install` writes a required CI check (`.github/workflows/erg-verify.yml`);
`erg-github verify` fails a PR that references a still-open ticket. Run it
directly: `./tickets/erg-github verify`.

## Uninstall

To remove erg from your project, delete the binary and the two files
`erg init` placed in `tickets/`:

```sh
rm tickets/.ergrc tickets/AGENTS.md tickets/erg
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

## Optional: .gitignore

Add `tickets/erg` to `.gitignore` if you do not want to commit the
bootstrap binary. If you *do* commit it (recommended for offline
environments), skip this step.
