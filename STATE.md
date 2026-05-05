# State — git-erg

_Last updated: 2026-05-05 — UX sweep: 10 tickets (0067–0076) + #86 hotfix merged; all green._

## Stats

- Tickets: 77 total — 68 closed (tickets/closed/), 8 archived (tickets/archive/), 1 open
- Tests: green — ALL TESTS PASSED (validate:26, check:22, ready:21, update:12, close:21, migrate:11, next-id:9, log:10, new:12, init/uninstall:19, main:5, archive:17, pipeline:6, help:21) + unit tests (coverage: 22.2%)
- Open PRs: none

## Ready to work

No actionable tickets open (0001 is a fixture).

## Blocked

None.

## Notes

- **`erg archive`** is live — moves closed tickets from `tickets/` to `tickets/closed/`, skipping any that are still blocking open tickets. Run after closing tickets.
- **`erg new`** is live — creates a ticket atomically with the next sequential ID.
- **`erg log`** is live — appends a timestamped log entry to any ticket by ID. Exits non-zero if ticket lacks `--- body ---` separator.
- **`erg validate` vs `erg check`**: validate is per-file (format/headers/refs), check is corpus-level (duplicate IDs, cycles, folder closure).
- **`erg next-id`** now scans `tickets/closed/` and `tickets/archive/` recursively — ID collision bug fixed in #67.
- **`erg version`** now reports path, hash, build date, OS/arch, and detects obsolete copies — feature added in #68.
- **`resolveAuthor()`**: new function in `author.go`; fallback chain `$ERG_AUTHOR → git config user.name → $USER → "unknown"`; sanitized against newline injection.
- **UX convention**: all usage strings and doc-comments use UPPER_CASE for required args (e.g. `FILE...`, `ID`, `TITLE`) and `[LOWER]` for optional args.
- **`check` output prefix**: `WARN` (was `WARNING`). Count is pluralized: `1 warning` / `N warnings`.
- **Archive-dir warnings**: tickets in `tickets/archive/` with `Closed:` headers emit folder-closure warnings from `erg check`. Intentional; not actionable.
- **`Status:` is no longer part of the format.** Use `Closed:` header; run `erg migrate` to convert legacy files.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Documented in `tests/README.md`.

Autonomous-run policy is maintained in `AGENTS.md`.
