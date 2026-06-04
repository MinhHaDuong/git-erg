# AGENTS — git-erg policy

This file defines stable operating policy for autonomous agent runs.
Operational snapshot (ready/blocked/sequencing) lives in `STATE.md`.
Policy changes belong here; `STATE.md` should remain status-only.

## Earning a feature

A feature is **pulled by verified empirical need, not pushed from a
wishlist.** The default answer to "should we build this?" is *no*. The
burden of proof is on adding, never on deferring. An idea that has sat on
the `STATE.md` dreamlist is there on merit until evidence moves it — being
written down is not a need.

What counts as evidence: a measured slowness with numbers, a real failure
or near-miss, a user (human or agent) who actually hit the friction, an
audit that surfaced the pain. What does not count: "would be nice", "for
completeness", "while we're here", symmetry, or an abstraction that merely
feels tidier. If you cannot name who felt the lack and how, it is not ready.

Respect the core invariants. They are non-negotiable, and a proposal that
fights one starts deep in the negative:

Seven invariants, one above the rest — **never lose data** is the tool's
*job*; the other six guard its *form*. None is a lesser "nice to have":

- **Never lose data** (the one job) — the ticket files are sacred. Every
  mutation is atomic (write-temp-then-rename), validates before it replaces,
  never clobbers on ID collision, preserves the log/body losslessly, and
  confines its writes to the resolved store (a fail-safe against fat-fingers
  and overeager agents — removable, but it stops the common mistake); a
  killed `erg` leaves the old file or the new, never a truncated one. The
  blanket "never" can't be proven, so it is backed by a data-safety guard
  suite + standing audit (0149).
- **Agnostic** (POSIX-first) — a hand edit must always win; the plain `.erg`
  file is the contract and the binary is optional. Nothing may make the file
  the second-class source of truth (this is why a stale cache is a bug, not
  an optimisation).
- **Offline / disconnected** — no network calls, ever. Even `erg update` now
  shells out to `git fetch` (0148) rather than an embedded HTTP client, so the
  binary carries no network code at all — the invariant holds everywhere.
- **Standalone** — one *static* binary plus POSIX; zero third-party
  dependencies (the fat stdlib is what lets us hold that line).
- **Stateless** — the files are the only state; no external state encoded in
  tickets (no `pending`/`claimed`/`doing` — we *removed* that abstraction in
  0143/0144; do not smuggle it back under a new name).
- **Fast** — erg is *invoked* (pre-commit hook every commit, CI every push,
  agents in loops), never resident, so per-invocation cost is the product.
  Work stays linear in the corpus; no redundant passes.
- **Small** — the binary is committed and travels with every clone, so size
  is paid by everyone. Stay near the Go runtime floor (≤ ~5 MB — ratcheted
  down from 10 MB once 0147's static+stripped build and 0148's net/http
  removal cut the shipped binary to ~2.4 MB); guard against dependency bloat
  (0147 builds it static+stripped; 0146 guards it).

Scope confinement: `install` is the only verb that mutates outside `tickets/`
(`.git/hooks` and the root `AGENTS.md` pointer), and only behind explicit
opt-in flags (`--hooks`, `--inject-agents`) that default to off. Every other
verb stays confined to `tickets/` and offline. This is the implementation of
the Offline and Standalone invariants at the command boundary -- a contributor
adding a new verb must respect the same scope, or move the mutation behind an
equally explicit opt-in.

Audits are the evidence engine. Prefer an audit that discovers real need
over a feature built on a guess — the audit tells you whether the feature
is felt at all. Build the thing the audit justifies, not the thing the
wishlist remembered.

