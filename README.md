# git-erg

**An issue tracker that's just files in your repo — offline, agent-native, zero-install.**

Author: Minh Ha-Duong <minh.ha-duong@cnrs.fr>
Last modified: 2026-05-31
Status: Working draft

## Start in 10 seconds — nothing to install

A ticket *is* a text file. You already have everything you need:

```bash
mkdir tickets
cat > tickets/0001-add-auth.erg <<'EOF'
%erg 0.1
Title: Add authentication flow
Created: 2026-05-29
Author: me

--- log ---
2026-05-29T10:00Z me created

--- body ---
Need auth before shipping the API.
EOF

ls tickets/                          # there's your backlog
grep -L '^Closed:' tickets/*.erg     # there's your open list
```

That's a working ticket system: in git, offline, no account, no server, no
database. Delete `tickets/` and it never existed.

## Why it's a no-brainer

- **Zero-install start.** `cat`, `grep`, `find`, `vim`, your agent's
  `Read`/`Edit` — the plain `.erg` file *is* the interface. No binary required
  to begin.
- **Offline-first.** No network, no API, no SaaS. Listing open work is a local
  file read, not an API call: `ls tickets/` works on a plane, in CI, in a locked-down
  sandbox.
- **A captain's log, not just a backlog.** Closing a ticket or adding a note is
  an append-only entry in git. You keep a versioned, `git blame`-able record of how
  you steered the repo, that travels with the code for reproducibility and audit.
- **Zero lock-in — owned, not rented.** It's text in git. No DB to migrate, no export
  step, no vendor whose terms can change under you. `rm -rf tickets/` and it's gone.
- **Agent-native.** Your coding agent picks its own work offline — no API keys,
  no MCP wiring. Drop `tickets/` into a repo and any agent (Claude Code,
  Cursor, Aider, Codex, …) reads, writes, and closes tickets with the tools it
  already has.

## Want more? Add the `erg` binary (optional)

The `erg` CLI is a fast, token-saving upgrade *on the same files*: strict
validation, `--json` queries, atomic close, and a pre-commit guardrail. It's
optional — the files stay the source of truth, and a hand edit always wins.
See **Install into a project** below.

---

The rest of this README is the design depth — how the plain files and the `erg`
CLI fit together, the spec, and the binary policy. The short version: the `.erg`
file is the universal interface, the CLI adds validation and `--json` on top, and
the same pair serves hooks, CI, agents, and humans (see below).

## Text files first, CLI second

Ticket `.erg` files are plain text. The *first* interface is text tools: `cat`, `grep`,
`find`, `sed`, `vim`, and an agent's `Read`/`Edit` tools. This is the universal
interface, available anywhere a shell runs. The `erg` binary offers a *second*
interface: a single static Go file that adds validation, atomic mutation, and
`--json` queries on top of the same files. Both interfaces serve the same
four users:

| User   | Text files direct                    | With the `erg` tool               |
|--------|--------------------------------------|-----------------------------------|
| Hooks  | `grep -l '^%erg 0.1' tickets/*.erg`  | `erg validate FILES`              |
| CI     | `find tickets -name '*.erg'`         | `erg check tickets/`              |
| Agents | `Read`/`Edit` on `.erg`              | `erg list --json`, `erg close ID` |
| Humans | `cat`, `vim`, `ls tickets/`          | `erg ls`, `erg --help`            |

## Specification

- Normative specification: `erg spec` (or `tickets/spec-erg-v1.md` in older installs).
- Reference implementation: `src/go/` (source), `tickets/erg` (bootstrap binary).
- Design rationale: `pep-erg-v1.md`.
- Contributing (build, test, adding a subcommand): `CONTRIBUTING.md`.

## Forge layer: `erg-github`

`erg core` is offline and forge-blind. `tickets/erg-github` is a separate
committed POSIX-sh helper (it travels with the clone) that adds GitHub
integration. It is not a subcommand of `erg` -- run it directly:

- `tickets/erg-github install` writes `.github/workflows/erg-verify.yml`, a
  required pre-merge check.
- `tickets/erg-github verify` fails a PR that references a still-open ticket
  ("Please close ticket NNNN in this PR"). The close is committed in the PR by
  the author (`erg close`); this only discovers and enforces, with no bot and
  no dependency on any merge tooling. A PR that references no ticket passes
  (an audited escape hatch for emergency/unlinked merges), and outside CI the
  check degrades non-blockingly when `gh` is unavailable.

## Install into a project

The zero-install path above already works. To add the optional `erg` CLI,
drop it into `tickets/` so each project carries its own pinned version — no
global install, no version skew across machines or teammates.

