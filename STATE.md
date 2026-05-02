# State — git-erg

_Last updated: 2026-05-02 — post 0013 (drop .wip / claim/release, PR #14)_

## Stats

- Tickets: 21 total — 10 closed, 11 open (7 ready, 4 blocked)
- Tests: `go test` blocked by pre-existing `go vet` false-positive on `%erg` in `printUsage()` — tracked in 0021
- Open PRs: none

## Ready to work

| #    | Title                                                              | Notes |
|------|--------------------------------------------------------------------|-------|
| 0007 | Install erg binary on PATH                                         | Independent — land anytime |
| 0012 | Drop Status header — derive closed-ness from path or marker        | Design questions still open |
| 0014 | Modularize Go source + godoc                                       | Prereq for 0008 |
| 0015 | Add Tags header for revocable triage labels                        | Independent |
| 0016 | Cross-repo blocker references — design umbrella                    | Unblocks 0017/0018/0019 |
| 0021 | Fix go vet false-positive in printUsage()                          | Small fix, unblocks `go test` |

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

1. **0021** — tiny fix, unblocks `go test` immediately.
2. **0007** — independent, land anytime.
3. **0014** (modularize) — unblocks 0008.
4. **0016** (cross-repo design) — unblocks 0017 → 0018 → 0019 chain.
5. **0012** — design questions open; resolve before implementing.
6. **0015** (Tags header) — independent, land anytime.

## Notes

- **`needs-human` status gap:** referenced in 0008 body but not in the spec enum (`open|doing|closed|pending`). Resolve when 0015 (Tags) lands — `needs-human` will likely become a tag, not a Status value.
