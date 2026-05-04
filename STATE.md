# State — git-erg

_Last updated: 2026-05-04 — archived closed tickets under `tickets/closed/`; corrected next-id to scan non-recursively (spec compliance); distribution design split into 0032/0033/0034._

## Stats

- Tickets: 34 total — 20 closed, 14 open (7 ready, 7 blocked)
- Tests: green
- Open PRs: none

## Ready to work

| #    | Title                                                              | Notes |
|------|--------------------------------------------------------------------|-------|
| 0001 | Sample open ticket — blocker fixture for 0002                      | Fixture; do not pick for feature work |
| 0015 | Add Tags header for revocable triage labels                        | Unblocks 0008 |
| 0024 | Integration tests for `erg next-id`                                | Unblocks 0026/0028 |
| 0030 | Forge-agnostic Blocked-by grammar                                  | Grammar simplification follow-up |
| 0031 | `erg close` removes `Blocked-by` refs from dependent tickets       | Independent behavior hardening |
| 0032 | Implement `erg init` / `erg uninstall` from embedded assets        | Distribution execution step 1 |
| 0034 | Make `erg migrate` explicit instead of update-time side effect     | Distribution execution step 2 |

0001 is the open half of the validator fixture pair (0001 + 0002) — leave open
indefinitely; it has no actionable work.

## Blocked

| #    | Title                                        | Blocked by       |
|------|----------------------------------------------|------------------|
| 0002 | Sample blocked                               | 0001 (fixture)   |
| 0008 | Add erg pick command                         | 0015             |
| 0026 | Go unit tests for parser and validator       | 0024             |
| 0027 | CI add `go test` and coverage                | 0026             |
| 0028 | Fill remaining branch coverage               | 0024             |
| 0029 | Init/uninstall round-trip test + harness hardening | 0032       |
| 0033 | Keep `erg update` binary-only                | 0023, 0032, 0034 |

## Sequencing

1. **0032** (`erg init`/`erg uninstall`) — distribution execution step 1.
2. **0034** (explicit migrate flow) — distribution execution step 2.
3. **0033** (`erg update` binary-only contract) — lands after 0032/0034.
4. **0024 → 0026 → 0027** — test and coverage chain.
5. **0015 → 0008** — tags then pick command.

## Notes

- **`Status:` is no longer part of the format.** Closure is now derived
  from the path component test or a `Closed: <reason>` preamble header.
  `erg validate` rejects any `Status:` line; `erg migrate` is the only
  command that tolerates it (in order to convert it).
- **`needs-human` status gap:** resolved by 0015 — `needs-human` becomes a
  Tag, not a Status value. Unrepresentable until 0015 lands.

Autonomous-run policy is maintained in `AGENTS.md`.
