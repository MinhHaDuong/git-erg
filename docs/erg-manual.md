# erg manual

Author: minh.ha-duong@cnrs.fr
Generated from: erg built 2026-05-28T11:48:11Z rev a12e6a2

`git-erg` is an agent-friendly local ticket system for development in disconnected
environments. Tickets are plain-text files committed alongside source code.
This manual describes all `erg` commands. For the ticket file format
specification, see `tickets/spec-erg-v1.md`.

**Store auto-discovery.** When no DIR is given, `erg` tries three candidates in
order: (1) the directory containing the `erg` binary, (2) `tickets/` under the
current working directory, (3) the current working directory itself. A directory
qualifies as a ticket store if its basename is `tickets`, or if it contains at
least one `.erg` file. The first qualifying candidate is used; if none qualify,
`erg` exits with an error listing the directories it tried.

## erg validate FILE...

Validate individual .erg ticket files (format, headers, refs).

Each FILE must be a .erg ticket. For every file the validator enforces:

  1. Magic first line is '%erg 0.1' (rejects unknown versions).
  2. All required headers present AND non-empty: Title, Created, Author.
  3. No unknown headers (Status: is unknown; run 'erg migrate' to convert it).
  4. Non-repeatable headers (Title, Created, Author, Closed) appear at most once.
  5. Tag: values are from the vocabulary (default: needs-human, deferred; see tickets/.ergrc [tags]).
  6. Closed: header has a non-empty value and does not appear in the log or body sections.
  7. Created is a valid ISO date (YYYY-MM-DD).
  8. Filename matches NNNN-slug.erg (4-digit ID, lowercase ASCII kebab slug).
  9. Blocked-by values parse as local-ref (NNNN, exactly 4 digits) or
     forge-ref (host/owner/repo#N, e.g. github.com/acme/myrepo#42).
  10. Local Blocked-by refs point to existing ticket IDs in the same directory.
  11. Log lines match 'YYYY-MM-DDThh:mmZ actor verb [detail]' format.
  12. Both separators (`--- log ---`, `--- body ---`) appear at least once;
      the first occurrence of each is the section separator, subsequent
      occurrences are body text (legitimate bodies may quote the literals).
  13. No dependency cycles among local Blocked-by refs.

Error format: 'filename:LINE: message' when a specific line applies
(rules 1-7, 9, 11); 'filename: message' when no line applies (rules 8, 12).
Line numbers are 1-indexed.

For corpus-level checks (duplicate IDs, cycles), use: erg check [dir]

Exit codes: 0 on pass, 1 on any violation. Directories are rejected — use erg check.

## erg check [DIR]

Corpus-level integrity checks across the full ticket store.

Unlike erg validate (which checks individual files), check loads all .erg files
under DIR recursively and verifies invariants that require a global view:

  - No duplicate ticket IDs across the corpus.
  - All Blocked-by local refs point to tickets that exist in the corpus.
  - No dependency cycles among Blocked-by edges.
  - All per-ticket format rules (delegates to validateCorpus, which folds in parser-emitted errors).

Additionally emits warnings (non-fatal) for:

  - Folder/header mismatch: open ticket in closed/ or closed ticket not in closed/.
  - Stray Go source files (*.go, go.mod, go.sum) inside the ticket store directory.
  - Interior header blank: a blank line inside the header block (tolerated on
    read; run 'erg migrate' to normalise).

Exit codes: 0 on pass (warnings are printed but do not affect exit code), 1 on any violation.

## erg list [DIR] [TAG...] [not TAG...] [--all] [--json]

List tickets, one per line, sorted by ID. Each line carries any [refs] —
git branches, remote-tracking branches, and worktree paths that reference the
ticket per the spec-erg-v1.md matching rule — plus (tags: …) and (blocked-by:
…) when present. The refs scan is local-only (git for-each-ref, git worktree
list); no network calls.

Tag arguments filter the list as a conjunction: a bare TAG keeps only tickets
carrying it, and "not TAG" drops tickets carrying it. Beyond the literal Tag:
vocabulary, three computed pseudo-tags are accepted:

  - closed   — the ticket is closed (Closed: header or closed/ path).
  - open     — the ticket is not closed.
  - blocked  — the ticket has an unsatisfied blocker (a forge ref, or a
               Blocked-by pointing at an open local ticket).

Open is the default: with no open/closed term and without --all, only open
tickets are shown. --all drops that default so closed tickets appear too
(marked [closed]). Tickets are sorted by ID ascending.

DIR selects the ticket store: an argument naming an existing directory (or one
containing '/'), e.g. 'erg ls tickets/'. The pseudo-tags closed/open/blocked are
always filter terms, so 'erg ls closed' lists closed tickets even from inside a
store that contains a closed/ directory.

Without --json, prints a human-readable line per ticket. With --json, prints a
JSON array where each element has the fields: id, title, file, closed, refs,
tags, blocked_by.

Alias: erg ls.

Examples:
  erg ls                      open tickets
  erg ls needs-human          open tickets tagged needs-human
  erg ls not deferred         open tickets not tagged deferred
  erg ls closed               closed tickets
  erg ls --all blocked        all blocked tickets, open or closed

## erg ready [DIR] [--json]

List tickets ready for work — a saved filter over 'erg list'.

A ticket is ready when all of the following hold:

  - Open (not closed).
  - Not blocked: no Blocked-by pointing at an open local ticket, and no
    forge-ref Blocked-by (forge refs are offline-unknown, treated as
    blocking).
  - Carries none of the skip tags (default: needs-human, deferred;
    configurable via tickets/.ergrc [tags]).

Equivalent to 'erg list open not blocked not needs-human not deferred', and
shares its output: a human-readable line per ticket, or --json for a JSON
array with the fields id, title, file, closed, refs, tags, blocked_by.

Each line is annotated with the comma-separated [refs] — git branch short
names, remote-tracking branch short names (with their <remote>/ prefix),
and worktree paths — that reference the ticket per spec-erg-v1.md. The scan
is local-only; PRs and forge state are out of scope (pep-erg-v1.md §7).

## erg next-id [DIR]

Print the next available ticket ID.

Scans DIR (default: auto-discovered tickets/) and all subdirectories for .erg
files, extracts the leading 4-digit numeric prefix from each filename, and
returns the maximum found plus one, zero-padded to 4 digits. Prints "0001" if
no numbered tickets exist or the directory does not exist.

The scan is local to the working directory; other branches, worktrees, and remotes
are not consulted. ID allocation is optimistic: two concurrent invocations may
return the same ID. The pre-commit hook rejects duplicate IDs; the losing agent
renames its ticket with a new ID from a fresh invocation.

## erg new TITLE [DIR]

Create a new %erg 0.1 ticket file atomically.

Allocates the next available ID by scanning DIR (default: auto-discovered tickets/)
for the highest numeric .erg filename prefix, then creates a file named
NNNN-{slug}.erg where the slug is the title lowercased and kebab-cased (truncated
to 40 characters). Uses O_EXCL to prevent races with concurrent invocations.

The new file contains the required preamble headers (Title, Created, Author),
an empty log section with a "created" entry, and an empty body section.
Author is resolved from the ERG_AUTHOR environment variable, or the git user.name,
or the system username — whichever is available first.

Prints 'CREATED NNNN-slug.erg' on success. Exits non-zero on I/O errors.

## erg close ID|FILE REASON [DIR]

Atomically close a ticket.

Closing a ticket is a three-step operation:

  1. Inserts a Closed: REASON header at the end of the preamble (before `--- log ---`).
  2. Appends a timestamped log line: `TIMESTAMP AUTHOR closed — REASON`.
  3. Scans every open ticket in DIR for Blocked-by: ID and removes those lines,
     appending a log entry to each modified ticket:
     `TIMESTAMP AUTHOR note blocker ID closed — Blocked-by removed.`
     Already-closed tickets that reference the ID are not modified. If a ticket
     has multiple Blocked-by: ID lines, all are removed in one pass.
     Step 3 iterates all open tickets; it is idempotent but not atomic.

ID may be a 4-digit ticket ID or a full filename (e.g. 0042-some-title.erg).
REASON must be non-empty. The operation is idempotent (safe to call twice for
the same ticket): closing an already-closed ticket prints 'CLOSED (already)' and
exits 0. Step 3 (Blocked-by removal) is also idempotent; re-running close on
an already-closed ticket does not re-scan dependents.

## erg log ID LINE [DIR]

Append a timestamped entry to a ticket's log section.

Resolves the ticket by 4-digit ID in DIR (default: auto-discovered tickets/), then
prepends the current UTC timestamp (YYYY-MM-DDThh:mmZ) to LINE and inserts the
resulting line at the end of the log section, just before the `--- body ---` separator.

The resulting log entry format is:

  `YYYY-MM-DDThh:mmZ LINE`

LINE must be non-empty. It must follow the format 'actor verb [detail]'
(e.g. "claude note retried with narrower scope"). The timestamp, actor, and verb
tokens are required; the detail token is optional. The log-line format is
enforced by erg validate (rule 11).

Prints "LOGGED" on success. Exits non-zero if the ticket is not found or has no
`--- body ---` separator (which would indicate a malformed file).

## erg tag ID TAGNAME [DIR]

Add a Tag: header to the ticket's preamble and append a log line.

The tag value must be in the project vocabulary (tickets/.ergrc [tags]
section; default: needs-human, deferred). If the ticket already has the
tag, prints "TAGGED (already)" and exits 0 without modifying the file.

Exits non-zero if the tag is not in the vocabulary or the ticket is not found.

## erg untag ID TAGNAME [DIR]

Remove a Tag: header from the ticket's preamble and append a log line.

The tag value must be in the project vocabulary. If the ticket does not
have the tag, prints "NOT TAGGED" and exits 0 without modifying the file.

Exits non-zero if the tag is not in the vocabulary or the ticket is not found.

## erg archive [ID...] [DIR]

Move closed tickets to DIR/closed/.

With no IDs, scans only the direct children of DIR (default: tickets/) — not subdirectories — for tickets that
have a non-empty Closed: header and are not already inside a closed/ directory,
then moves each eligible ticket to DIR/closed/. With IDs given, archives only
the named tickets.

A ticket is skipped (with a SKIPPED message) if any open ticket in DIR still
has a Blocked-by: pointing to its ID; archiving would silently break that ref.
Run 'erg close ID REASON' (which removes Blocked-by refs automatically) before
archiving, or manually delete the stale Blocked-by line.

The command creates DIR/closed/ if it does not exist. It will not overwrite
an existing file at the destination.

## erg migrate [DIR]

Convert legacy headers to %erg 0.1 format.

Idempotent (safe to run repeatedly: already-migrated files are not modified twice). For every .erg file under DIR (default: tickets/) the migration
rules are:

  - 'Status: closed' (case-insensitive) → drop the line; append
    'Closed: migrated from Status: closed' to the preamble.
  - 'Status: open', 'Status: doing', or 'Status: pending' → drop the line;
    the ticket becomes not-closed (the correct new state).
  - 'Tags:' preamble line → rewrite the key to 'Tag:' (singular; the header is
    repeatable and singular names are the v1 convention). The value is preserved.
  - Legacy '%erg v1' magic line → rewritten to '%erg 0.1'.
  - Interior blank lines inside the header block → swept (ticket 0141:
    accept on read, autofix on write). The first blank line still terminates
    the header block; only blanks between header lines are removed.
  - No legacy line and no interior blanks → no-op.

After migration, erg validate will reject any remaining Status: or Tags: lines.

When DIR is named "tickets" (the canonical layout), also performs a one-time
project layout upgrade: removes tickets/tools/ and tickets/FORMAT.md if present,
renames archive/ to closed/ if archive/ exists and closed/ does not, then
refreshes init assets via cmdInit.

Does NOT commit. Exits 1 on archive/→closed/ filename collision (both directories are left untouched; the user must resolve manually). Exits 0 otherwise.
Review the diff with 'git diff tickets/' and commit manually.

## erg init [DIR]

Unpack embedded bootstrap assets into the project.

Writes (or refreshes) four files relative to DIR (default: current directory):

  - tickets/.ergrc — project configuration (tag vocabulary, update URL).
  - tickets/AGENTS.md — agent operating instructions for the ticket workflow.
  - tickets/spec-erg-v1.md — the %erg 0.1 format specification.
  - tickets/integration.md — setup guide for the pre-commit hook and CI integration.

Requires tickets/erg (the binary) to already exist in the project; the command
refuses if it is absent. This requirement ensures that agents do not accidentally
initialize an empty directory that was never meant to be a ticket store.

Each asset is compared byte-for-byte with the embedded version; unchanged files
are skipped and counted separately from newly created or refreshed files.

## erg version

Print self-diagnostic info and discover other erg binaries.

Prints the following fields for the running binary:

  - path:     resolved absolute path (symlinks followed).
  - hash:     first 12 hex characters of the SHA-256 of the binary file.
  - built:    build date injected at compile time via -ldflags (or "[unknown]").
  - revision: VCS commit hash injected at compile time via -ldflags (if present).
  - arch:     GOOS/GOARCH of the running binary.

After printing the running binary info, `erg version` discovers other erg binaries
in well-known locations (./build/erg, ./tickets/erg, ~/.local/bin/erg, and PATH
entries), compares VCS revisions and build dates against each discovered copy, and
prints the update command for any outdated copy it finds.

Set ERG_VERSION_NO_DISCOVER=1 to suppress discovery (used internally by version
comparison to avoid recursion).

## erg update

Fetch the upstream binary and replace this executable atomically.

Downloads the binary from ERG_UPDATE_URL (default: the main branch of the
upstream GitHub repo). If the downloaded hash matches the running binary,
prints "already up to date" and exits 0. Otherwise replaces the binary via
an atomic rename (write to .tmp, then rename over self).

Network and HTTP errors exit 0 so that 'erg update && erg validate' chains
do not fail in offline or isolated environments.

After a successful update, checks whether any .erg files in the ticket store
still carry legacy Status: headers. If found, prints explicit migration
guidance: 'erg migrate DIR', 'git diff tickets/', 'git commit'. The update
command never mutates ticket files itself — migration is a separate, reviewable step.
