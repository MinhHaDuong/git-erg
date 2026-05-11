# Ticket format spec — %erg v1

Author: Minh Ha-Duong <minh.ha-duong@cnrs.fr>
Last modified: 2026-05-08
Status: Working draft

## Introduction

`git-erg` is an agent-friendly local ticket system for development in disconnected environments. Tickets are committed to git and travel with the repo. This file is normative and defines the format for valid `%erg v1` files. Command documentation is available via `erg COMMAND --help` and `erg --help --all`.

Any divergence between this document and the `erg` binary must be resolved by aligning the specification with the behavior. Rationale is in `pep-erg-v1.md`.

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
| `Tag` | no | yes | enum | `needs-human`, `deferred`, `post-talk`, `post-conference` |

No other headers are valid in v1. No `X-` extensions.

`Tag:` is repeatable; each occurrence adds one tag value. The validator enforces the closed value set per occurrence; see the validate rules.

**`Closed:` header:** optional, non-repeatable, preamble only. Value is required and non-empty.
Forbidden in the log and body sections (header-key match at line start; substrings in prose are fine).
Example: `Closed: completed in PR #5`

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

Local refs (`0042`) point to tickets in `tickets/`. Forge refs (`github.com/owner/repo#N`) name
issues on any code forge. `erg` never makes network calls; unknown forge refs are blocking by default.
One `Blocked-by:` line per dependency; remove the line once the dependency is resolved.

There is no pending or doing header. If two agents need to avoid stepping on each other, they should
coordinate via out-of-band signals — typically a git branch whose name contains the ticket ID.

### Closed / not-closed criterion

A ticket is **closed** if at least one of these holds:

1. **Path test.** A path component (directory name or basename without extension) equals `closed`
   (case-insensitive), starts with `closed-` or `closed.`, or ends with `-closed`.
   Covers `tickets/closed/`, `0001-foo-closed.erg`. Rules out `disclosed`, `enclosed`.
2. **Header test.** A preamble line begins with `Closed:`
   (header-key match at line start; value required, non-empty).

Otherwise the ticket is **not-closed** (open). There is no other state.

`erg check` emits a corpus hygiene **warning** (non-fatal) when a ticket's folder placement and `Closed:` header disagree. This mismatch does not make the ticket invalid — the disjunctive criterion above is authoritative for the closed/not-closed decision.

### ID assignment

The ticket ID is derived from the filename, not a header.

Filename pattern: `{ID}-{slug}.erg`
- ID: zero-padded sequential number, 4 digits (`0001`, `0002`, …)
- Slug: lowercase kebab-case, ASCII only (`[a-z0-9-]`)
- Slug is truncated to 40 characters by `erg new`; trailing hyphens are stripped after truncation.

```sh
erg next-id tickets/
```

Returns the next available ID. If no valid ticket filenames are found, returns `0001`.

**Collision handling:** optimistic. Multiple worktrees may select the same ID. The pre-commit
validator rejects duplicates; the losing agent renames and retries.

### Log section

Append-only. Each line records one event:

```
{ISO-8601-timestamp} {actor} {verb} [{detail}]
```

**Timestamp:** `YYYY-MM-DDThh:mmZ` (UTC, minute precision).
**Actor:** agent or human identifier (e.g., `claude`, `user`).
**Verbs (open set, v1):** any single token. Suggested: `created`, `closed`, `note`.

Lines are never edited or deleted. To correct an error, append a new line.

### Body section

Free-form markdown. Suggested structure for actionable tickets:

```
## Context / ## Actions / ## Test / ## Exit criteria
```

Not enforced by the validator.

## Postel's Law

The validator enforces %erg v1 on commit (strict on write).

Agents may interpret non-conforming input, but must produce valid %erg v1
when creating or modifying tickets. Non-conforming files are rejected by the validator.
