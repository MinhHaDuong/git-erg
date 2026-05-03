# Ticket format spec — %erg v1

## Overview

Local ticket system for agent coordination across worktrees on one machine.
Not a replacement for GitHub Issues — those handle inter-agent and human coordination.
Tickets are committed to git and travel with the repo.

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
| `Blocked-by` | no | yes | ref | Local `NNNN`, `gh#N`, or `gh:owner/repo#N` (see grammar) |

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
  - `Closed: gh#42 — superseded by 0099`
  - `Closed: abandoned — out of scope`

**`Blocked-by` references** take one of three forms:

```
ref       := local-ref | gh-same | gh-cross
local-ref := [0-9]{4}
gh-same   := "gh#" number
gh-cross  := "gh:" owner "/" repo "#" number
number    := [1-9][0-9]*
```

- **Local** — `0042` refers to a ticket in this `tickets/` directory.
- **Same-repo GitHub** — `gh#N` refers to issue N in the GitHub
  project hosting the repo that contains this `.erg` file. The bare
  form is shorthand: it follows the repo. **If the repo is forked or
  vendored, `gh#N` re-points to the fork's issue tracker.** That's
  usually what you want for "the bug we filed about ourselves." When
  it isn't, use the explicit cross-repo form.
- **Cross-repo GitHub** — `gh:owner/repo#N` names a specific upstream
  issue regardless of fork. Use this form when the dependency must
  follow the original repository (third-party libraries, sibling
  services, anything outside this repo).

`#` is reserved as the issue-number separator; `:` is the scheme
separator. `gh` is the only scheme defined in v1; future hosts
(`gl:`, `gh:host/...`) extend the grammar additively.

**Owner and repo rules** mirror GitHub's own validation. The
validator rejects refs that GitHub would reject:

- `owner` (login) — `[A-Za-z0-9](?:[A-Za-z0-9]|-(?=[A-Za-z0-9]))*`,
  max 39 characters, no leading or trailing `-`, no underscores.
- `repo` — `[A-Za-z0-9._-]+`, max 100 characters, may not be `.` or
  `..`, may not start with `.` or `-`, may not contain `..`.

Schemes are case-sensitive: only literal `gh#` and `gh:` are
accepted. `GH#`, `Gh:`, etc. are rejected. The number must be a
positive integer with no leading zero.

Repeatable: one `Blocked-by:` line per dependency. A local blocker
must be **closed** (per the criterion below) to unblock. GitHub refs
are resolved out-of-band; the validator parses syntax only, never
reaches the network, and a malformed `gh:` ref fails at
`erg validate` rather than at runtime.

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
- ID: zero-padded sequential number, 4 digits. `0001`, `0002`, ...
- Slug: lowercase kebab-case, ASCII only (`[a-z0-9-]`).

To assign the next ID: read filenames in `tickets/`, extract the numeric
prefix from each, take the maximum, increment by 1, zero-pad to 4 digits.
If no tickets exist, start at `0001`.

**Collision handling:** optimistic. Two worktrees may pick the same number.
The pre-commit validator catches duplicate IDs. The agent that loses renames
its ticket (increment again). This matches git's own optimistic concurrency.

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
- Every GitHub ref (`gh#N` or `gh:owner/repo#N`) is treated as
  satisfied for the offline `erg ready` query. (Resolution against
  the GitHub API is a runtime concern handled by the resolver, not
  a property of this format.)

## Validator rules (pre-commit)

The Go validator enforces:
1. Magic first line is `%erg v1` (reject unknown versions).
2. All required headers present (`Title`, `Created`, `Author`).
3. No unknown headers. `Status:` is unknown — `erg migrate` is the one
   command that tolerates it (in order to convert it).
4. `Created` is a valid ISO date (`YYYY-MM-DD`).
5. Filename matches `NNNN-{slug}.erg` pattern (4-digit ID, ASCII slug).
6. No duplicate IDs within `tickets/`.
7. `Blocked-by` values parse as one of `local-ref`, `gh-same`, or
   `gh-cross` (see grammar above). Malformed refs are rejected with
   a precise message identifying the failure mode.
8. `Blocked-by` local refs point to existing ticket IDs.
9. No dependency cycles. Cross-repo refs are terminal from this
   repo's view and cannot participate in local cycles.
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

`erg update` invokes `erg migrate` automatically when it detects
`Status:` lines after a successful binary swap.

## Relationship to GitHub Issues

| Concern | Tool |
|---------|------|
| Local work organization | `.erg` files |
| Multi-agent coordination | GitHub Issues |
| Public visibility, review | GitHub Issues + PRs |

A ticket may reference a GitHub issue (`Blocked-by: gh#435`) but never
caches it. The two systems are independent.

## Postel's Law

**Strict on write, tolerant on read.** The validator enforces `%erg v1`
on commit. But you — the agent — are the parser for arbitrary input. If you
receive ticket-like information in any form (raw JSON from `gh`, a sentence,
a markdown sketch), understand the intent and write clean `%erg v1`. The
pre-commit hook catches mistakes. The tolerance is in you, not the tooling.
