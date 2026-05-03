# State — git-erg

_Last updated: 2026-05-03 — closed 0014 (modularize) and 0022 (drop Status: / add Closed: marker / erg migrate); migrated all tickets to %erg v1 without Status._

## Stats

- Tickets: 23 total — 15 closed, 8 open (3 ready, 4 blocked, 1 fixture)
- Tests: green
- Open PRs: none

## Ready to work

| #    | Title                                                              | Notes |
|------|--------------------------------------------------------------------|-------|
| 0015 | Add Tags header for revocable triage labels                        | Beat-ready (0022 landed) |
| 0017 | Cross-repo Blocked-by — spec change and validator                  | Unblocks 0018/0019 |
| 0023 | Distribution model — design                                        | Human gate; objectives need confirmation |

0001 is the open half of the validator fixture pair (0001 + 0002) — leave open
indefinitely; it has no actionable work.

## Blocked

| #    | Title                                        | Blocked by       |
|------|----------------------------------------------|------------------|
| 0002 | Sample blocked                               | 0001 (fixture)   |
| 0008 | Add erg pick command                         | 0015             |
| 0018 | Cross-repo resolver + erg ready integration  | 0017             |
| 0019 | erg graph cross-repo rendering               | 0018             |

## Sequencing

1. **0015** (Tags header) — unblocks 0008.
2. **0017** (cross-repo spec) — unblocks 0018 → 0019 chain.
3. **0023** (distribution model) — human gate; resolve before next install-path work.

## Notes

- **`Status:` is no longer part of the format.** Closure is now derived
  from the path component test or a `Closed: <reason>` preamble header.
  `erg validate` rejects any `Status:` line; `erg migrate` is the only
  command that tolerates it (in order to convert it).
- **`needs-human` status gap:** resolved by 0015 — `needs-human` becomes a
  Tag, not a Status value. Unrepresentable until 0015 lands.
