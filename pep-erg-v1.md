# PEP: %erg v1 — Agent-native local ticket system

**Status:** Draft
**Created:** 2026-03-27
**Modified:** 2026-04-04
**Author:** Minmh Ha-Duong <minh.ha-duong@cnrs.fr>

## Abstract

An agent-friendly file-based ticket system designed for development in disconnected environments. Tickets are plain-text files with a
versioned format (`%erg v1`), committed to git, and validated by a
pre-commit hook. The system complements (not replaces) GitHub Issues.

The source of truth is the specification in `tickets/spec-erg-v1.md`.
This PEP documents the rationale supporting the specification.

## Motivation

Using an online forge like GitHub or Gitlab means that listing open issues requires an  API call. In offline, rate-limited, or latency-sensitive contexts, this blocks the agent's ability to pick work. The erg ticket system uses local files for offline reads and a simple text format that works equally well with human and agent workflows.

### Name

The word `erg` is an old unit of work in Physics, so it stands right as an extension of a ticket file.

**Alternatives considered:**
- `.ticket` was too long and already encoded in path name.
- `.tix` sounded closer by was not as scientifically legitimate as `.erg`

### Text based files with structured body

Tickets have:
- The magic line
- A preamble of Key: Value headers
- Separator
- Timestamped log lines
- Separator
- Freeform body.

The format was inspired by Internet Message Format (email) RFC 5322.

## Design choices and rationale

### 1. Magic first line: `%erg v1`

**Choice:** Every ticket file starts with `%erg v1`.

**Rationale:** Enables file-type detection without relying on the `.erg`
extension. Provides a schema version for forward compatibility — a `v2`
that adds headers won't break v1 validators (they reject unknown versions
rather than silently misparsing).

**Alternatives considered:**
- YAML front matter (`---`/`---`): ambiguous, could be confused with log
  separators. YAML parsing is heavy for agents.
- JSON: not human-readable, poor diffability in git.
- No version marker: makes format evolution impossible without breaking
  existing files.

### 2. Closed header set (no X- extensions)

**Choice:** v1 defines exactly 5 headers: Title, Closed, Created, Author,
Blocked-by. No `X-` extensions are allowed.

**Rationale:** Agents work best with rigid schemas where there's exactly
one right way to write a file. Open extension headers invite creative
variations that break tooling. If v2 needs new headers (Priority, Labels,
Assignee), it declares `%ticket v2` and extends the closed set.

