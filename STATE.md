# State — git-erg

_Last updated: 2026-05-04 — closed 0026/0027/0028/0029/0033/0035 (merged PRs); filed 0036/0037; all open tickets are ready._

## Stats

- Tickets: 31 total — 26 closed, 5 open (3 ready + fixtures 0001/0002 in archive/)
- Tests: green
- Open PRs: none

## Ready to work

| #    | Title                                                              | Notes |
|------|--------------------------------------------------------------------|-------|
| 0008 | Add erg pick command: mechanical shortlist for the IDH pick-ticket skill | Body references deprecated Status: workflow — refresh before executing |
| 0036 | Add standing regression check for shell test binary defaults       | Follow-up from 0035 |
| 0037 | Make `erg validate` non-recursive by default                       | Spec already states non-recursive; implementation gap |

## Blocked

None.

## Sequencing

1. **0037** (`erg validate` non-recursive) — self-contained, low risk.
2. **0036** (regression test for binary defaults) — self-contained.
3. **0008** (`erg pick` command) — depends on ticket body refresh first; larger scope.

## Notes

- **`Status:` is no longer part of the format.** Closure is derived from the path component test or a `Closed:` preamble header. `erg validate` rejects any `Status:` line; `erg migrate` is the only command that tolerates it (to convert it).
- **Closed-in-place tickets**: 0026/0027/0028/0029/0033/0035 live in `tickets/` with `Closed:` headers — not yet moved to `tickets/closed/`. Safe to archive when convenient.
- **Fixture tickets 0001/0002** live in `tickets/archive/` without `Closed:` headers — `erg ready` surfaces them as open because `archive/` is not a closed path. Leave as-is; they are not actionable work.

Autonomous-run policy is maintained in `AGENTS.md`.
