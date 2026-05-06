# Ticket format spec — %erg v1

Author: Minh Ha-Duong <minh.ha-duong@cnrs.fr>
Last modified: 2026-05-04
Status: Working draft

## Introduction

`git-erg` is an agent-friendly local ticket system for development in disconnected environment.
Tickets are committed to git and travel with the repo.
It intends to complement and work along with forges like GitLab or GitHub, which solve inter-agent and human coordination.

This file is normative and comprises two parts. It defines the format for valid `%erg v1` files and the `erg` binary utility to manage the tickets store. Rationale and design decisions are documented in `pep-erg-v1.md`.

Any divergence between this document and the `erg` binary reference implementation  must be resolved by aligning the specification with the behavior. The utility is optional: the ticket store is designed as a collection of text files so that the Unix toolkit works starting with `ls tickets` and `ls tickets/closed`.

## File format

Extension: `.erg`
Location: `tickets/`
Encoding: UTF-8, LF line endings.

### Magic first line

```
%erg v1
```

Every `.erg` file starts with this line. It declares the format version
and enables file-type detection without relying on the extension. A future
`%erg v2` adds headers without breaking v1 validators (they reject
unknown versions rather than silently misparsing).

### Structure

```
%erg v1
Title: Short imperative description
Created: 2026-03-27
Author: claude

--- log ---
2026-03-27T10:00Z claude created

--- body ---
Free-form markdown body.
```

Three sections, in order:
1. **Headers** — RFC 822 style, one per line, immediately after magic line.
2. **Log** — append-only ledger, after `--- log ---` separator.
3. **Body** — free-form markdown, after `--- body ---` separator.

A blank line ends the header block. Both separators are required (the
validator rejects files missing either one).

### Headers (closed set, v1)

| Header | Required | Repeatable | Type | Values |
|--------|----------|------------|------|--------|
| `Title` | yes | no | string | Short imperative sentence |
| `Created` | yes | no | date | `YYYY-MM-DD` |
| `Author` | yes | no | string | Agent or human identifier |
| `Closed` | no | no | string | Closure reason (PR ref, supersession note, etc.); non-empty |
| `Blocked-by` | no | yes | ref | Local `NNNN` or forge ref `host/owner/repo#N` (see grammar) |
| `Tags` | no | yes | enum | `needs-human`, `deferred`, `post-talk`, `post-conference` |

No other headers are valid in v1. No `X-` extensions. If v2 needs new
headers, it declares `%erg v2` and extends the set.

**`Closed:` header:**
- Optional, non-repeatable, preamble only.
- Value is required and non-empty — it carries the reason for closure
  (PR reference, supersession note, "abandoned — out of scope", …).
- Forbidden in the log and body sections (header-key match at line
  start; substrings inside prose are fine).
- Examples:
  - `Closed: completed in PR #5`
  - `Closed: superseded by 0099`
  - `Closed: abandoned — out of scope`

**`Blocked-by` references** take one of two forms:

```
ref        := local-ref | forge-ref
local-ref  := [0-9]{4}
forge-ref  := host "/" owner "/" repo "#" number
host       := [A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?
owner      := [A-Za-z0-9_.-]+
repo       := [A-Za-z0-9_.-]+
number     := [1-9][0-9]*
```

- **Local** — `0042` refers to a ticket in this `tickets/` directory.
- **Forge** — `github.com/owner/repo#N` names an issue or PR on any
  code forge. The hostname is the forge identity; no scheme prefix.
  Owner and repo use a loose pattern — forge-specific validation is
  not erg's job.

`#` is reserved as the issue-number separator. The number must be a
positive integer with no leading zero.

Repeatable: one `Blocked-by:` line per dependency. A local blocker
must be **closed** (per the criterion below) to unblock. Forge refs
are always **unknown** — erg never makes network calls. Unknown is
blocking by default; remove the line once you have verified the
upstream dependency is resolved.

The system is disconnected by design, `erg` never queries external forges.

There is no pending or doing header by design.
If two agents need to avoid stepping on each other, they should observe out-of-band
signals — typically a git branch whose name contains the ticket ID — and
coordinate there.

### Closed / not-closed criterion

