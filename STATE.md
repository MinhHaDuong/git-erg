# State — git-erg

_Last updated: 2026-05-05 — raid: filed tickets 0044–0053 (test hygiene), all 11 executed as PRs; 11 open PRs awaiting author merge._

## Stats

- Tickets: 55 total — 34 closed (tickets/closed/), 8 archived (tickets/archive/), 11 open, 2 fixtures
- Tests: green on main — ALL TESTS PASSED (validate:36, check:13, ready:21, update:9, close:20, migrate:11, nextid:9, log:9, new:16, init:15, main:5, archive:14, pipeline:pending merge)
- Open PRs: 11 (#53–#63)

## Open PRs (await author merge)

| PR  | Ticket | Title |
|-----|--------|-------|
| #53 | 0046 | refactor(test): remove 6 subsumed structural checks from test_new.sh |
| #54 | 0043 | feat(test): add unwired-suite guard to _test-lint target |
| #55 | 0044 | fix(test): eliminate double-run pattern in test_validate.sh |
| #56 | 0047 | docs(test): add sh/go testing layers policy to README.md |
| #57 | 0045 | fix(test): convert fixture paths to mktemp -d in 5 test files |
| #58 | 0052 | style(test): align test_update.sh with harness conventions |
| #59 | 0048 | test(go): add unit tests for slugify and appendLogLine |
| #60 | 0053 | fix(test): tighten JSON assertions in test_ready.sh |
| #61 | 0051 | test: add end-to-end pipeline integration test |
| #62 | 0050 | test: add ID-mode blocker guard + missing-separator guard in log |
| #63 | 0049 | refactor(test): remove sh/go overlap from test_validate.sh |

**Merge order**: #56 (policy doc) first, then #53–#55 and #57–#58 (Wave 1) in any order, then #59–#63 (Wave 2).

**After all merges, expected test delta:**
- validate: 36 → 21 cases (15 duplicates removed)
- new: 16 → 10 cases (6 subsumed checks removed)
- archive: 14 → 16 (+2 ID-mode blocker tests)
- log: 9 → 10 (+1 missing-separator guard test)
- pipeline: new suite (+6 cases)
- Go unit tests: +11 new cases (8 slugify + 3 appendLogLine)

**Local main note**: 3 commits on local main not yet on origin/main (ticket file batch + fixes from 2026-05-05 session). After merging PRs, reconcile with `git pull --rebase origin main` and resolve any ticket-file conflicts by taking origin's (closed) versions.

## Ready to work

None — all open tickets have open PRs.

## Blocked

None.

## Notes

- **`erg archive`** is live — moves closed tickets from `tickets/` to `tickets/closed/`, skipping any that are still blocking open tickets. Run after closing tickets.
- **`erg new`** is live — creates a ticket atomically with the next sequential ID.
- **`erg log`** is live — appends a timestamped log entry to any ticket by ID. Now exits non-zero if ticket lacks `--- body ---` separator (guard added in PR #62).
- **`erg validate` vs `erg check`**: validate is per-file (format/headers/refs), check is corpus-level (duplicate IDs, cycles, folder closure).
- **`erg next-id` gap**: only scans `tickets/`, not `tickets/closed/`. New IDs may alias closed tickets — `erg check` catches the duplicate. Known limitation, not yet ticketed.
- **Archive-dir warnings**: tickets in `tickets/archive/` with `Closed:` headers emit folder-closure warnings from `erg check`. Intentional; not actionable.
- **`Status:` is no longer part of the format.** Use `Closed:` header; run `erg migrate` to convert legacy files.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Documented in tests/README.md (PR #56).

Autonomous-run policy is maintained in `AGENTS.md`.
