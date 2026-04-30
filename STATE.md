# State — git-erg

_Last updated: 2026-04-30 — housekeeping run_

## Stats

- Tickets: 11 total — 5 closed, 6 open (4 ready, 2 blocked)
- Tests: 31 passing (validate 18, ready 9, archive 4)
- Open PRs: 1 (#4 ci/0009-add-ci)

## Ready to work

| # | Title |
|---|-------|
| 0006 | Add erg claim/release/close commands, modularize Go source, add godoc |
| 0009 | Add CI — validator self-test on PR/push |
| 0011 | erg self-update from origin when online |

0002 is a sample blocked ticket used for validation testing — leave open.

## Blocked

| # | Title | Blocked by |
|---|-------|------------|
| 0007 | Install erg binary on PATH | 0006 |
| 0008 | Add erg pick command | 0006 |

## Next actions

1. Implement **0006** (claim/release/close + modularize) — unblocks 0007 and 0008
2. Add **CI** (0009) — straightforward, no blockers
3. **erg self-update** (0011) — decide distribution mechanism (raw GitHub vs Releases) before starting

## Blockers / notes

- 0011: open question on distribution mechanism (raw GitHub download vs GitHub Releases asset)
- No Go toolchain required for users — binary is committed to repo; self-update preserves this