A ticket is **closed** if at least one of these holds:

1. **Path test.** A path component (directory name or basename without
   extension) equals `closed` (case-insensitive), starts with `closed-`
   or `closed.`, or ends with `-closed`. Covers `tickets/closed/`,
   `0001-foo-closed.erg`. Rules out `disclosed`,
   `enclosed`.
2. **Header test.** A preamble line begins with `Closed:`
   (header-key match at line start; value required, non-empty).

Otherwise the ticket is **not-closed** (open).

There is no other state. WIP is observable out of band (a branch
whose name contains the ticket ID). `pending` and `doing` are no
longer expressible.

### ID assignment

The ticket ID is derived from the filename, not a header.

Filename pattern: `{ID}-{slug}.erg`
- ID: zero-padded sequential number, 4 digits (`0001`, `0002`, …)
- Slug: lowercase kebab-case, ASCII only (`[a-z0-9-]`)

Preferred method:

```sh
erg next-id tickets/
```

This command reads the files directly in the given directory (non-recursive),
selects filenames ending in `.erg`, extracts the numeric prefix before the
first `-` if present (or the full stem otherwise), and keeps the maximum
parsable integer. The next ID is that maximum plus 1, zero-padded to 4 digits.

If no valid ticket filenames are found, or the directory does not exist,
the command returns `0001`.

Fallback method (if the binary is unavailable): perform the same filename
scan on `.erg` files in `tickets/`.

The scan is local to the working directory. Other branches, worktrees,
or remotes are not considered.

**Collision handling:** optimistic. Multiple worktrees or branches may
select the same ID concurrently. The pre-commit validator rejects
duplicates. The agent that loses renames its ticket by obtaining a new
ID and retrying.

### Log section

Append-only. Each line records one event:

```
{ISO-8601-timestamp} {actor} {verb} [{detail}]
```

**Timestamp:** `YYYY-MM-DDThh:mmZ` (UTC, minute precision).
**Actor:** agent or human identifier (e.g., `claude`, `user`).
**Verbs (open set, v1):** any single token. Suggested:

| Verb | Meaning |
|------|---------|
| `created` | Ticket created |
| `closed` | Ticket closed (paired with the `Closed:` header) |
| `note` | Free-form annotation |

Lines are never edited or deleted. To correct an error, append a new line.

### Body section

Free-form markdown. Convention for actionable tickets:

```
## Context
Why this work exists.

## Actions
1. Concrete steps.

## Test
First test to write (TDD red step).

## Exit criteria
Definition of done.
```

Not enforced by the validator. Agents are encouraged to follow the convention
but the body is structurally unconstrained.

## Tickets management tool

### The tickets store

Tickets are stored in a `tickets/` directory at project root.
Closed tickets may go to `tickets/closed/`.

A Linux/AMD64 `erg` binary lives in the `tickets/` repo and is tracked along with tickets.
Virtual machines can use it in isolated environment without recompiling or installation.

The `erg` binary finds the ticket store to operate on as follows:
1. Explicit [DIR] argument. When a command accepts an optional directory argument, that value is used as-is — discovery is skipped entirely.
2. Directory containing the binary. `erg` resolves its own path at runtime and tries the directory it lives in. When installed as `tickets/erg`, this is `tickets/` — the common case.
3. `tickets/` under the working directory. If the binary's own directory is not a store (e.g. `~/.local/bin/`), `erg` tries a subdirectory named `tickets/` in the current directory.
4. Current working directory. As a last resort, `erg` tries the working directory itself, to support bare stores where tickets live at the repo root. However, see condition below.

A directory qualifies as a ticket store if it is named `tickets`, contains a `.erg-bootstrap-manifest.json` file, or contains at least one `.erg` file. If none of the three candidates qualifies, `erg` exits with an error listing the paths it tried.


### File format validation

`erg validate FILES...` enforces:
1. Magic first line is `%erg v1` (reject unknown versions).
2. All required headers present (`Title`, `Created`, `Author`).
3. No unknown headers. `Status:` is unknown — `erg migrate` is the one
   command that tolerates it (in order to convert it).
