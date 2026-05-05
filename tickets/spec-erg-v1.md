# Ticket format spec — %erg v1

Author: Minh Ha-Duong <minh.ha-duong@cnrs.fr>
Last modified: 2026-05-04
Status: Working draft

## Overview

An agent-friendly local ticket system for development in disconnected environment.
Not a replacement for GitHub Issues — those handle inter-agent and human coordination.
Tickets are committed to git and travel with the repo.

## Scope

This file is normative. It defines the %erg v1 format and validator rules.
The Go `erg` validator is the reference implementation and enforces these rules at commit time.
Any divergence between this document and the validator must be resolved by aligning the specification with the enforced behavior.
Rationale and design decisions are documented in `pep-erg-v1.md`.

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

## Coordination is out of scope

%erg v1 describes what a ticket is, not how concurrent agents or worktrees
share access to one. There is no claim file, no lock, no doing-but-mine state.
If two agents need to avoid stepping on each other, they observe out-of-band
signals — typically a git branch whose name contains the ticket ID — and
coordinate there. Such conventions are workflow choices, not properties of
this format.

## Ready query

A ticket is **ready** when:
- It is **not-closed** (per the criterion above).
- Every `Blocked-by` local ref points to a **closed** ticket.
- Every `Blocked-by` forge ref (`host/owner/repo#N`) is treated as
  **unknown** — erg never makes network calls. Unknown is blocking
  by default (fail-closed). Remove the forge ref from the ticket
  once you have verified the dependency is resolved.

`erg ready --json [dir]` returns open tickets with structured
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

## Closing a ticket

`erg close <id|file> <reason> [dir]` closes a ticket atomically:

1. Inserts a `Closed: <reason>` header in the preamble.
2. Appends a log line: `{timestamp} claude closed — <reason>`.
3. Scans every open ticket in `[dir]` for `Blocked-by: <id>` and
   removes that line, appending a log entry to each modified ticket:
   `{timestamp} claude note blocker <id> closed — Blocked-by removed`.

Step 3 keeps the ticket set clean and enables immediate archiving of
the closed ticket (no open ticket will reference it after the command
runs). The removal is recorded in the log of each dependent ticket so
the history of why it was blocked is not lost.

`erg close` is idempotent: running it twice on the same ticket prints
`ALREADY_CLOSED` and exits 0.

## Archiving

Move a closed ticket to `tickets/archive/` (or any subdirectory of
`tickets/`) once no open ticket references it. `erg validate [dir]`
recurses into subdirectories and validates every `.erg` file it finds.
Archived tickets remain inside validation scope when validating the
top-level `tickets/` directory.

Do not archive a ticket that is still named in a `Blocked-by:` header
of an open ticket — the validator would then report a missing reference.

## Validator rules (pre-commit)

The Go validator enforces:
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

## Migration from %erg v1 with Status

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

## Relationship to GitHub Issues

| Concern | Tool |
|---------|------|
| Local work organization | `.erg` files |
| Multi-agent coordination | GitHub Issues |
| Public visibility, review | GitHub Issues + PRs |

A ticket may reference an issue on any forge
(`Blocked-by: github.com/org/repo#435`) but never queries it.
The two systems are independent.

## Postel's Law

The validator enforces %erg v1 on commit (strict on write).

Agents may interpret non-conforming input, but must produce valid %erg v1
when creating or modifying tickets. Non-conforming files are rejected by the validator.
