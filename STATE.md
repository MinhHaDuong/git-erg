# State — git-erg

_Last updated: 2026-05-05 — session closed 0036/0038/0039/0040/0041/0042; landed erg log, erg new, erg archive, validate/check split; 1 open ticket._

## Stats

- Tickets: 44 total — 34 closed (tickets/closed/), 8 archived (tickets/archive/), 1 open + fixtures in archive/
- Tests: green — ALL TESTS PASSED (validate:36, check:13, ready:21, update:9, close:20, migrate:11, nextid:9, log:9, new:16, init:15, main:5, archive:14)
- Open PRs: none

## Ready to work

| #    | Title                                                              | Notes |
|------|--------------------------------------------------------------------|-------|
| 0043 | Guard against unwired test suites in Makefile                      | Add `_test-lint` cross-reference check for test_*.sh vs TEST_SUITES |

## Blocked

None.

## Sequencing

1. **0043** — small, self-contained, direct follow-up from session.

## Notes

- **`erg archive`** is live — moves closed tickets from `tickets/` to `tickets/closed/`, skipping any that are still blocking open tickets. Run after closing tickets.
- **`erg new`** is live — creates a ticket atomically with the next sequential ID.
- **`erg log`** is live — appends a timestamped log entry to any ticket by ID.
- **`erg validate` vs `erg check`**: validate is per-file (format/headers/refs), check is corpus-level (duplicate IDs, cycles, folder closure).
- **`erg next-id` gap**: only scans `tickets/`, not `tickets/closed/`. New IDs may alias closed tickets — `erg check` catches the duplicate. Known limitation, not yet ticketed.
- **Archive-dir warnings**: tickets in `tickets/archive/` with `Closed:` headers emit folder-closure warnings from `erg check`. Intentional; not actionable.
- **`Status:` is no longer part of the format.** Use `Closed:` header; run `erg migrate` to convert legacy files.

Autonomous-run policy is maintained in `AGENTS.md`.