1. Create a `tickets/` dir at the project's root (if you haven't).
2. Drop the `erg` binary into it. If you have a git-erg clone, the
   committed `tickets/erg` is the prebuilt Linux x86-64 binary — just copy
   it. From outside a clone, download it:

   ```bash
   curl -fsSL https://github.com/MinhHaDuong/git-erg/raw/2026-05-30/tickets/erg \
     -o tickets/erg && chmod +x tickets/erg
   ```

   The URL is pinned to the signed release tag `2026-05-30`. To get a newer
   version, find the latest tag at
   [github.com/MinhHaDuong/git-erg/tags](https://github.com/MinhHaDuong/git-erg/tags)
   and substitute it in the URL above.

   This committed `tickets/erg` is the *traveling* binary (Linux x86-64) — keep
   it as-is for hooks and CI. To also run `erg` locally on macOS, ARM, or Windows,
   build a *system* binary for your machine (clone this repo and `make build`, or
   ask your coding agent to) and put it on your PATH; don't overwrite `tickets/erg`.
   See **Binary policy** below.
3. Run `tickets/erg init` to unpack `.ergrc` and `AGENTS.md`.
4. Run `tickets/erg install --hooks` to set up the pre-commit validation hook,
   and (for AI agents) `tickets/erg install --inject-agents` to point your root
   `AGENTS.md` at `tickets/AGENTS.md`. Run `erg integration` for the full guide.

**No prebuilt binary for your platform? You don't need one.** The text-files path
is fully functional without `erg`, and a `grep`-based pre-commit hook validates
tickets without the binary. Why the binary ships is in **Binary policy** below;
build and CI mechanics are in [`CONTRIBUTING.md`](CONTRIBUTING.md).

## Quick start (with the `erg` binary)

The zero-install demo at the top creates and queries tickets with stock POSIX
tools. The `erg` binary adds validation and structured queries over the same
files:

```bash
# Create a new ticket
tickets/erg new "Add authentication flow"
# → CREATED 0001-add-authentication-flow.erg

# Validate a single ticket (validate takes file paths, not IDs)
tickets/erg validate tickets/0001-add-authentication-flow.erg

# Or validate the whole store at once
tickets/erg check tickets/

# List ready tickets
tickets/erg ready
```

## Managing dependencies

Add `Blocked-by:` lines to a ticket header to encode dependencies; `tickets/erg
close ID REASON` clears them from dependents automatically (otherwise you edit
the lines out by hand). Full semantics: `erg close --help` or
`docs/erg-manual.md`.

## Updating

`tickets/erg update` looks up the committed binary at the git remote and
replaces the running one in place if it differs. Details: `erg update --help` or
`docs/erg-manual.md`.

## Binary policy

`erg` ships as two binaries, by role:

- **Traveling binary** — committed at `tickets/erg`, always Linux x86-64. It rides
  in the repo so the same bytes run your hooks and CI/CD on the Linux boxes
  automation uses. One artifact for the whole team; nobody cross-compiles for the
  pipeline.
- **System binary** — on your PATH, built for your own architecture, for
  interactive use. On Linux x86-64 it's the same build as the traveling one (you
  can just run `tickets/erg`); on macOS, ARM, or Windows you build a native one —
  clone git-erg and `make build`, or ask your coding agent to — and run that
  locally. Either way, leave `tickets/erg` as the Linux binary: overwriting it with
  your architecture breaks the shared pipeline.

Both are caches of the source in `src/go/` — the authoritative artifact, which
lives in git-erg and is never copied into the projects that use erg. The committed
binary exists for boxes where Go is unavailable and the text-files path isn't
enough. Rationale: `docs/audit-infrastructure-class.md`. Rebuild/refresh mechanics:
[`CONTRIBUTING.md`](CONTRIBUTING.md). Verification: below.

`erg version` reports which role the running binary is (`role: traveling` for a
path ending in `tickets/erg`, `role: system` otherwise), so you never have to
guess which copy you're looking at.

**Opt out of committing the binary.** Committing `tickets/erg` is the default —
one artifact runs everyone's hooks and CI. If you'd rather not vendor a binary
(e.g. every box has Go, or policy forbids committed binaries), add `tickets/erg`
to your `.gitignore`. Then there is no traveling copy: each machine builds its own
system binary (`make build`, or the curl install below) and that single copy on
your PATH does everything. The leverage against "which binary is this?" confusion
is this one-binary-on-disk opt-out, not an arch suffix — the traveling binary,
when committed, stays a single Linux x86-64 artifact for CI and agents.

## Verifying the binary

The committed `tickets/erg` is a convenience, but also a risk. Pick the trust model
that fits you — nobody should run binaries from the internet blindly.

### a/ Trust the maintainer; verify you got the right blob
Releases are signed. Import the key once, check the tag you pinned to, and confirm
the file's hash:

```bash
gpg --recv-keys 4A46C91E03B83B23   # from keys.openpgp.org (or: gpg --import signing-key.asc)
git verify-tag 2026-05-30          # confirms the maintainer signed this release
sha256sum tickets/erg              # compare against the hash that tag attests
```

Key fingerprint: `04E2 A281 FC44 DCA6 9C71  4A6A 4A46 C91E 03B8 3B23`
UID: `Minh Ha-Duong (git-erg signing) <minh.ha-duong@cnrs.fr>`

This proves the blob is the one the maintainer published — i.e., authenticity. If you
took a `main` build rather than a signed release, there's no tag to check — use **b/**
below.

### b/ Trust no one; have your AI verify
The source is small, single-package, and dependency-free, so it's genuinely
reviewable. Point a capable AI at `src/go/`, have it audit the code, then rebuild and
byte-compare:

```bash
make verify     # rebuilds tickets/erg from src/go/ and diffs it — expect: verify: PASS
```

Reviewable source beats an opaque blob. Needs Go, but no trust in the committed binary or the maintainer.

The full threat model — what we defend, against whom, and the repeatable check — is in
[`docs/threat-model.md`](docs/threat-model.md) and
[`docs/red-team-checklist.md`](docs/red-team-checklist.md).

## License

MIT
