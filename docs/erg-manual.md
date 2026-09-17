# erg manual

Author: minh.ha-duong@cnrs.fr
Generated from: erg

`git-erg` is an agent-friendly local ticket system for development in disconnected
environments. Tickets are plain-text files committed alongside source code.
This manual describes all `erg` commands. For the ticket file format
specification, run `erg spec`.

**Store auto-discovery.** When no DIR is given, `erg` tries three candidates in
order: (1) the directory containing the `erg` binary, (2) `tickets/` under the
current working directory, (3) the current working directory itself. A directory
qualifies as a ticket store if its basename is `tickets`, or if it contains at
least one `.erg` file. The first qualifying candidate is used; if none qualify,
`erg` exits with an error listing the directories it tried.

When the store is auto-discovered, `erg` refuses to use a store that lies in
a different git worktree than the working directory. Pass DIR explicitly to override.

**Exit codes (shared by `check` and `init`).** `0` success;
`1` a hard error (bad flag, unreadable directory, write failure, or a
corpus violation); `2` a file was preserved and skipped, having either
local edits or a stamp newer than the running binary
(`init` only -- run with `--force` to overwrite). Any non-zero
status is a failure for scripting purposes. The value `1` always means a
hard failure -- it never doubles as "skipped".

## erg validate FILE...

Validate individual .erg ticket files (format, headers, refs).

