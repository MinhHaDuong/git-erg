# State — git-erg

_Last updated: 2026-05-03 — closed 0012 (design) + 0016 (cross-repo design); filed 0022 (implement 0012) + 0023 (distribution model)_

## Stats

- Tickets: 23 total — 13 closed, 10 open (5 ready, 4 blocked)
- Tests: green
- Open PRs: none

## Ready to work

| #    | Title                                                              | Notes |
|------|--------------------------------------------------------------------|-------|
| 0014 | Modularize Go source + godoc                                       | Prereq for 0008 |
| 0015 | Add Tags header for revocable triage labels                        | Land after 0022 in practice |
| 0017 | Cross-repo Blocked-by — spec change and validator                  | Unblocks 0018/0019 |
| 0022 | Implement 0012 — drop Status header, add Closed: marker, erg migrate | Beat-ready |
| 0023 | Distribution model — design                                        | Human gate; objectives need confirmation |

0001 is the open half of the validator fixture pair (0001 + 0002) — leave open
indefinitely; it has no actionable work.

## Blocked

| #    | Title                                        | Blocked by       |
|------|----------------------------------------------|------------------|
| 0002 | Sample blocked                               | 0001 (fixture)   |
| 0008 | Add erg pick command                         | 0014, 0015       |
| 0018 | Cross-repo resolver + erg ready integration  | 0017             |
| 0019 | erg graph cross-repo rendering               | 0018             |

## Sequencing

1. **0022** (drop Status + migrate) — spec breaking change; land before 0015.
2. **0014** (modularize) — unblocks 0008.
3. **0015** (Tags header) — unblocks 0008 (needs 0022 + 0014 both landed).
4. **0017** (cross-repo spec) — unblocks 0018 → 0019 chain.
5. **0023** (distribution model) — human gate; resolve before next install-path work.

## Notes

- **`needs-human` status gap:** resolved by 0015 — `needs-human` becomes a Tag, not a Status value. Unrepresentable until 0015 lands.
