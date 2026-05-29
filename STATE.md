# State — git-erg

_Last updated: 2026-05-29T13:25Z — Closed 0149 (data-safety guard suite): shipped `tests/test_datasafety.sh` + `src/go/atomicwrite_test.go` and a shared atomic ticket-write path (`writeTicketAtomic`/`atomicWriteFile`) — temp-then-rename, validate-before-replace, write-confinement, mode-preserving, read-only-respecting — that all mutating commands (close/log/tag/untag/migrate) now funnel through; wired in as the `datasafety` suite. Finding: there was no shared atomic path before (all mutations used truncating `os.WriteFile`); now consolidated, no follow-up. Earlier: Closed 0146 (design-contract guardrails): shipped `tests/test_contract.sh`, a falsifiable guard (each with a negative control) for the deterministic five invariants — agnostic, offline, standalone, stateless, small (5 MB ceiling) — wired into `make test` as the `contract` suite. The `fast` guards (parse-once + linear N-vs-2N op-count + wall-clock backstop) need loader instrumentation, so they split out to **0154** (READY) per 0146's own "land deterministic first; fast can lag". Earlier: 0147 (static+stripped build) and 0148 (self-update via git fetch) landed — the bootstrap binary is now static **and** stripped (~2.9 MB), no network code anywhere. Remaining work: 0150 (install/uninstall round-trip, READY), 0154 (fast guards, READY); needs-human: 0151 (security/red-team — standing QA process, max threat model: binary in thousands of repos; trust via git-native signed tags), 0152 (UX try-and-learn — standing AI-assisted QA process). Closed 0153 (signed manifest/revocation) as YAGNI. never-lose-data is the 7th, foremost invariant. 150 closed, 4 open, 2 ready._

## North star:

An agent-friendly local ticket system for development in disconnected environments.

## Milestones

- Feature freeze lifted — conference is past. This does **not** mean drain the
  dreamlist into `ready`. The bar is now "verified empirical need" (AGENTS.md);
  the queue stays empty until evidence pulls something into it.
- Next move, if any, is evidence-first: the `audit:` items (usage in idh,
  UX friendliness, data-safety) generate the need that would justify — or
  kill — the `feat:` items below. Audits before features.

## Stats

- Tickets: 150 closed, 4 open — ready: 0150, 0154; needs-human: 0151, 0152
- Tests: green — ok git-erg
- Open PRs: none

## Notes

- **erg validate vs erg check**: validate is per-file; check is corpus-level.
- **CI**: bootstrap binary rebuilt automatically on every push to main changing `src/go/`.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Test fixture IDs ≥9000 reserved for unclaimed-ticket tests.

## Deferred ideas, nice to have or never-ending

These are deferred *on purpose*, not just un-started. Each one is either
premature (no measured pain), in tension with a core invariant (offline,
zero-dep, no encoded state — note 0143/0144 removed the "claimed/pending"
abstraction), or waiting on an audit to prove it is felt. Do not promote one
to `ready` without the evidence the bar in AGENTS.md demands.

- feat: O(1) everything, tickets store cache.
- feat: Pre-create 00XX-is-next-ticket.erg.
- feat: erg list + filters. (Delivered in 0138: positive/negative tag filters,
  closed/open/blocked pseudo-tags.)
- feat: Flag blocking tags as a first-class concept (still open — currently
  the skip-tag set is configurable but the "blocking" framing is implicit).
- feat: Make erg ready an alias of erg list not blocking. (Delivered: 0143
  dropped the claimed-as-blocker divergence; 0144 reduced ready to a thin
  alias and replaced the "claimed" abstraction with literal git ref +
  worktree annotation per spec-erg-v1.md's matching rule.)
- feat: erg new with body on the line.
- audit: usage in idh.
- audit: UX friendliness for beginner, for expert trying out, for other agents, for developper
- audit security: never loose data, never leak keys
- feat: tests guaranteeing fundamental design criteria (disconnected, agnostic, discoverable)
- feat: AI script to realign docs and code
- audit: go coding practices, lint, smells

## Status
<!-- generated 2026-05-12T13:52Z -->

**Recent commits:**
  d1dcb0d docs: update erg-manual.md Generated-from header to current binary
  c1881d3 chore(0131): close — YAGNI, shell test suites already cover CLI surface
  759df32 chore: refresh STATE.md — 135 closed, 1 deferred, 0 open
  9d05413 chore: archive 0136 (closed via Closed: header in earlier commit)
  2377a8b docs(0129): document URL-shortcut Blocked-by ref grammar (spec-only) (#144)