Each FILE must be a .erg ticket. For every file the validator enforces:

  1. Magic first line is '%erg 0.1' (rejects unknown versions).
  2. All required headers present AND non-empty: Title, Created, Author.
  3. No unknown headers (Status: is unknown; run 'erg migrate' to convert it).
  4. Non-repeatable headers (Title, Created, Author, Closed) appear at most once.
  5. Label: values are from the vocabulary (default: needs-human, deferred; see tickets/.ergrc [labels]).
  6. Closed: header has a non-empty value and does not appear in the log or body sections.
  7. Created is a valid ISO date (YYYY-MM-DD).
  8. Filename matches NNNN-slug.erg (4-digit ID, lowercase ASCII kebab slug).
  9. Blocked-by values parse as a URI-reference (RFC 3986): a local NNNN, a
     relative path-ref (auth/0042), or an absolute URI (https://...). Only a
     malformed URI-reference (a space or control character) is rejected.
  10. Local Blocked-by refs point to existing ticket IDs in the same directory.
  11. Log lines match structural format: timestamp (YYYY-MM-DDThh:mmZ)
      followed by at least two whitespace-separated tokens. By convention
      these are 'actor verb [detail]', but the validator checks structure,
      not the semantic meaning of those tokens.
  12. Both separators (`--- log ---`, `--- body ---`) appear at least once;
      the first occurrence of each is the section separator, subsequent
      occurrences are body text (legitimate bodies may quote the literals).
  13. No dependency cycles among local Blocked-by refs.
  14. Title does not begin or end with a status word (ready, done, closed,
      open) -- these read as a status assertion about the ticket rather than
      the thing being changed. Enforced on open tickets; closed tickets are
      grandfathered (existing closed history is never flagged).
  15. Superseded-by values parse as a URI-reference -- same grammar as
      Blocked-by. Local
      refs must point to existing ticket IDs. Self-reference is an error.
      Repeatable (one-to-many supersession). Carried by the CLOSED ticket,
      pointing at the ticket(s) that replace it; it is durable lineage and is
      never stripped on close.

Error format: 'filename:LINE: message' when a specific line applies
(rules 1-7, 9, 11, 14, 15 self-ref); 'filename: message' when no line applies (rules 8, 12, 10, 15 unknown-ref).
Line numbers are 1-indexed.

For corpus-level checks (duplicate IDs, cycles), use: erg check [dir]

Exit codes: 0 on pass, 1 on any violation. Directories are rejected -- use erg check.

## erg check [DIR]

Corpus-level integrity checks across the full ticket store.

Unlike erg validate (which checks individual files), check loads all .erg files
under DIR recursively and verifies invariants that require a global view:

  - No duplicate ticket IDs across the corpus.
  - All Blocked-by local refs point to tickets that exist in the corpus.
  - tickets/AGENTS.md is not locally edited. erg ships and upgrades that file,
    so a local edit is lost at the next init; put project-specific lore in
    tickets/LOCAL.md instead, which erg never touches. Only a store whose
    .erg-assets stamp records what init wrote can be checked this way -- an
    unstamped store gets the "cannot tell" note instead, not this error.
  - All Superseded-by local refs point to tickets that exist in the corpus.
  - No dependency cycles among Blocked-by edges.
  - All per-ticket format rules (delegates to validateCorpus, which folds in parser-emitted errors).

  - Folder/header closure: open ticket in closed/ or closed ticket not in
    closed/ (a hand-edited Closed: header that was not filed -- erg close now
    files in one step; run 'erg close ID' or 'erg archive'). Also a ticket
    outside closed/ whose *filename* reads as closed (NNNN-...-closed.erg)
    but carries no Closed: header -- rename it or add the header.

Additionally emits warnings (non-fatal) for:

  - Open Superseded-by carrier: an open ticket carries a Superseded-by header
    (the normal pattern is for the closed ticket to carry it; close the old
    ticket or remove the header).
  - Stray Go source files (*.go, go.mod, go.sum) inside the ticket store directory.
  - Interior header blank: a blank line inside the header block (tolerated on
    read; run 'erg migrate' to normalise).
  - Asset drift: the .erg-assets stamp differs from this binary's embedded
    asset. The message names the direction, because the remedy differs and one
    of the two would destroy data if applied to the other:
      - this binary is NEWER than the stamp (upgraded since the last init):
        refreshing is an upgrade; run 'erg init' to refresh.
      - this binary is OLDER than the stamp (it predates the last init):
        refreshing would REVERT the deployed assets; run 'erg sync' first,
        then 'erg init'.
    Requires a stamp FOR THAT ASSET: the comparison is stamp against embedded.
    An asset no stamp covers gets the stampless NOTE below instead. A stamp with
    no comparable date carries no direction and is reported as the upgrade case,
    which is the pre-0279 behaviour.
  - Stampless asset (NOTE, not WARN): no .erg-assets entry usably stamps this
    asset -- the store has no manifest at all, or the manifest it has says
    nothing about this file, or it carries an entry that is not a usable hash
    (truncated mid-line, hand-edited) -- yet the asset on disk differs from this
    binary's embedded copy. A manifest that covers only some assets is a normal
    state, not a corrupted one: erg init stamps what it installed and leaves
    an asset it preserved alone, so a store with one customised asset ends up
    exactly there. The difference is real but unattributable -- with no stamp, nothing
    records whether it is an upgrade this store never stamped or a deliberate
    local edit -- so no direction is claimed and no overwrite is prescribed.
    Run 'erg init --show NAME' to see the copy this binary ships. Silent
    when the asset is absent (a store that never adopted erg's asset management
    is not nagged) and silent when it matches the embedded copy exactly.
    NOTE marks the weaker class: a condition reported, not a repair advised.
    Both classes are counted together in the trailing "N warnings" summary.
  - Vendored drift (NOTE, not WARN): tickets/erg-github on disk differs from
    the copy this binary ships. That file is vendored, not installed -- it
    travels with the clone and the adopter owns it -- so erg compares and
    reports, and never writes it. No direction is claimed (the copy may be an
    older upstream one OR your own customisation) and no .erg-assets stamp is
    consulted, since init never wrote the file and the stamp says nothing about
    it. The remedy is manual: re-vendor it (see README, "Forge layer:
    erg-github"). Silent when the file is absent -- the forge layer is
    optional, and a repo that never adopted it is never nagged into it.

Exit codes: 0 on pass (warnings are printed but do not affect exit code), 1 on any
violation. The value 1 is a hard failure here, consistent with the shared exit-code
table (see "Exit codes" in erg --help --all); check never returns 2.

## erg list [DIR] [LABEL...] [not LABEL...] [--all] [--json]

List tickets, one per line, sorted by ID. Each line carries any [refs] --
git branches, remote-tracking branches, and worktree paths that reference the
ticket per the spec-erg-v1.md matching rule -- plus (labels: ...) and (blocked-by:
...) when present. The refs scan is local-only (git for-each-ref, git worktree
list); no network calls.

Label arguments filter the list as a conjunction: a bare LABEL keeps only tickets
carrying it, and "not LABEL" drops tickets carrying it. Beyond the literal Label:
vocabulary, three computed pseudo-labels are accepted:

  - closed   -- the ticket is closed (Closed: header or closed/ path).
  - open     -- the ticket is not closed.
  - blocked  -- the ticket has a Blocked-by that resolves to an open ticket
               (a local NNNN, or an open sibling path-ref); an unresolved
               reference only warns, it does not block.

Open is the default: with no open/closed term and without --all, only open
tickets are shown. --all drops that default so closed tickets appear too
(marked [closed]). Tickets are sorted by ID ascending.

DIR selects the ticket store: an argument naming an existing directory (or one
containing '/'), e.g. 'erg ls tickets/'. The pseudo-labels closed/open/blocked are
always filter terms, so 'erg ls closed' lists closed tickets even from inside a
store that contains a closed/ directory.

Without --json, prints a human-readable line per ticket. With --json, prints a
JSON array where each element has the fields: id, title, file, closed, refs,
labels, blocked_by.

Alias: erg ls.

Examples:
  erg ls                      open tickets
  erg ls needs-human          open tickets labeled needs-human
  erg ls not deferred         open tickets not labeled deferred
  erg ls closed               closed tickets
  erg ls --all blocked        all blocked tickets, open or closed

## erg ready [DIR] [--json]

List tickets ready for work -- a saved filter over 'erg list'.

A ticket is ready when all of the following hold:

  - Open (not closed).
  - Not blocked: no Blocked-by that resolves to an open ticket (a local NNNN,
    or a relative path-ref to an open sibling). An unresolved reference -- an
    absolute URI, or a path not present in this checkout -- is optimistic: it
    warns, it does not block.
  - Carries none of the skip labels (default: needs-human, deferred;
    configurable via tickets/.ergrc [labels]).

Equivalent to 'erg list open not blocked' with every configured label
(.ergrc [labels]; default: needs-human, deferred) also negated. Shares
its output: a human-readable line per ticket, or --json for a JSON array
with the fields id, title, file, closed, refs, labels, blocked_by.

Each line is annotated with the comma-separated [refs] -- git branch short
names, remote-tracking branch short names (with their <remote>/ prefix),
and worktree paths -- that reference the ticket per spec-erg-v1.md. The scan
is local-only; PRs and forge state are out of scope (pep-erg-v1.md sec.7).

## erg next-id [DIR]

Print the next available ticket ID.

Scans for the maximum ticket ID across three sources and returns max+1,
zero-padded to 4 digits. Prints "0001" if no numbered tickets exist anywhere.

  1. DIR (default: auto-discovered tickets/) and its subdirectories -- the
     local filesystem walk.
  2. The same-relative subdir of every sibling worktree, enumerated via
     'git worktree list'. Catches uncommitted tickets drafted in parallel
     agent worktrees of the same repository.
  3. The same-relative subtree of every refs/heads/ and refs/remotes/ tip
     in the local refs cache, enumerated via 'git for-each-ref' + 'git
     ls-tree'. Catches tickets committed on branches not currently checked
     out anywhere, and IDs already burned on origin that have been fetched
     but not yet merged locally. No network call -- remote-tracking refs
     come from the local cache populated by the last 'git fetch'. Bounded
     by a 200ms wall-clock deadline; on timeout, falls back to the Pass
     1+2 result and prints a WARNING to stderr.

When DIR is outside a git repository, or git is unavailable, behavior
reduces to the Pass 1 local walk alone.

Cache freshness: the remote-tracking scan is only as fresh as the last
'git fetch'. If parallel agents push tickets to origin between fetches,
their IDs are invisible to this scan and may be re-allocated. Run
'git fetch' before starting a parallel raid if you want the freshest
view; next-id itself never makes a network call.

ID allocation is still optimistic: two concurrent invocations in different
worktrees may return the same ID -- the cross-worktree window has narrowed
but is not eliminated. The pre-commit hook rejects duplicate IDs on merge;
the losing agent renames its ticket with a new ID from a fresh invocation.

## erg new TITLE [DIR] [--author NAME]

Create a new %erg 0.1 ticket file atomically.

Allocates the next available ID by scanning DIR (default: auto-discovered tickets/)
for the highest numeric .erg filename prefix, then creates a file named
NNNN-{slug}.erg where the slug is the title lowercased and kebab-cased (truncated
to 40 characters).

Uses an optimistic post-check retry loop to handle concurrent invocations:
O_EXCL writes the file, then a glob for NNNN-*.erg verifies uniqueness of the
NNNN prefix. If a collision is detected (two concurrent invocations computed the
same ID for different slugs), the losing invocation removes its file and retries
with the next free ID. Up to 20 attempts are made before giving up.

The new file contains the required preamble headers (Title, Created, Author),
an empty log section with a "created" entry, and an empty body section.

  --author NAME, -a NAME
      Override the Author header with NAME. If not given, author is resolved
      from the ERG_AUTHOR environment variable, or the git user.name, or the
      system username -- whichever is available first. NAME may not be empty
      or whitespace-only. Newlines and carriage returns are stripped.

Prints 'CREATED NNNN-slug.erg' on success. Exits non-zero on exhaustion or I/O errors.

## erg close ID|FILE REASON [DIR]

Atomically close a ticket.

Closing a ticket is a four-step operation:

  1. Inserts a Closed: REASON header at the end of the preamble (before `--- log ---`).
  2. Appends a timestamped log line: `TIMESTAMP AUTHOR closed — REASON`.
  3. Scans every open ticket in DIR for Blocked-by: ID and removes those lines,
     appending a log entry to each modified ticket:
     `TIMESTAMP AUTHOR note blocker ID closed — Blocked-by removed.`
     Already-closed tickets that reference the ID are not modified. If a ticket
     has multiple Blocked-by: ID lines, all are removed in one pass.
     Step 3 iterates all open tickets; it is idempotent but not atomic.
  4. Moves the closed ticket into DIR/closed/, so closing files the ticket in
     one step -- no separate `erg archive` -- and a closed ticket has a single
     terminal location. A ticket that is already closed but still at top-level
     (hand-closed, or a close interrupted before the move) is filed by re-running
     close. The move is durable and confined to the store.

ID may be a 4-digit ticket ID or a full filename (e.g. 0042-some-title.erg).
REASON must be non-empty. A REASON that begins with '-' (or is literally
'--help') must follow a '--' end-of-options marker, e.g.
`erg close 0042 -- "-- superseded by 0050"`.

The operation is idempotent (safe to call twice): once the ticket is filed
under closed/ AND carries the Closed: header, close prints 'CLOSED (already)'
and exits 0. A ticket that is closed by path but still missing the header gets
the header (and REASON) written, so a supplied reason is never silently
dropped. Step 3 (Blocked-by removal) is also idempotent.

## erg log ID LINE [DIR] [--author NAME]

Append a timestamped entry to a ticket's log section.

Resolves the ticket by 4-digit ID in DIR (default: auto-discovered tickets/), then
prepends the current UTC timestamp (YYYY-MM-DDThh:mmZ) AND the resolved author to
LINE, and inserts the resulting line at the end of the log section, just before
the `--- body ---` separator.

The resulting log entry format is:

  `YYYY-MM-DDThh:mmZ AUTHOR LINE`

So LINE supplies `VERB [detail]` -- NOT the author. A bare verb is a
complete entry ("reopened"); LINE must simply be non-empty.

The author is resolved exactly as `erg new` resolves it: --author NAME wins,
else $ERG_AUTHOR, else git config user.name, else $USER, else "unknown".

CONTRACT CHANGE (ticket 0276). Before this release LINE carried the actor too,
and nothing checked that it did: `erg log ID "note fixed it"` wrote
`<ts> note fixed it`, putting a verb in the actor slot. That could not be
validated after the fact -- the format is positionally ambiguous, so
`<ts> A B ...` parses whether A is an actor or a verb, and erg validate
(rule 11) passed such lines. The actor is now supplied rather than policed, which
removes the failure mode at its source and needs no verb vocabulary.

Callers written against the old contract must drop the actor from LINE, or pass
it as --author. Passing it in LINE now doubles it.

Prints "LOGGED" on success. Exits non-zero if the ticket is not found or has no
`--- body ---` separator (which would indicate a malformed file).

## erg label ID LABELNAME [DIR]

Add a Label: header to the ticket's preamble and append a log line.

The label value must be in the project vocabulary (tickets/.ergrc [labels]
section; default: needs-human, deferred). If the ticket already has the
label, prints "LABELED (already)" and exits 0 without modifying the file.

Exits non-zero if the label is not in the vocabulary or the ticket is not found.

## erg unlabel ID LABELNAME [DIR]

Remove a Label: header from the ticket's preamble and append a log line.

The label value must be in the project vocabulary. If the ticket does not
have the label, prints "NOT LABELED" and exits 0 without modifying the file.

Exits non-zero if the label is not in the vocabulary or the ticket is not found.

## erg archive [ID...] [DIR] [-n|--dry-run]

Move closed tickets to DIR/closed/.

With no IDs, scans only the direct children of DIR (default: tickets/) -- not subdirectories -- for tickets that
have a non-empty Closed: header and are not already inside a closed/ directory,
then moves each eligible ticket to DIR/closed/. With IDs given, archives only
the named tickets.

A ticket is skipped (with a SKIPPED message) if any open ticket in DIR still
has a Blocked-by: pointing to its ID; archiving would silently break that ref.
Run 'erg close ID REASON' (which removes Blocked-by refs automatically) before
archiving, or manually delete the stale Blocked-by line.

The command creates DIR/closed/ if it does not exist. It will not overwrite
an existing file at the destination: a collision is a real ID conflict, so
archive reports it, leaves the source in place, and exits non-zero (rename one
of the two tickets to resolve it).

With -n / --dry-run, archive renames nothing: it prints "WOULD ARCHIVE <file>"
for each eligible ticket and "WOULD SKIP <file> (needed by ...)" for tickets
held open by a Blocked-by ref, then exits 0. This is the read-only listing the
pre-push hook (erg install --push-hook) uses to warn about closed-but-
unarchived tickets without mutating the working tree.

## erg rm ID|FILE [DIR] [--force]

Delete a ticket file outright -- no Closed: header, no archive, no record.

Use rm only for tickets that should never have existed: a duplicate, a
typo-titled file, a fat-fingered draft, spam. For work that was done or
abandoned with history worth keeping, use 'erg close' (keeps the file, adds
a Closed: header) or 'erg archive' (moves it under closed/). Only rm removes
the record entirely.

Deletion is destructive and irreversible from the tool's side, so rm verifies
the dependency graph before touching the filesystem:

  - By default, if any ticket in the corpus (open OR closed) has a Blocked-by:
    referencing the target ID, rm refuses: it prints each dependent and exits
    non-zero WITHOUT deleting anything. The closed tickets are scanned too -- a
    closed ticket may carry a historical Blocked-by: line, and deleting its
    blocker would leave a dangling ref that 'erg check' flags.
  - With --force, rm deletes the target and strips the now-dangling Blocked-by:
    lines from every dependent (open or closed), appending a log entry to each:
    `TIMESTAMP AUTHOR note blocker ID removed — ticket deleted.`

ID may be a 4-digit ticket ID or a full filename (e.g. 0042-some-title.erg).
A non-existent or ambiguous ID is reported with the usual resolver error.

## erg migrate [DIR]

Convert legacy headers to %erg 0.1 format.

Idempotent (safe to run repeatedly: already-migrated files are not modified twice). For every .erg file under DIR (default: tickets/) the migration
rules are:

  - 'Status: closed' (case-insensitive) -> drop the line; append
    'Closed: migrated from Status: closed' to the preamble.
  - 'Status: open', 'Status: doing', or 'Status: pending' -> drop the line;
    the ticket becomes not-closed (the correct new state).
  - 'Tag:' (or legacy 'Tags:') preamble line -> rewrite the key to 'Label:'. The
    value is preserved; legacy 'Tags:' converges to 'Label:' in a single run.
  - '.ergrc' '[tags]' section header -> rewritten to '[labels]'.
  - Legacy '%erg v1' magic line -> rewritten to '%erg 0.1'.
  - Interior blank lines inside the header block -> swept (ticket 0141:
    accept on read, autofix on write). The first blank line still terminates
    the header block; only blanks between header lines are removed.
  - Log continuation lines: any non-blank line in the log section that does
    not start with a YYYY-MM-DD timestamp is joined (single space, stripped)
    onto the preceding log entry. Blank lines between an entry and its
    continuation content are dropped. Content before the first timestamped
    entry is untouched.
  - Date-only log stamps: a leading 'YYYY-MM-DD ' (date, no T separator) is
    rewritten to 'YYYY-MM-DDT00:00Z '.
  - No legacy line and no interior blanks -> no-op.

After migration, erg validate will reject any remaining Status:, Tags:, or Tag: lines,
and folds legacy wrapped log details plus date-only log stamps so a migrated store
passes validation (orphan content before the first log entry is left for validate to flag).

When DIR is named "tickets" (the canonical layout), also performs a one-time
project layout upgrade: removes tickets/tools/ and tickets/FORMAT.md if present,
renames archive/ to closed/ if archive/ exists and closed/ does not, refreshes
tickets/AGENTS.md (force-overwrite, no prompt -- agent docs track the binary;
.ergrc is configuration, delivered by 'erg init', so run 'erg sync && erg
init' to refresh it with the dpkg 3-state rule, which preserves a file for
either of two reasons: it has local edits, or it matches an .erg-assets stamp
newer than this binary -- see 'erg init --help'). Because .ergrc is outside
this command's reach, migrate never stamps it: whatever .erg-assets recorded
for it is carried forward untouched. Where .ergrc still holds the exact bytes
a NEWER binary's stamp records, the manifest is left entirely as it stands, so
'erg check' can still tell you the store is ahead of the binary that just swept
it. The price is that such a sweep records nothing about the AGENTS.md it did
write, so erg check reports that file as ahead too when it is no longer: one
header cannot date two assets separately, and the write was announced with an
undo hint when it happened. Run sync, then init from the corresponding updated
traveling or native binary, to clear it. It also
rewrites .git/hooks/pre-commit if it references
the legacy tickets/tools/go/erg path or the legacy 'validate tickets/' CLI
form. The hook rewrite is content-based and idempotent; hooks without legacy
patterns are left untouched.

The layout upgrade also cleans up artifacts vendored by early (pre spec 0.1)
erg init bundles (ticket 0243):

  - Removes previously-vendored .claude/skills/ticket-* dirs, content-gated:
    a dir is deleted only when its files carry the vendored tell-tales
    ('%erg v1', tickets/tools/go). User-authored skills are never touched.
  - Refreshes a stale managed git-erg block in the root CLAUDE.md (one that
    claims "no CLI needed", points at tickets/tools/go/, or describes
    %erg v1), replacing only the block between the markers.
  - Sweeps the entire work tree -- gitignored files included, since
    hook-definition files like .claude/settings.local.json are routinely
    untracked -- plus the repository hooks directory, rewriting stale
    references (tickets/tools/go/erg -> tickets/erg, 'erg validate tickets/'
    -> 'erg check tickets/') wherever they appear: git hooks,
    .claude/settings.json, CI workflows, Makefiles, scripts. The ticket
    store, nested git repositories and submodules, binary files, and files
    over 2 MiB are exempt; files without a match are left byte-identical.
    One notice is printed per artifact pruned or file rewritten.

Does NOT commit. Exits 1 on archive/->closed/ filename collision (both directories are left untouched; the user must resolve manually). Exits 0 otherwise.
Review the diff with 'git diff tickets/' and commit manually.

## erg init [DIR] [-n|--dry-run] [--force] [--show NAME]

Unpack embedded bootstrap assets into the project.

Writes two files relative to DIR (default: current directory):

  - tickets/.ergrc -- project configuration (label vocabulary, update remote).
  - tickets/AGENTS.md -- agent operating instructions for the ticket workflow.

It also writes tickets/.erg-assets, a provenance manifest recording this
binary's rev/date and the SHA-256 of each embedded asset. The manifest is
committable durable state (not gitignored) and is invisible to erg check, so
it never trips the pre-commit hook. It is deterministic: the same binary and
assets always produce byte-identical content.

The format specification and setup guide are available on demand via
erg spec and erg integration respectively.

Requires tickets/erg (the binary) to already exist in the project; the command
refuses if it is absent. This requirement ensures that agents do not accidentally
initialize an empty directory that was never meant to be a ticket store.

Each asset is compared against the embedded version with a dpkg-style 3-state
rule. Byte-identical files are left unchanged. A differing file that still
matches the .erg-assets stamp (or, with no stamp, a known shipped hash) is a
clean upgrade -- erg never touched it, so it is overwritten and a
"git restore -- <path>" hint is printed. A differing file that matches neither
is a local edit: it is preserved and the command exits 2 (local edits are never
overwritten without --force).

A preserved file is not stamped. The manifest records what init INSTALLED, so
an asset init declined to touch keeps whatever the previous manifest said about
it and gains no new entry -- stamping it with the embedded hash would certify a
customised file as identical to the shipped default and silence every later
report about it. An asset with no entry is compared against the embedded copy
directly, and erg check says so without claiming a direction.

When no stamp attests the difference, init says exactly that rather than
"local edits": with no recorded provenance, nothing distinguishes your own edit
from an upgrade the store never stamped, and --show is how you find out.

The stamp also records which binary wrote it, and init compares that date with
its own. If this binary is the OLDER one -- an erg from before the last init --
then refreshing would revert the deployed assets, not upgrade them. Such a file
is preserved too, and init says so and points at 'erg sync' rather than
claiming a local edit. A stamp with no date (written by an erg predating the
field) carries no direction and is treated exactly as before. This is what makes
sync followed by init from the corresponding updated binary an order the code
enforces and not merely a convention.

Flags:

  -n, --dry-run   Preview what init would create, refresh, skip, or leave
                  unchanged without writing or removing any file.
  --force         Overwrite files that differ from the embedded version
                  instead of skipping them. Use with care: local edits are
                  replaced. On a rollback (the .erg-assets stamp is newer than
                  this binary) a forced overwrite of a file still matching that
                  stamp is reported as "downgraded", not "refreshed": nothing
                  there was locally edited, the file is being reverted to an
                  older release. Run 'erg sync' first if that is not what you
                  want.
  --show NAME     Print this binary's embedded copy of NAME on stdout and exit,
                  writing nothing. NAME is .ergrc, AGENTS.md or erg-github
                  (with or without the "tickets/" prefix). The output is byte-
                  identical to what the asset compare uses, so it pipes into
                  diff or sha256sum and settles what an asset report cannot:
                  whether a copy differs because it was edited locally or
                  because the binary moved on.

                    erg init --show .ergrc | diff - tickets/.ergrc

                  Needs no project and no tickets/ directory.

If tickets/spec-erg-v1.md or tickets/integration.md exist from a previous init
and match the current embedded content, they are removed as orphaned assets.
Files that have been edited locally are preserved.

After a successful run (not in dry-run), init chains a read-only corpus check
and prints any warnings, but its exit code reflects the init outcome only --
the chained warnings never change it.

Canonical keep-current sequence: run 'erg sync', then run init from the newly
synchronized tickets/erg on Linux x86-64, or from a native system erg rebuilt from
the same reviewed revision on other platforms. Sync replaces the traveling binary;
init delivers embedded-asset changes and refreshes the default label vocabulary.
The default vocabulary is frozen-by-copy into .ergrc at init time -- a
new default added later to the binary is shadowed by the existing file and never takes
effect until erg init overwrites the file (clean upgrade) or the user opts in with
--force (local edit). erg sync alone cannot un-shadow a frozen vocabulary.

Exit codes: 0 success; 1 a hard error (bad flag, missing binary, write
failure); 2 a file was preserved and skipped -- either it has local edits, or
it is newer than this binary (run with --force to overwrite). See "Exit codes"
in erg --help --all.

## erg install [DIR] [--hooks] [--push-hook] [--inject-agents] [--create-agents-md]

Wire up integration hooks and agent instructions for a project that already
has a ticket store (created by erg init).

By default -- with no flags -- install does nothing outside tickets/. Each
piece of wiring requires an explicit opt-in flag:

  --hooks              Install (or upgrade) the pre-commit hook (erg validate
                       + erg check on every commit, rejects tickets/erg on
                       non-default branches) AND the pre-push hook (warns
                       about closed-but-unarchived tickets, never blocks).
                       Both are delimited by sentinel markers and inserted
                       right after the shebang so they run before any
                       third-party hook content. Existing content outside
                       the markers is preserved.

  --push-hook          Install (or upgrade) the pre-push hook alone, without
                       the pre-commit hook. The hook WARNS about tickets that
                       are closed but not yet archived, printing the exact
                       archive+commit+push recipe. It mutates nothing and
                       never blocks the push.

  --inject-agents      Add a one-line pointer to tickets/AGENTS.md inside a
                       sentinel-marked block in the project-root AGENTS.md.
                       If the root AGENTS.md does not exist, the flag is
                       refused unless --create-agents-md is also given.

  --create-agents-md   Permit --inject-agents to create a root AGENTS.md when
                       none exists. On its own it does nothing.

All wiring flags default to off. install never overwrites content outside its
managed block; on rerun or upgrade it replaces only the region between the
markers. All preconditions are checked before any file is written, so a refused
run changes nothing on disk.

Requires tickets/erg (the binary) to already exist in the project, same as
erg init.

Exit codes: 0 success; 1 a hard error (bad flag, missing binary, not a git
repository, unbalanced markers, refused AGENTS.md creation, or a write
failure). See "Exit codes" in erg --help --all.

## erg spec

Print the embedded %erg 0.1 format specification to stdout.

This is the same content that older versions of erg deposited as
tickets/spec-erg-v1.md during init. It is now served on demand to keep
the tickets/ directory uncluttered.

## erg integration

Print the embedded long-form guide to stdout: the pre-commit hook and CI
integration, then the working conventions that are too long to keep in
tickets/AGENTS.md -- optimistic ID allocation and collision recovery, scanning
open PRs for a colliding ID, checking the merged default branch after a ticket
PR lands, decision records versus artifacts, the handoff-document section
template, and where project-specific ticket lore belongs.

tickets/AGENTS.md is resident context, re-read at the start of every agent
session, so it stays short and points here. This is the same content that older
versions of erg deposited as tickets/integration.md during init; it is now
served on demand to keep the tickets/ directory uncluttered.

## erg version

Print self-diagnostic info and discover other erg binaries.

Prints the following fields for the running binary:

  - path:     resolved absolute path (symlinks followed).
  - sha256:   full 64-char hex SHA-256 of the binary file; recompute and verify
              with stock tools by hashing the resolved 'path:' printed above,
              e.g. `sha256sum <path>` (or `shasum -a 256`,
              `openssl dgst -sha256`).
  - built:    build date injected at compile time via -ldflags (or "[unknown]").
  - revision: VCS commit hash injected at compile time via -ldflags (if present).
  - arch:     GOOS/GOARCH of the running binary.
  - role:     "traveling" for the committed tickets/erg (a path ending in
              /tickets/erg), "system" for a copy on your PATH. See the README
              "Binary policy" section for what each role is for.
  - verify:   a ready-to-paste `sha256sum` command for the binary's resolved
              path. Shown only for the traveling copy (a path ending in
              /tickets/erg), where verifying the committed binary matters most.

After printing the running binary info, `erg version` discovers other erg binaries
in well-known locations (./build/erg, ./tickets/erg, ~/.local/bin/erg, and PATH
entries), compares VCS revisions and build dates against each discovered copy, and
prints the sync command for any outdated copy it finds.

Set ERG_VERSION_NO_DISCOVER=1 to suppress discovery (used internally by version
comparison to avoid recursion).

## erg sync [--upstream]

Synchronize this project's vendored binary with a committed binary fetched through git.

By default, sync reads tickets/erg from the current PROJECT'S origin. It aligns clones
with the version vendored and reviewed by that project. It does NOT check whether the
git-erg project has published a newer binary.

Use --upstream to import tickets/erg explicitly from the git-erg project. Review and
commit the resulting binary in the adopter project so its other clones can use the
default project-origin mode. ERG_UPDATE_URL and the .ergrc [update] url key remain
custom-source overrides when --upstream is absent; the environment wins over config.
The explicit --upstream flag wins over both overrides.

Project-origin mode reads the binary at the adopter's local store-relative path.
Upstream and configured sources instead read canonical tickets/erg, because their
repository layout is independent of the adopter's local store name. Upstream fetches
only the source tip needed for that blob rather than importing git-erg's full history.

Sync uses git (already a dependency of git-erg) -- never an embedded network client --
so the binary carries no network code. Project-origin and configured-source fetches run
in the ticket store's repository. The shallow upstream fetch runs in a private throwaway
bare repository under the ticket store, so all writes stay confined there while it
neither imports git-erg objects nor marks the adopter repository as shallow. Sync
extracts the committed binary at the source's default branch and compares its hash to
the vendored binary at <ticket store>/erg. The executable used to invoke sync is never
replaced, so a system-native erg remains intact.

Messages name the resolved source, sanitizing configured URLs and suppressing git's
raw diagnostics so credentials cannot be echoed. If the hash differs, sync replaces
the vendored binary atomically (exclusive temp file, fsync, then rename).

Transport errors exit 0 so that 'erg sync && erg validate' chains do not fail in
offline or isolated environments (no remote configured, no network, not a git repo).
If the trusted project origin lacks its store-relative binary, sync warns and also exits
0: the file may simply be gitignored or not committed yet. A configured source or the
explicit upstream missing canonical tickets/erg is a hard error instead. If no ticket
store is found, sync does nothing and exits 0 -- it never pulls the binary from an
unrelated repository you happen to be standing in.

After a successful project-origin sync, checks whether any .erg files in the ticket store
still carry legacy Status: headers. If found, prints explicit migration guidance:
'erg migrate DIR', 'git diff tickets/', 'git commit'. The sync command never mutates
ticket files itself -- migration is a separate, reviewable step.

erg sync replaces the binary only -- it never writes or modifies any managed store file
(.ergrc, AGENTS.md, or tickets). Its private temporary git directory is removed after
the upstream fetch. Embedded-asset changes and new default label vocabulary are
delivered by a follow-up 'erg init'. On Linux x86-64, invoke the vendored binary for
that follow-up so init uses the bytes just synchronized:

  erg sync
  tickets/erg init

The explicit import sequence keeps review ahead of first execution:

  erg sync --upstream
  git diff -- tickets/erg
  # review or verify the imported binary here
  tickets/erg init
  git diff -- tickets/
  git commit

An upstream or configured-source import is not executed automatically. Review it first,
then run '<ticket store>/erg check' before init. Project-origin sync may run that check
automatically because those bytes are the project's already-reviewed vendored version.

The vendored binary is always Linux x86-64. On macOS, Windows, or another architecture,
'erg sync' still updates that project/CI artifact but you must update or rebuild your
native system erg from the same reviewed git-erg revision before running 'erg init'.

erg init applies the dpkg-style 3-state rule: byte-identical files are left untouched;
a file that matches the previously recorded stock hash is a clean upgrade and is
overwritten; a locally-edited file is preserved (exit 2). A file the stamp says a
NEWER erg wrote is preserved too, so an init run from a stale binary reports the
situation instead of reverting the store. Running erg sync alone is never
sufficient to absorb new defaults.

The old command name 'erg update' is a compatibility alias for 'erg sync'.
