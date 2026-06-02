# PEP: %erg v1 — Agent-native local ticket system

**Status:** Draft
**Created:** 2026-03-27
**Modified:** 2026-05-29
**Author:** Minmh Ha-Duong <minh.ha-duong@cnrs.fr>

## Abstract

An agent-friendly file-based ticket system designed for development in disconnected environments. Tickets are plain-text files with a
versioned format (`%erg 0.1`), committed to git, and validated by a
pre-commit hook. The system complements (not replaces) GitHub Issues.

The source of truth is the specification (run `erg spec` to read it).
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

### 1. Magic first line: `%erg 0.1`

**Choice:** Every ticket file starts with `%erg 0.1`.

**Rationale:** Enables file-type detection without relying on the `.erg`
extension. The version follows a MAJOR.MINOR scheme (no `v` prefix),
matching conventions used by PDF (`%PDF-1.7`) and YAML (`%YAML 1.2`).
Pre-1.0 signals that the format is still evolving; minor bumps may still
introduce breaking changes. Post-1.0, minor bumps add features
backward-compatibly and major bumps signal breaking changes.

Provides a schema version for forward compatibility — a future `%erg 1.0`
that stabilizes the header set won't break 0.1 validators (they reject
unknown versions rather than silently misparsing).

**Alternatives considered:**
- YAML front matter (`---`/`---`): ambiguous, could be confused with log
  separators. YAML parsing is heavy for agents.
- JSON: not human-readable, poor diffability in git.
- No version marker: makes format evolution impossible without breaking
  existing files.

### 2. Closed header set (no X- extensions)

**Choice:** 0.1 defines exactly 6 headers: Title, Closed, Created, Author,
Blocked-by, Label. No `X-` extensions are allowed.

**Rationale:** Agents work best with rigid schemas where there's exactly
one right way to write a file. Open extension headers invite creative
variations that break tooling. If a future version needs new headers (Priority, Labels,
Assignee), it bumps the version and extends the closed set.

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

Two mechanisms exist because they serve different purposes. The `Closed:` header is the explicit author-intent signal: the agent or human who closes a ticket writes the reason directly into the file content. The path-based test is the structural/filesystem signal: it lets tooling move tickets into a `closed/` directory without editing file content at all. These are complementary — a ticket moved by `erg archive` gains the path signal; a ticket closed in-place by `erg close` gains the header signal. Either alone is sufficient to mark a ticket done.

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
   agents/humans, not properties of the `%erg 0.1` format.

%erg 0.1 describes what a ticket is, not how concurrent agents or worktrees
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