Posture properties — security and UX — cannot be reduced to a single guard or
closed by a one-shot audit; they drift as the tool changes. They need a
**standing, AI-assisted QA process** that re-runs per release: agents do the
legwork (red-team passes, persona dry-runs), a falsifiable subset runs in CI,
and humans gate the findings (0151 security, 0152 UX). The UX dry-run
procedure is in `UX-PROCESS.md`. The security red-team procedure is in
`SECURITY-PROCESS.md`. The foundations-coherence procedure is in
`FOUNDATIONS-PROCESS.md`. A binary embarked in thousands of repos is a
deliberate design-class choice, not measured adoption
(see `docs/audit-infrastructure-class.md`); under that class the security bar
is maximal — supply-chain and mass-RCE
class — and reproducible builds are the keystone control, **verifiable locally
and offline** (rebuild the in-repo `src/go/` with git+go; no phone-home). The
displayed hash uses a widely-available algorithm (SHA-256, recomputable with
stock tools), and we kindly teach two opsec tiers — basic (local vs
self-reported vs a maintainer-signed git release tag) and advanced (AI-review
the source, rebuild, hash that) — discoverably, so users actually verify.
Trust is published the way git itself does it — signed tags (`git tag -s`),
not a bespoke signing/revocation system (0153 closed YAGNI).

Simplicity is a feature, and declining is legitimate work. Closing a ticket
as YAGNI (0131), reducing a command to a thin alias (0144), or saying "no"
with a reason is a win, not a failure to ship. The smallest correct system
that meets a *demonstrated* need beats a larger one that anticipates an
imagined one.

## Autonomous Run Policy

Purpose: allow unattended sweeps without paralysis, risky edits, or
rubber-stamp merges.

### 1) Pick gate (what may be worked unattended)

An unattended run may pick only tickets that are:

- in `Ready`
- implementation- or test-focused (not design/umbrella/spec-policy)
- bounded (small/medium diff expected)
- local-repo only (no cross-repo coordination requirement)

Auto-skip and continue when ticket text includes high-risk or human-gate terms:
`design`, `umbrella`, `distribution model`, `security`, `auth`,
`credentials`, `policy`, `breaking change`, `migration strategy`.

### 2) Safety gate (hard no-go for unattended)

Never perform unattended:

- destructive git (`reset --hard`, history rewrite, forced checkout)
- secret handling or token setup
- infra/admin changes (branch protection, repo settings, permission model)
- unreviewed network-side mutations outside this repo

If a ticket requires any of the above: mark `not-picked` with reason and continue.

### 3) Change-scope gate (file allowlist)

Unattended edits are allowed only in these paths:

- `tickets/*.erg`
- `tickets/**/*.erg`
- `src/go/*.go`
- `tests/*.sh`
- `README.md`
- `Makefile`
- `AGENTS.md`

Any required edit outside this set: open PR as draft and stop merge for that ticket.

### 4) Verification gate (must pass before merge)

For each ticket PR, require:

- verification by a panel of external agents
- correction of all remarks surfaced by the verification panel, no matter how small worded
- focused tests for touched area (if present)
- `make test`
- `make validate`
- clean ticket DAG state (`make ready` still coherent; no unexpected blocker regressions)

On failure: do not merge; record failure; continue sweep with next ticket.

### 5) Merge gate (prevents shy-wont-merge and risky-merge)

Auto-merge only when all are true:

- ticket exit criteria satisfied
- verification gate fully green
- diff remains inside allowlist
- no unresolved review findings

Otherwise: leave PR open with explicit blocker note and continue next ticket.

### 6) Throughput rule (avoid full-run stop)

When one ticket is blocked/unclear/risky, skip it and proceed to the next ready ticket.
Unattended run should stop only when no safe ready ticket remains.

### 7) Shell test binary rule

Shell integration tests under `tests/*.sh` must default:

- `ERG="${ERG_BIN:-build/erg}"`

Never default to `tickets/erg` in test scripts. The committed
bootstrap binary can be stale when scripts are invoked directly.

`make test` must enforce this with a grep check and fail fast if any
script reintroduces the legacy default.
git-erg local tickets: see tickets/AGENTS.md
