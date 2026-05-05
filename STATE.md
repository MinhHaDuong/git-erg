# State — git-erg

_Last updated: 2026-05-05 — raid: 13 tickets (0054–0066) executed and merged (#64–#74); all green._

## Stats

- Tickets: 67 total — 58 closed (tickets/closed/), 8 archived (tickets/archive/), 1 open
- Tests: green — ALL TESTS PASSED (validate:21, check:12, ready:21, update:9, close:20, migrate:11, next-id:9, log:10, new:10, init:15, main:5, archive:15, pipeline:6) + unit tests (coverage: 22.8%)
- Open PRs: none

## Ready to work

- **#0067** — `erg check`: warn if Go source files found in `tickets/tools/go/` outside git-erg project. Fully specified, no blockers.

## Blocked

None.

## Notes

- **`erg archive`** is live — moves closed tickets from `tickets/` to `tickets/closed/`, skipping any that are still blocking open tickets. Run after closing tickets.
- **`erg new`** is live — creates a ticket atomically with the next sequential ID.
- **`erg log`** is live — appends a timestamped log entry to any ticket by ID. Exits non-zero if ticket lacks `--- body ---` separator.
- **`erg validate` vs `erg check`**: validate is per-file (format/headers/refs), check is corpus-level (duplicate IDs, cycles, folder closure).
- **`erg next-id`** now scans `tickets/closed/` and `tickets/archive/` recursively — ID collision bug fixed in #67.
- **`erg version`** now reports path, hash, build date, OS/arch, and detects obsolete copies — feature added in #68.
- **Archive-dir warnings**: tickets in `tickets/archive/` with `Closed:` headers emit folder-closure warnings from `erg check`. Intentional; not actionable.
- **`Status:` is no longer part of the format.** Use `Closed:` header; run `erg migrate` to convert legacy files.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Documented in `tests/README.md`.

Autonomous-run policy is maintained in `AGENTS.md`.
