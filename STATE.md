# State — git-erg

_Last updated: 2026-05-05 — raid: 11 tickets (0043–0053) executed and merged (#53–#63); all green._

## Stats

- Tickets: 55 total — 34 closed (tickets/closed/), 8 archived (tickets/archive/), 11 open (local, pending archive), 2 fixtures
- Tests: green — ALL TESTS PASSED (validate:21, check:13, ready:21, update:9, close:20, migrate:11, nextid:9, log:10, new:10, init:15, main:5, archive:16, pipeline:6)
- Open PRs: none

## Ready to work

None.

## Blocked

None.

## Notes

- **`erg archive`** is live — moves closed tickets from `tickets/` to `tickets/closed/`, skipping any that are still blocking open tickets. Run after closing tickets.
- **`erg new`** is live — creates a ticket atomically with the next sequential ID.
- **`erg log`** is live — appends a timestamped log entry to any ticket by ID. Exits non-zero if ticket lacks `--- body ---` separator.
- **`erg validate` vs `erg check`**: validate is per-file (format/headers/refs), check is corpus-level (duplicate IDs, cycles, folder closure).
- **`erg next-id` gap**: only scans `tickets/`, not `tickets/closed/`. New IDs may alias closed tickets — `erg check` catches the duplicate. Known limitation, not yet ticketed.
- **Archive-dir warnings**: tickets in `tickets/archive/` with `Closed:` headers emit folder-closure warnings from `erg check`. Intentional; not actionable.
- **`Status:` is no longer part of the format.** Use `Closed:` header; run `erg migrate` to convert legacy files.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Documented in `tests/README.md`.

Autonomous-run policy is maintained in `AGENTS.md`.