**Choice:** The primary interface is POSIX — `.erg` files are plain text, and any stock Unix tool (`cat`, `grep`, `find`, `vim`, an agent's `Read`/`Edit`) reads and writes them. The `erg` binary is a token-saving and validating helper on top of the same files, also installed as a validator in a pre-commit hook. POSIX is transport #1; the CLI is transport #2.

**Rationale:** Agents are good at running CLIs and at parsing structured text, but the former is cheaper and faster.

**Alternative considered (no second transport):** Use only POSIX — the agent reads `.erg` files directly using `Read`/`Edit` and the binary is only a validator in the pre-commit hook, not exposed as a query/mutation surface. Problem: asking an LLM to burn tokens for something a local binary can do faster and deterministically is irresponsible. The CLI exists as a token-saving optimization, not as a contract replacement.

**Why a CLI as transport #2, and not an MCP server.** Once transport #1 is fixed
(POSIX, no choice in the matter — every system has it), the question is what
the second transport should look like. A bash CLI wins because it serves the
same four users that POSIX already serves, with no per-consumer wiring:

1. **Hooks.** `pre-commit` runs `erg validate` non-interactively and reads the
   exit code. Hooks live in a shell; they cannot reasonably hand off to a
   long-running server that may or may not be running, owned by some other
   process, or unreachable from a detached worktree. Before the CLI existed,
   a `grep`-based check on transport #1 would have worked too.
2. **CI.** GitHub Actions, GitLab runners, and local `make` targets all shell
   out the same way; `--json` feeds downstream steps. No client SDK, no
   per-runner configuration.
3. **Agents.** Every coding agent — Claude Code, Cursor, Aider, Codex, Cline,
   Continue, plain-Python scripts driving the API — can run a subprocess, and
   every one of them can `Read`/`Edit` files directly. No agent needs to be
   MCP-aware to participate. MCP support is uneven, demands per-client
   `settings.json` wiring in every consuming repo, and locks the integration
   to whichever clients implement it today.
4. **Humans.** `cat tickets/0042-*.erg | less` works on transport #1 without
   `erg`; `erg --help` is self-documenting on transport #2 and output pipes to
   `grep`/`jq`/`less`. An MCP server is opaque without a client.

An MCP server would be a *third* transport, not a replacement for either of
the first two. It would sit on top of the same files (POSIX) or shell out to
the same handlers (CLI), and would have to justify the installation,
`settings.json` wiring, and client-version skew that POSIX and the CLI both
avoid. The headline MCP benefit — structured I/O — is already covered by
`--json` on `list` and `ready`, and corpus sizes do not justify a long-running
process with a lifecycle of its own. The CLI further keeps the spec as the
contract: an agent without `erg` still participates via POSIX. An MCP server
would tend to become the *de facto* interface, weakening the
file-as-source-of-truth invariant the rest of the design defends.

**Why not even an opt-in `erg mcp` subcommand (third-transport experiment).**
Honest answer: we have not evaluated the need. No agent integrator has asked
for it, no workflow has hit a shell-escaping or token-cost wall that `--json`
did not solve, and the maintainer cost of a third transport (schema drift
between CLI flags and MCP tool definitions, version skew across clients, a new
surface for the validator to keep in sync) is non-trivial. The door is not
closed: a stdio JSON-RPC wrapper over the existing handlers is a small change
that would preserve §8 while letting MCP-native clients skip the shell. We
are waiting for a concrete
use case before paying that cost.

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

`closed/` is now the canonical subdirectory for done tickets. `archive/` is deprecated but still accepted on read for backward compatibility. `erg migrate` promotes `archive/` → `closed/` automatically when both directories exist. The decision to canonicalise `closed/` was made after early deployments created both `archive/` and `closed/` in different repos, splitting queries: tooling that searched `tickets/closed/` missed tickets in `tickets/archive/` and vice versa. A single canonical name eliminates that split-brain without breaking existing repos — migration is automatic.

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
`tickets/AGENTS.md` for the spec pointer.


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

**Other architectures: deferred, demand-gated.**

<!-- GENERATED by Claude — human review needed -->

The committed `tickets/erg` is unconditionally Linux x86-64: the *traveling* artifact for the boxes CI/CD and agents actually run on. Supporting other architectures stays a v1 design goal, but shipping *prebuilt* multiarch binaries is deliberately deferred until a concrete adopter needs it, rather than offered on spec. The must-have audience — agents and CI — is already on Linux x86-64; every other platform has the zero-install POSIX path; and a non-Linux developer who wants the CLI locally can cross-build in seconds (`cd src/go && GOOS=... GOARCH=... go build`; Go's cross-compilation is trivial and CGO-free) or delegate it to a coding agent. Standing up a build-sign-reproduce pipeline for N targets is recurring solo-maintainer cost against speculative demand — the same evidence-gating this section applies to a Rust rewrite.

When a real non-Linux adopter makes the friction concrete, the chosen mechanism is *courtesy builds*: cross-compile that target, publish it as a signed release asset — not committed into the repo, since N committed binaries would bloat every clone — and grow the matrix by demand. The committed Linux binary remains the traveling/CI artifact; a developer's native build coexists with it on the PATH and must never overwrite `tickets/erg`. Bundling erg's own source into adopter repos was considered and rejected: erg has zero third-party dependencies (nothing to vendor in the Go sense), the source already travels in git-erg, and copying it into every adopter repo would add clutter and a three-way sync among source, committed binary, and vendored copy without even removing the local build step.

**Why Go over Rust/C, revisited against the six traits (2026-05-29):**
The contract is *agnostic, offline, standalone, fast, small, stateless*.
Agnostic/offline/stateless are architecture, language-neutral; the language
only decides small, fast, standalone, and development pragmatics.

- **C** wins size (KB-class, like `awk` at 167 KB) and startup, but is
  effectively disqualified for a *data-custody* tool maintained by a small
  team: manual memory management is a standing data-loss/CVE risk that fights
  the "never lose data" goal directly, for a few MB of savings.
- **Rust** is the only credible challenger — it wins small + fast and keeps
  memory safety, with a clean static `musl` target. Its costs are a full
  rewrite, slower edit/compile cycles, and friction with the zero-dependency
  trait (idiomatic Rust reaches for crates like serde/clap; avoiding them
  means hand-rolling). A rewrite is the ultimate push-from-wishlist and has
  no evidence behind it. Rust is the **documented fallback** if a hard
  size/startup requirement ever lands with measurement.
- **Go** (chosen) is weakest only on binary size — and that size *is the flip
  side of the trait we rely on most*: the fat stdlib that lets us hold zero
  third-party dependencies is the same ~6 MB runtime. Measured: a
  `CGO_ENABLED=0 -trimpath -ldflags "-s -w"` build is **6.2 MB and fully
  static** — smaller than the prior dynamic build, mid-pack against its real
  peer (`git` main 3.9 MB, `git-core` 11 MB), and the floor is the language,
  not bloat. Go uniquely nails trivial cross-compilation (`GOOS`/`GOARCH`),
  zero-dep via stdlib, memory safety, and solo/agent development velocity —
  the axes that actually serve agnostic / travels-everywhere / maintainable.

Conclusion: stay on Go; bank the static + strip wins. "Small" therefore means
*smallest sensible Go binary*, not parity with C — KB-class would be a
re-platform decision, evidence-gated, not a tuning task. The design-contract
guardrails (ticket 0146) lock these properties in as tests.

**Distribution stance: source is primary, the binary is an optional cache.**
The single binary is the *implementation*, but it is not the primary
*distribution* artifact — `src/go/` is. The committed `tickets/erg` is a
convenience cache for boxes where Go is unavailable and the POSIX path is not
enough; it is reproducibly buildable from the vendored source and adds no trust
the source does not already carry. This follows directly from the §8 POSIX-first
invariant (the binary is optional) and from the adoption-first decision that
`erg` must be useful without a Go toolchain: keeping the committed binary is a
deliberate choice that holds git-erg in the *infrastructure class* (a blob in
many repos, run in hooks/CI), which is why the reproducible-build + signed-tag
controls (ticket 0151) are obligatory rather than precautionary. The full
trade-off analysis — including the vendor-and-rebuild alternative that would
shed the class at the cost of requiring Go — is in
`docs/audit-infrastructure-class.md`.

**Single-threaded by design.** Corpus scans (`check`, `list`, `ready`) run
sequentially, not across goroutines. The work is I/O-bound, not CPU-bound, and
even at ~1000 tickets a full command is tens of milliseconds — there is no
latency worth recovering. Staying single-threaded keeps output ordering
deterministic (reproducible `list`/`check`, golden-file tests) and the resource
story trivial: no goroutines means no goroutine leaks and no fan-in error
handling to reason about. For the same reason `erg` never lowers its own
scheduling priority — that is the invoker's prerogative (`nice`/`ionice`/systemd
slice), and with nothing running concurrently there is nothing to throttle.
Should a future batch operation (e.g. a large `migrate`) ever justify
parallelism, it would be gated behind an explicit `--jobs` flag defaulting to 1,
so the default path stays deterministic.

### 11. Postel's Law: tolerant on read, strict on write

**Choice:** Nothing prevents an agent to parse a free-form text file as a ticket. The validator enforces `%erg 0.1` strictly on commit for `.erg` files.

**Rationale:** An agent may receive ticket-like information in any form:
raw `gh issue view --json` output, a sentence in conversation, a markdown
sketch, a paste from a PR comment. Requiring the agent to first convert
this into `%erg 0.1` before it can reason about it would be a barrier.

Instead: the agent reads whatever it finds, understands the intent, and
writes clean `%erg 0.1`. The pre-commit hook catches any formatting
mistakes.

This keeps the tooling simple (one format to parse, one format to
validate) while making the system maximally agent-friendly. The strict
format is a *write contract*, not a *read requirement*.

**Interior header blanks (ticket 0141).** The spec originally read "a blank
line ends the header block", and the parser implemented it literally: the
first blank line stopped header parsing and every line below it was silently
discarded. That is the *opposite* of read-tolerance. A human or LLM authoring
a ticket by hand naturally groups headers visually — a blank line before a
`Tag:`/`Blocked-by:` cluster — and the parser would drop those headers on the
floor. Worse, the drop was *silent and validation-masking*: a `Blocked-by:` or
`Closed:` placed after a blank vanished, so the unknown-ref rule never fired
and a stray `Closed:` would fail to close the ticket while still passing
`erg validate`. A blank after `Title:` instead made `Created:`/`Author:` look
*missing*, pointing the author at the wrong problem.

So an interior blank is now accepted on read rather than treated as the block
terminator (the block ends only at the first blank *not* followed by another
header line). The rule was relaxed deliberately, and is documented here so a
future reader does not "restore" the literal blank-ends-header behaviour and
reintroduce the silent data loss. Because accept-on-read alone would let
hand-authored files linger dirty, the same normalisation is applied on write
(`close`/`label`/`unlabel`), swept corpus-wide by `erg migrate`, and surfaced as a
non-fatal warning by `erg validate` and `erg check` — the graded tiers in the
spec's *Postel's Law* section. The blank stays a hygiene issue, never a
commit-blocking format error, consistent with how BOM/CRLF are handled.

### 12. Label header: a closed enum for workflow labels

<!-- GENERATED by Claude — human review needed -->

A `Label:` header (originally named `Tag:`) was added in v1 after initial deployment. Unlike a free-form label, it uses a closed enum for the same reasons as the header set itself: open labels proliferate without a schema. It was renamed from `Tag:` to `Label:` in 0175 to follow the dominant forge convention (GitHub/GitLab) and because the controlled-vocabulary model is semantically closer to "labels" than to free-form "tags"; `erg migrate` rewrites legacy `Tag:` (and pre-0106 `Tags:`) headers to `Label:`.

`Label` is repeatable because the allowed values are orthogonal flags — a ticket can legitimately carry more than one label simultaneously. The default vocabulary is `needs-human` and `deferred`. Extended values can be configured via `tickets/.ergrc [labels]`.

The addition is backward-compatible: tickets without `Label:` remain valid. No version bump was needed.

### 13. Artifacts live in the tree, referenced by path

<!-- GENERATED by Claude — human review needed -->

A ticket's body is freeform markdown for the ticket's own content; artifacts the project merely consumes or produces — generated reports, data files, diagrams, prototype scripts — are not part of the ticket and do not live inside it. Following the same out-of-band discipline as coordination (§6), they are stored in their natural location in the project tree and referenced from the body by path. This is not a novel position — issue trackers, build systems, and documentation toolchains have long kept large or binary artifacts beside the record that points at them rather than inside it — and %erg simply declines to reinvent an in-band attachment store. The filename-twinned sidecar (`0002-slug.md` beside `0002-slug.erg`) is specifically discouraged: erg validates and relocates only the `.erg` file, so a name-coupled companion is invisible to `check`, is orphaned when the ticket moves to `closed/`, and breaks silently on rename — the same implicit-coupling failure the format avoids elsewhere. Whether to add a first-class, validated `Attachment:` header (existence-checked, lifecycle-aware) is deferred pending more observational data on how often genuine attachments arise; until then, reference by path.

### 14. The store as the repo's durable management log

<!-- GENERATED by Claude — human review needed -->

The tickets store is more than a backlog; it is the repo's durable record of how the project was steered over time — closer to a ship's `journal de bord` than to a private diary. Because the `--- log ---` entries are append-only and the store lives in the tree, that steering history is versioned, diffable, and `git blame`-able alongside the code it governs. Unlike forge Issues (§7), which live in the forge's server-side database, the record survives forge migration, policy change, and ownership change: the management layer is owned, not rented. For local-first and sovereignty-conscious users that ownership is the point; for research reproducibility and for auditing in regulated settings, the same property means the decision process travels with the artifact and can be reconstructed after the fact.

That value comes with an honest limit. %erg 0.1 makes the log append-only by *convention* — `erg validate` enforces its shape, but git history is rewritable (`git push --force`, `filter-repo`), so the store is a strong versioned record, not a tamper-proof one; the logbook metaphor is borrowed for its shape (chronological, owned, accountable), not for the statutory immutability a real ship's log carries. Closing that gap decomposes into two layers at different altitudes, and the split is deliberate. *Prevention* — forbidding force-push and rewrites — is necessarily a forge or server concern (branch protection, protected refs, pre-receive hooks); leaning on it would reintroduce the forge coupling §7 keeps at arm's length, so it is a non-goal of the format. *Evidence* — making a rewrite detectable rather than impossible — can live in the files themselves and is the sovereignty-consistent answer: a hash chain over log entries (each digesting its predecessor, checkable by `erg check`), optionally pinned by a periodically signed anchor over a known-good head, reusing the existing release-signing cadence; signed commits with an author-to-key binding would further tie each log line to a verified signer. None of these are commitments of 0.1 — they are the design space a future minor or major version would draw from. The format ships the durable record now and leaves cryptographic tamper-evidence as a separable, opt-in extension.
