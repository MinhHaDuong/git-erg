# State — git-erg

_Last updated: 2026-05-03 — closed 0017 (cross-repo Blocked-by spec + validator) via PR #19; migrated 0024–0029 stale Status: headers; 0018 now ready._

## Stats

- Tickets: 29 total — 16 closed, 13 open (6 ready, 6 blocked, 1 fixture)
- Tests: green
- Open PRs: none

## Ready to work

| #    | Title                                                              | Notes |
|------|--------------------------------------------------------------------|-------|
| 0015 | Add Tags header for revocable triage labels                        | Beat-ready; unblocks 0008 |
| 0018 | Cross-repo resolver + erg ready integration                        | Unblocked by 0017; network + cache + auth |
| 0023 | Distribution model — design                                        | Human gate; objectives need confirmation |
| 0024 | Integration tests for `erg graph` and `erg next-id`                | Test backfill; unblocks 0026/0028 |
| 0025 | Cover `archive --execute` and `update` replace path                | Test backfill; destructive paths |
| 0029 | Installer round-trip test and harness hardening                    | Test backfill; independent |

0001 is the open half of the validator fixture pair (0001 + 0002) — leave open
indefinitely; it has no actionable work.

## Blocked

| #    | Title                                        | Blocked by       |
|------|----------------------------------------------|------------------|
| 0002 | Sample blocked                               | 0001 (fixture)   |
| 0008 | Add erg pick command                         | 0015             |
| 0019 | erg graph cross-repo rendering               | 0018             |
| 0026 | Go unit tests for parser and validator       | 0024             |
| 0027 | CI add `go test` and coverage                | 0026             |
| 0028 | Fill remaining branch coverage               | 0024             |

## Sequencing

1. **0018** (cross-repo resolver) — completes the 0016 chain; unblocks 0019.
2. **0015** (Tags header) — unblocks 0008.
3. **0023** (distribution model) — human gate; resolve before next install-path work.
4. **0024** (graph/next-id tests) — gate for 0026/0028 test-coverage chain.

## Notes

- **`Status:` is no longer part of the format.** Closure is now derived
  from the path component test or a `Closed: <reason>` preamble header.
  `erg validate` rejects any `Status:` line; `erg migrate` is the only
  command that tolerates it (in order to convert it).
- **`needs-human` status gap:** resolved by 0015 — `needs-human` becomes a
  Tag, not a Status value. Unrepresentable until 0015 lands.
