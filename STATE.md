# State — git-erg

_Last updated: 2026-05-12T14:00Z — Closed 0131 (YAGNI). All tickets closed. 0 open, 0 deferred._

## Stats

- Tickets: 137 closed, 0 open
- Tests: green — ok git-erg (37.3% coverage)
- Open PRs: none

## Ready to work

_(none)_

## Deferred

_(none)_

## Notes

- **0116 chain plan**: 0116 → 0117 → 0118. Each delivers value standalone. Endpoint: schema-pure `Erg` with `Created time.Time`, `BlockedBys []Ref`, zero accessor methods except `IsClosed()`. All three closed.
- **erg validate vs erg check**: validate is per-file; check is corpus-level.
- **CI**: bootstrap binary rebuilt automatically on every push to main changing `src/go/`.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Test fixture IDs ≥9000 reserved for unclaimed-ticket tests.
- **Blocked-by grammar**: now supports `path-ref` (e.g. `module/0042`) for cross-module refs in multi-module monorepos; resolver implementation deferred.

Autonomous-run policy is maintained in `AGENTS.md`.

## Status
<!-- generated 2026-05-12T13:52Z -->

**Recent commits:**
  d1dcb0d docs: update erg-manual.md Generated-from header to current binary
  c1881d3 chore(0131): close — YAGNI, shell test suites already cover CLI surface
  759df32 chore: refresh STATE.md — 135 closed, 1 deferred, 0 open
  9d05413 chore: archive 0136 (closed via Closed: header in earlier commit)
  2377a8b docs(0129): document URL-shortcut Blocked-by ref grammar (spec-only) (#144)
