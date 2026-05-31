# git-erg

**An issue tracker that's just files in your repo — offline, agent-native, zero-install.**

Author: Minh Ha-Duong <minh.ha-duong@cnrs.fr>
Last modified: 2026-05-29
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
  file read, not an API call — it works on a plane, in CI, in a locked-down
  sandbox.
- **Zero lock-in.** It's text in git. No DB to migrate, no export step, no
  vendor. `rm -rf tickets/` and it's gone.
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

## POSIX first, CLI second

`.erg` files are plain text. The *first* transport — and the contract every
component agrees on — is POSIX: a `.erg` file IS a ticket; `cat`, `grep`,
`find`, `sed`, `vim`, and an agent's `Read`/`Edit` tools are the universal
interface, available anywhere a shell runs. The `erg` binary is the *second*
transport: a single static Go file that adds validation, atomic mutation, and
`--json` queries on top of the same files. Both transports serve the same
four users:

| User   | POSIX                               | CLI (`erg`)                       |
|--------|-------------------------------------|-----------------------------------|
| Hooks  | `grep -l '^%erg' tickets/*.erg`     | `erg validate FILES`              |
| CI     | `find tickets -name '*.erg'`        | `erg check tickets/`              |
| Agents | `Read`/`Edit` on `.erg`             | `erg list --json`, `erg close ID` |
| Humans | `cat`, `vim`, `ls tickets/`         | `erg ready`, `erg --help`         |

Could a *third* transport — MCP, an HTTP API, a language SDK — be worth adding?
The bar is high: POSIX is already universal and zero-install, and the CLI is
already a single static binary with `--json`. The trade-offs, and why the door
isn't closed, are in `pep-erg-v1.md` §8.

## Specification

- Normative specification:  `tickets/spec-erg-v1.md`.
- Reference implementation: `src/go/` (source), `tickets/erg` (bootstrap binary).
- Design rationale: `pep-erg-v1.md`.
- Contributing (build, test, adding a subcommand): `CONTRIBUTING.md`.

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

   On any other platform, clone this repo and `make build` from `src/go/`
   (Go needed for this step only). See **Binary policy** below for why the
   binary ships in the repo.
3. Run `tickets/erg init` to unpack `AGENTS.md`, `spec-erg-v1.md`, and
   `integration.md`.
4. Follow `tickets/integration.md` to a/ install the pre-commit validation hook
   and b/ tell your agent that ticket-management instructions live in
   `tickets/AGENTS.md`.

**No prebuilt binary for your platform? You don't need one.** The POSIX path
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

`tickets/erg update` looks up the committed binary at your git remote and
replaces the running one in place if it differs. Details: `erg update --help` or
`docs/erg-manual.md`.

## Binary policy

**Source is primary; the committed binary is an optional cache.** The
authoritative artifact is `src/go/`, which travels in every clone. `tickets/erg`
is a committed Linux x86-64 bootstrap *convenience* for environments where Go
may be unavailable (CI runners, agents) and where the POSIX path isn't enough —
built from the vendored source, not a separate source of trust (bit-for-bit
reproducibility is the goal). (Rationale and the supply-chain trade-offs:
`docs/audit-infrastructure-class.md`.)

Because the binary is a cache of the source, the aim is that anyone can rebuild
it bit-for-bit and verify the committed blob matches — that reproducibility is
what justifies shipping a binary at all, and is the keystone control. How the
binary is rebuilt, refreshed, and kept out of the test path is build-side
mechanics; those live in [`CONTRIBUTING.md`](CONTRIBUTING.md).

## Verifying the binary

The committed `tickets/erg` is a convenience cache of the source, and a cache is
only as trustworthy as your ability to check it. Two tiers — pick the one that
fits your threat model.

**Basic — transit integrity.** `erg version` prints the binary's full
SHA-256 digest and names the algorithm, so stock tools can reproduce it:

```bash
sha256sum tickets/erg          # or: shasum -a 256, openssl dgst -sha256, certutil
tickets/erg version            # compare against the sha256: line it reports
```

If those two agree, the file was not corrupted in transit. Note: this is
self-attestation — a tampered binary would report its own hash. It confirms
the bits on disk match the running code, not that the code is authentic. For
authenticity, use `make verify` (rebuild from source, below) or a signed
release tag.

**Release cadence, not CI cadence.** Signed tags cover specific releases
— not every CI rebuild. The `curl` command above pins to the latest signed
tag, so the binary you download is always the one the maintainer attested.
`main` may contain newer CI-rebuilt binaries; verify those with `make verify`
(rebuild from source) instead of a signed tag.

`git verify-tag <tag>` confirms the published hash came from the maintainer
(git-native trust). Import the key first:

```bash
gpg --recv-keys 4A46C91E03B83B23   # from keys.openpgp.org
# or: gpg --import signing-key.asc  # from this repo
```

Key fingerprint: `04E2 A281 FC44 DCA6 9C71  4A6A 4A46 C91E 03B8 3B23`
UID: `Minh Ha-Duong (git-erg signing) <minh.ha-duong@cnrs.fr>`

**Advanced — don't trust the blob, rebuild it.** The source travels in the
repo, so you can rebuild offline and byte-compare:

```bash
make verify     # rebuilds tickets/erg from src/go/ and diffs it — expect: verify: PASS
```

For the highest assurance, read `src/go/` (or have a tool review it) before you
rebuild — it's small, and reviewable source beats an opaque blob.

The full picture — what we defend, against whom, and the repeatable check —
lives in [`docs/threat-model.md`](docs/threat-model.md) and
[`docs/red-team-checklist.md`](docs/red-team-checklist.md).

## License

MIT
