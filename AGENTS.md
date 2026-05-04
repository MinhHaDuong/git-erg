# AGENTS — git-erg policy

This file defines stable operating policy for autonomous agent runs.
Operational snapshot (ready/blocked/sequencing) lives in `STATE.md`.
Policy changes belong here; `STATE.md` should remain status-only.

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

- `tickets/**/*.erg`
- `tickets/tools/go/*.go`
- `tests/*.sh`
- `README.md`
- `Makefile`
- `AGENTS.md`

Any required edit outside this set: open PR as draft and stop merge for that ticket.

### 4) Verification gate (must pass before merge)

For each ticket PR, require:

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
