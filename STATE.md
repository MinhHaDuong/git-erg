# State — git-erg

_Last updated: 2026-05-06 — Raid: ticket 0082 merged (PR #92), erg init simplified to unpack-only. All 82 tickets closed._

## Stats

- Tickets: 82 total — 73 closed (tickets/closed/), 9 archived (tickets/archive/), 0 open
- Tests: green — ALL TESTS PASSED (validate:26, check:24, ready:21, update:11, close:21, migrate:11, next-id:9, log:10, new:12, init:11, main:5, archive:17, pipeline:6, help:20) + unit tests (coverage: 25.2%)
- Open PRs: none

## Ready to work

None — all tickets closed.

## Blocked

None.

## Notes

- **`erg init` simplified (PR #92)**: now unpacks exactly 3 files (README.md, spec-erg-v1.md, integration.md) and exits. Requires `tickets/erg` binary to be present first. No more hook installation, AGENTS.md automation, .gitignore editing, or manifest tracking. `erg uninstall` removed — replaced by `rm tickets/AGENTS.md tickets/spec-erg-v1.md tickets/integration.md`.
- **`erg ready` perf**: branch-claim check lazy-loads once on first unblocked ticket (0 spawns when all blocked). O(1) regardless of ticket count.
- **Pre-commit hook**: now validates staged `.erg` files individually — fixed directory-path regression introduced when `erg validate` stopped accepting directory args (PR #87).
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
- **CI**: bootstrap binary (`tickets/erg`) is rebuilt automatically on every push to main that changes Go source (`src/go/`) — no manual step needed.

Autonomous-run policy is maintained in `AGENTS.md`.
