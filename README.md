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

The rest of this README is the design depth — how the two transports fit
together, the spec, and the binary policy.

- **File-based**: plain text `.erg` files committed to git
- **Offline-first**: no network, no API, no database
- **Zero runtime dependencies**: single Go binary, shell tests
- **Agent-friendly by design**: the spec is the interface, the binary is the guardrail and token-saving utilities
- **Two transports, four users**: POSIX is the first transport, the `erg` CLI is the second, and the same pair serves hooks, CI, agents, and humans (see below)

## POSIX first, CLI second

`.erg` files are plain text. The *first* transport — and the contract every
component agrees on — is POSIX: a `.erg` file IS a ticket; `cat`, `grep`,
`find`, `sed`, `vim`, and an agent's `Read`/`Edit` tools are the universal
interface, available anywhere a shell runs. The `erg` binary is the *second*
transport: a single static Go file that adds validation, atomic mutation, and
`--json` queries on top of the same files. Both transports serve the same
four users:

| User   | POSIX (transport #1)                | CLI (`erg`, transport #2)         |
|--------|-------------------------------------|-----------------------------------|
| Hooks  | `grep -l '^%erg' tickets/*.erg`     | `erg validate FILES`              |
| CI     | `find tickets -name '*.erg'`        | `erg check tickets/`              |
| Agents | `Read`/`Edit` on `.erg`             | `erg list --json`, `erg close ID` |
| Humans | `cat`, `vim`, `ls tickets/`         | `erg ready`, `erg --help`         |

A *third* transport — MCP, an HTTP API, a language SDK — would have to beat
both. POSIX is already universal and zero-install; the CLI is already a
single static binary with `--json`. There is no headroom left for a third
layer to add value without paying installation and version-skew costs the
project does not need. The design rationale is in `pep-erg-v1.md` §8.

## Specification

- Normative specification:  `tickets/spec-erg-v1.md`.
- Reference implementation: `src/go/` (source), `tickets/erg` (bootstrap binary).
- Design rationale: `pep-erg-v1.md`.

## Install into a project

The zero-install path above already works. To add the optional `erg` CLI:

1. Create a `tickets/` dir at the project's root (if you haven't).
2. Drop the `erg` binary into it — use a prebuilt one, or `make build` from
   `src/go/` (Go needed for this step only).
3. Run `tickets/erg init` to unpack `AGENTS.md`, `spec-erg-v1.md`, and
   `integration.md`.
4. Follow `tickets/integration.md` to a/ install the pre-commit validation hook
   and b/ tell your agent that ticket-management instructions live in
   `tickets/AGENTS.md`.

**No prebuilt binary for your platform? You don't need one.** The POSIX path
is fully functional without `erg`, and a `grep`-based pre-commit hook validates
tickets without the binary. Platform and build details are in **Binary policy**
below.

## Quick start

```bash
# Create a ticket (or just write the file — agents do)
cat > tickets/0001-add-auth.erg <<'EOF'
%erg 0.1
Title: Add authentication flow
Created: 2026-03-27
Author: claude

--- log ---
2026-03-27T10:00Z user created

--- body ---
## Context
Need auth before shipping the API.
EOF

# List tickets
ls tickets

# Validate a single ticket
tickets/erg validate 01

# List ready tickets
tickets/erg ready
```

## Managing dependencies

Add `Blocked-by:` lines in the ticket header to encode dependencies.

Dependencies on ticket ID will be automatically cleared by `tickets/erg close ID REASON`, otherwise you have to find them and remove the line by editing the text file.

## Updating

`tickets/erg update` will lookup for a more recent build and replace the existing binary in-place.

## Binary policy

`tickets/erg` is a committed Linux x86-64 bootstrap binary for environments where Go may be unavailable (CI runners, agents). I look forward to working with macOS, ARM or Windows early adopters.

Source lives in `src/go/`. CI builds always compile from source and do not rely on the committed binary: all tests and development builds must use `build/erg`, rebuilt from source via `make build`.

The bootstrap binary is updated explicitly via `make update-bootstrap-binary`
(typically after changes to the Go code or when releasing) and must never be
modified by `make test`.

## License

MIT