**Alternatives considered:**
- Open `X-` headers (as in PR #385): caused proliferation of
  `X-Phase`, `X-Discovered-from`, `X-Supersedes`, `X-Parent` — each
  requiring ad-hoc validation rules. The "closed set + version bump"
  approach is cleaner.
- RFC 822 with free extensions: too flexible for agent consumption.

### 3. Sequential numeric IDs (not mnemonics)

**Choice:** 4-digit zero-padded sequential IDs derived from filenames:
`0001-add-auth.erg`, `0002-fix-cache.erg`.

**Rationale:** Mechanical assignment — no creativity needed. The agent
runs `ls | sort | tail -1`, increments, and pads. Mnemonics (e.g., `afg`,
`ta`, `vt` from PR #385) require the agent to invent unique abbreviations,
which becomes fragile at scale.

**Collision handling:** Optimistic concurrency. Two worktrees may pick the
same number simultaneously. The pre-commit validator catches duplicates.
The agent that loses renames its ticket (increment again). This matches
git's own optimistic concurrency model.

**Alternatives considered:**
- Mnemonic acronym IDs: creative, readable, but collision-prone and
  not mechanically assignable.
- UUIDs: unique but unreadable, poor for human consumption.
- Hash-based IDs: explored early, abandoned because
  hashes are opaque and don't sort chronologically.

### 4. ID in filename, not header

**Choice:** The ticket ID is derived from the filename prefix, not from
an `Id:` header.

**Rationale:** Eliminates the consistency check between `Id:` header and
filename (a source of errors in PR #385). Single source of truth. The
filename is the canonical identifier.

**Alternatives considered:**
- `Id:` header (PR #385): required a validation rule to ensure
  header/filename consistency. Redundant information invites divergence.

### 5. Binary status values: open or closed

**Choice:** defaults to `open` (available), can be marked `closed` (done) in two ways: with a `Closed: [reason]` header or with the word `Closed` in the path name (directory or filename).

**Rationale:** Closing at the right time is hard enough. `pending` or `doing` adds ceremony without benefit if not executed perfectly, which is hard when development is distributed across machines and worktrees. Follow the `git` approach of optimistic concurrency. Synchronisation has a solution: use a forge not a local-first ticket system. The `erg` system is designed non-exclusive.

**Alternatives considered:**
- Three statuses (open/doing/closed): no way to express "waiting for review" without a separate mechanism.
- Four statuses (open/doing/pending/closed): consumes even more tokens to manage and still never right.
- Labels for sub-states: would require validating label values, adding complexity to the closed header set.

### 6. Coordination is out of scope

We explicitly declares coordination out of scope so future
readers do not reinvent a mechanism from silence. Coordination is where
previous attempts to track issues in git foundered and why code forges are used.

On top of doing/pending status, the original proposal included a `.git/ticket-wip/` claim protocol for cross-worktree coordination. This was removed for three reasons:

1. **Not a reliable lock.** Absence of a `.wip` file does not prove
   "not WIP" — agents may forget to claim, may crash mid-claim, or may
   work in a fresh clone where the file never existed.
2. **Observable out of band.** A git branch whose name contains the
   ticket ID is a sufficient deconfliction signal; no side file needed.
3. **Workflow, not spec.** Branch-naming conventions are choices between
   agents/humans, not properties of the `%erg v1` format.

%erg v1 describes what a ticket is, not how concurrent agents or worktrees
share access to one. There is no claim file, no lock, no doing-but-mine state.
If two agents need to avoid stepping on each other, they observe out-of-band
signals — typically a git branch whose name contains the ticket ID — and
coordinate there. Such conventions are workflow choices.


### 7. Forge Issues are a separate coordination layer

**Choice:** A ticket may reference a forge issue (`Blocked-by: github.com/MinhHaDuong/git-erg#435`) but never mirrors its state. The ticket system does NOT cache forge Issues locally.

**Rationale explored in conversation:**

*Pro cache:*
- Listing open work without network call.
- Local-first development workflow.
- Single query interface for both local and remote tickets.

*Con cache:*
- Sync protocol complexity (pull/push, conflict resolution, staleness).
- GitHub is source of truth — cache introduces eventual consistency bugs.
- Push/pull discipline overhead for every state change.
- Two representations of the same data invites divergence.

**Decision:** No cache. Local tickets are local-only; they never need to
exist on the forge. GitHub Issues remain the inter-agent coordination
layer. Forge references in `Blocked-by` use the form
`host/owner/repo#N`; they are never resolved by erg and are treated as
offline-unknown (blocking) until manually removed.

### 8. Agent-friendly with efficient binary helper

**Choice:** The primary interface is the `erg` binary. As fallback, agents can manipulate `.erg` files directly using `Read`/`Edit` tools. The binary runs as a validator in a pre-commit hook.

**Rationale:** Agents are good at running CLIs and at parsing structured text, but the former is cheaper and faster.

**Alternative considered:** The agent reads `.erg` files directly using `Read`/`Edit`
tools, the binary is only a validator in the pre-commit hook, not the
primary interface. Problem: asking an LLM to burn tokens for something that a local binary can do faster and deterministic is irresponsible.

### 9. Directory location: `tickets/` at repo root

**Choice:** Tickets live in `tickets/` at the repository root. Closed tickets may go to
in `tickets/closed/`. Agent-specific wiring (rules, skills) lives in
`tickets/integration`.

**Constraints**.
- Blockers should live in the same directory as blocked tickets.

**Rationale (interoperability, discoverability, ergonomics):**

The conceptual path for scaling a ticket store is:
1. All tickets at root.
2. Keep only active tickets in `tickets/` directory, move closed tickets to `tickets/closed/`.
3. Organization of `tickets/closed/` with subdirectories.
4. Active tickets in `tickets/open/`.
5. Organization of `tickets/open/` with subdirectories.

The current implementation is only tested with 1. and 2.
The other levels raise efficiency and file referencing issues -- the idea is to add an optional prefix in tickets ID with resolution along URL and filesystem standards.

- **Discoverability:** An agent dropped into a new repo runs `ls`. It sees
  `tickets/`. Done. The directory name is the documentation. Hidden
  directories (`.ergs/`, `.claude/tickets/`) require prior knowledge.
- **Interoperability:** Root-level project directories (`docs/`, `scripts/`,
  `tests/`, `hooks/`) are a universal convention. `tickets/` fits the
  pattern. Any agent framework, CI script, or human finds it the same way.
- **Ergonomics:** Tools co-located with data (`tickets/tools/` next to
  `tickets/*.erg`) means the validator doesn't need a config file to
  know where to look. Short paths tab-complete well.
- **Flexibility** Projects start with a simple `tickets/` directory. They can create `archive/` or `closed/` tickets.

**Alternatives considered:**
- `.ergs/` (hidden): invisible by default, violates "ls tells you
  what's here" principle. Agents must know to `ls -a`.
- `.claude/tickets/`: locks to Claude ecosystem, buried 2 levels deep,
  fights `.gitignore` rules (`.claude/*` is typically gitignored).
- Configurable location: adds a settings layer for zero benefit — one
  canonical location is simpler than a configurable one.

**Integration split:** Agent-specific wiring (`.claude/rules/`, `.claude/skills/`)
is separate from portable artifacts (`tickets/`). This mirrors how
`hooks/` (git infra) is separate from `.claude/rules/git.md` (agent
instructions about git). A non-Claude agent ignores `.claude/` and reads
`tickets/README.md` for the spec pointer.


### 10. Go binary as a single implementation

**Choice:** Single Go binary (`erg`) implements validate, ready, close,
and archive. No alternatives provided.

**Rationale:** The Go binary won because it runs with zero dependencies, fast, single file, cross-compilable.

**Alternatives explored:**
- Python: worked but required `PYTHONPATH` setup. Code readability does not matter anymore.
- Perl: 2.5x faster than Python, dropped as niche.
- Bash: Much slower, less portable.
- sh, awk: POSIX compliant, still slow.
- Rust: No real difference with Go. Lost on verbosity.
- Provide alternative reference implementation: more complexity, zero real benefit.

**Future plans:**
Supporting other architectures is a design goal for final v1. It involves
- Cross compilation (never tried ?)
- Distribution mechanism (`tickets/integration/[ARCH]/erg` ?)
- Design coexistence of arch-specific local and traveling version
- Installation documentation and automation


### 11. Postel's Law: tolerant on read, strict on write

**Choice:** Nothing prevents an agent to parse a free-form text file as a ticket. The validator enforces `%erg v1` strictly on commit for `.erg` files.

**Rationale:** An agent may receive ticket-like information in any form:
raw `gh issue view --json` output, a sentence in conversation, a markdown
sketch, a paste from a PR comment. Requiring the agent to first convert
this into `%erg v1` before it can reason about it would be a barrier.

Instead: the agent reads whatever it finds, understands the intent, and
writes clean `%erg v1`. The pre-commit hook catches any formatting
mistakes.

This keeps the tooling simple (one format to parse, one format to
validate) while making the system maximally agent-friendly. The strict
format is a *write contract*, not a *read requirement*.
