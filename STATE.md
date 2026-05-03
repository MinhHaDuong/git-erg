# State — git-erg

_Last updated: 2026-05-03 — post 0021 (fix go vet), 0007 (install binary on PATH, PR #15)_

## Stats

- Tickets: 21 total — 12 closed, 9 open (5 ready, 4 blocked)
- Tests: green
- Open PRs: none

## Ready to work

| #    | Title                                                              | Notes |
|------|--------------------------------------------------------------------|-------|
| 0012 | Drop Status header — derive closed-ness from path or marker        | Design questions still open |
| 0014 | Modularize Go source + godoc                                       | Prereq for 0008 |
| 0015 | Add Tags header for revocable triage labels                        | Independent |
| 0016 | Cross-repo blocker references — design umbrella                    | Unblocks 0017/0018/0019 |

0001 is the open half of the validator fixture pair (0001 + 0002) — leave open
indefinitely; it has no actionable work.

## Blocked

| #    | Title                                        | Blocked by       |
|------|----------------------------------------------|------------------|
| 0002 | Sample blocked                               | 0001 (fixture)   |
| 0008 | Add erg pick command                         | 0014             |
| 0017 | Cross-repo spec + validator                  | 0016             |
| 0018 | Cross-repo resolver + erg ready integration  | 0016, 0017       |
| 0019 | erg graph cross-repo rendering               | 0016, 0017, 0018 |

## Sequencing

1. **0014** (modularize) — unblocks 0008.
2. **0016** (cross-repo design) — unblocks 0017 → 0018 → 0019 chain.
3. **0012** — design questions open; resolve before implementing.
4. **0015** (Tags header) — independent, land anytime.

## Notes

- **`needs-human` status gap:** referenced in 0008 body but not in the spec enum (`open|doing|closed|pending`). Resolve when 0015 (Tags) lands — `needs-human` will likely become a tag, not a Status value.
