# Integration

After running `erg init`, follow these two steps to integrate git-erg
with your project.

## 1. Pre-commit hook

The hook prevents committing `tickets/erg` on feature branches — CI
rebuilds the binary after merge to main. See the `.gitignore` section
below for the full commit policy.

Append the following to `.git/hooks/pre-commit` (create the file and
`chmod +x` it if it does not exist):

```sh
# Reject tickets/erg commit on non-main branches
# CI rebuilds the binary after merge; feature PRs must not include it.
if git diff --cached --name-only | grep -q '^tickets/erg$'; then
    branch=$(git branch --show-current)
    if [ "$branch" != "main" ]; then
        echo "pre-commit: do not commit tickets/erg in feature branches." >&2
        echo " CI rebuilds the binary after merge. Use 'make build' and test" >&2
        echo " with build/erg. To override: git commit --no-verify" >&2
        exit 1
    fi
fi

# Validate .erg files (if any are staged)
erg_files=$(git diff --cached --name-only | grep '\.erg$' || true)
if [ -n "$erg_files" ]; then
    erg_bin="tickets/erg"
    if [ -x "$erg_bin" ]; then
        # shellcheck disable=SC2086
        if ! $erg_bin validate $erg_files; then
            echo "ERROR: Ticket validation failed. Fix errors above." >&2
            exit 1
        fi
        if ! $erg_bin check tickets/; then
            echo "ERROR: Ticket corpus check failed. Fix errors above." >&2
            exit 1
        fi
    else
        echo "ERROR: erg binary not found. Run 'make build' first." >&2
        exit 1
    fi
fi
```

## 2. Agent instructions

*(Skip if you are not using AI coding agents.)*

Add this line to your `AGENTS.md` (or equivalent agent-visible file):

```
git-erg local tickets: see tickets/AGENTS.md
```

If your project has no `AGENTS.md`, create one at the project root. You
can also add the line to `CLAUDE.md`, `.cursorrules`, or whichever file
your agent reads at session start.

## Uninstall

To remove erg from your project, delete the binary and the four files
`erg init` placed in `tickets/`, plus the pre-commit hook (if you
installed one in step 1):

```sh
rm tickets/.ergrc tickets/AGENTS.md tickets/spec-erg-v1.md tickets/integration.md tickets/erg
rm .git/hooks/pre-commit   # if you added the hook from step 1
```

If you installed erg to `~/.local/bin` via `make install-erg-binary`,
also remove that copy:

```sh
rm ~/.local/bin/erg
```

**Your tickets are not removed.** Files you created (`tickets/*.erg`,
`tickets/closed/`) are yours -- erg never deletes them. Remove them
yourself if you no longer need them.

If you added the `AGENTS.md` line from step 2, remove that line
manually from your agent-visible file.

## Optional: .gitignore

Add `tickets/erg` to `.gitignore` if you do not want to commit the
bootstrap binary. If you *do* commit it (recommended for offline
environments), skip this step.
