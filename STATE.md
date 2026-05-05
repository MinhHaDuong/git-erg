# State — git-erg

_Last updated: 2026-05-05 — Code smell sweep (0077) + ready perf fix (0078); PRs #87/#88 merged; all green._

## Stats

- Tickets: 79 total — 70 closed (tickets/closed/), 8 archived (tickets/archive/), 1 open
- Tests: green — ALL TESTS PASSED (validate:26, check:22, ready:21, update:12, close:21, migrate:11, next-id:9, log:10, new:12, init/uninstall:19, main:5, archive:17, pipeline:6, help:21) + unit tests (coverage: 22.2%)
- Open PRs: none

## Ready to work

No actionable tickets open (0001 is a fixture).

## Blocked

None.

## Notes

- **`erg ready` perf**: branch-claim check now uses one `git branch -a` spawn (was 2×N). O(1) regardless of ticket count.
- **`init` manifest fix**: re-running `erg init` no longer clobbers ownership flags — uninstall correctly removes only entries that init added.
- **`erg ready --json`**: output uses `encoding/json` (MarshalIndent); inner arrays are expanded. Downstream scripts should use `jq` rather than grepping raw format.
- **`erg archive`** is live — moves closed tickets from `tickets/` to `tickets/closed/`, skipping any that are still blocking open tickets. Run after closing tickets.
- **`erg new`** is live — creates a ticket atomically with the next sequential ID.
- **`erg log`** is live — appends a timestamped log entry to any ticket by ID.
- **`erg validate` vs `erg check`**: validate is per-file (format/headers/refs), check is corpus-level (duplicate IDs, cycles, folder closure).
- **`erg next-id`** scans `tickets/closed/` and `tickets/archive/` recursively — ID collision bug fixed in #67.
- **`erg version`** reports path, hash, build date, OS/arch, and detects obsolete copies.
- **`resolveAuthor()`**: fallback chain `$ERG_AUTHOR → git config user.name → $USER → "unknown"`; sanitized against newline injection.
- **UX convention**: all usage strings use UPPER_CASE for required args, `[LOWER]` for optional.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. See `tests/README.md`.
- **CI**: bootstrap binary (`tickets/tools/go/erg`) is rebuilt automatically on every push to main that changes Go source — no manual step needed.

Autonomous-run policy is maintained in `AGENTS.md`.
