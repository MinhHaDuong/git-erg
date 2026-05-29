# State — git-erg

_Last updated: 2026-05-29T19:20Z — Raid 151 (security audit): split the needs-human 0151 into four automatable children and merged all four. **0155** (#170): `erg version` prints the full SHA-256, names the algorithm, and emits a copy-paste `verify:` hint (shell-safe, gated to the bootstrap path). **0156** (#171): `make verify` rebuilds the committed `tickets/erg` byte-for-byte from its embedded revision (go1.21.13 pinned via go.mod toolchain; CI uses fetch-depth:0 so the ancestor revision is present) — the keystone supply-chain control, now live in `make test`/CI. **0157** (#172): `test_security.sh` — 26 falsifiable checks (path-traversal, symlink escape, input-DoS, ID-injection) each with a negative control — plus a real confinement fix the audit surfaced: `erg rm` FILE-form now passes through `withinStore` (the explicit-`new` DIR was deliberately documented as an escape hatch). **0158** (#182): `docs/threat-model.md`, `docs/red-team-checklist.md` (run once — zero high-severity), and a README "Verifying the binary" opsec section. 0151 stays **open (needs-human)** for its one remaining item: signed release tags (`git tag -s` — needs a maintainer GPG key). Every PR cleared CI + Copilot; #170/#172 also cleared ultra review (only nits). Process lesson saved: never `Blocked-by:` a parent tracker from a child — it breaks erg-pr-merge's per-file pre-commit validation. Earlier: 0149 (data-safety atomic write path), 0146 (contract guards), 0147/0148 (static build + git-fetch update) all landed. never-lose-data is the foremost invariant. 160 closed, 5 open._

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

- Tickets: 160 closed, 5 open — ready: 0163, 0164, 0166; needs-human: 0151, 0152
- Tests: green — `make test`, `make verify` (reproducible build), `erg check` (164) all pass on main
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
<!-- generated 2026-05-29T13:32Z (housekeeping sweep) -->

**Recent commits:**
  0ed6ffd chore(0149): close + archive — delivered in #162
  fde94a0 chore: rebuild bootstrap binary [skip ci]
  be36e4d fix(0149): respect read-only target files in the atomic write path
  94f1038 fix(0149): address Copilot review remarks
  b458f90 fix(0149): address review-panel findings on the data-safety write path