4. `Created` is a valid ISO date (`YYYY-MM-DD`).
5. Filename matches `NNNN-{slug}.erg` pattern (4-digit ID, ASCII slug).
6. No duplicate IDs within `tickets/`.
7. `Blocked-by` values parse as `local-ref` or `forge-ref` (see
   grammar above). Malformed refs are rejected with a precise message
   identifying the failure mode.
8. `Blocked-by` local refs point to existing ticket IDs.
9. No dependency cycles. Forge refs are terminal from this repo's
   view and cannot participate in local cycles.
10. Log lines match `{timestamp} {actor} {verb}` format.
11. Each separator (`--- log ---`, `--- body ---`) appears exactly once.
12. `Closed:` header appears at most once and has a non-empty value.
13. `Closed:` does not appear in the log or body sections (header-key
    match at line start).

A pre-commit hook to ensure `.erg` files validity is provided under `integration/hooks/`.

### Graph Integrity verification

`erg check [DIR]` verifies that all tickets are valid, and there are no duplicate IDs, no cycles, and Blocked-by references

### Ready query

`erg ready [DIR]` returns unblocked open tickets:

A ticket is **ready** when:
- It is **not-closed** (per the criterion above).
- It does not have any `Blocked-by:` headers.

`erg ready [DIR] --json` returns open tickets with structured
readiness fields:

```json
{
  "id": "0021",
  "title": "ship feature X",
  "file": "0021-ship-feature-x.erg",
  "ready": false,
  "tags": ["needs-human"],
  "blocked_by": [
    {"kind": "local", "id": "0017"},
    {"kind": "forge", "ref": "github.com/org/repo#123"}
  ]
}
```

- `ready=true` implies `blocked_by` is empty.
- `blocked_by` includes only currently blocking refs.
- `tags` is always present (possibly empty).

A ticket will not be returned ready if it has any `Blocked-by` line, even if that reference is in fact closed. User and agents can run `erg check` to detect and delete refs pointing to closed ticket. Note that `erg validate` only verifies reference parseability but does not check the state. And `erg` never resolves forge references by design, so these kind of blockers have to be cleared by editing the text file (delete the `Blocker-by:` line to online ticket, log the change).

### Closing a ticket

`erg close ID REASON [DIR]` closes a ticket atomically:

1. Inserts a `Closed: REASON` header in the preamble.
2. Appends a log line: `{timestamp} claude closed — REASON`.
3. Scans every open ticket in `[DIR]` for `Blocked-by: ID` and
   removes that line, appending a log entry to each modified ticket:
   `{timestamp} claude note blocker ID closed — Blocked-by removed`.

Step 3 keeps the ticket set clean and enables immediate archiving of
the closed ticket (no open ticket will reference it after the command
runs). The removal is recorded in the log of each dependent ticket so
the history of why it was blocked is not lost.

`erg close` is idempotent: running it twice on the same ticket prints
`ALREADY_CLOSED` and exits 0.

For projects tracked with a forge, it is suggested that agents define
a merge skill to bundle close-then-merge, so that the ticket is closed
in the last commit that precedes the merge.

### Archiving

`erg archive [ID...] [DIR]` moves closed tickets to `tickets/closed/`.

The utility ensures that
- Only closed tickets are archived.
- No open tickets reference an archived ticket as a blocker (could happen if the archived ticket was closed by editing the file).


### Migration from %erg v1 with Status

Existing tickets carrying `Status:` headers are converted by
`erg migrate [dir]`:

- `Status: closed` → `Status:` line removed; `Closed: migrated from
  Status: closed` appended to the preamble.
- `Status: open|doing|pending` → `Status:` line removed (ticket
  becomes not-closed).
- No `Status:` line → no-op.

`erg migrate` is idempotent. It does not commit; review with
`git diff tickets/` and commit manually. After migration completes,
`erg validate` rejects any remaining `Status:` lines.

`erg update` never mutates ticket files. When it detects `Status:`
lines after a successful binary swap, it prints an explicit migration
hint (`erg migrate ...`) so migration remains a separate, reviewable
step.

## Postel's Law

The validator enforces %erg v1 on commit (strict on write).

Agents may interpret non-conforming input, but must produce valid %erg v1
when creating or modifying tickets. Non-conforming files are rejected by the validator.
