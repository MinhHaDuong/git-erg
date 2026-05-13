# State — git-erg

_Last updated: 2026-05-12T14:00Z — Closed 0131 (YAGNI). All tickets closed. 0 open, 0 deferred._

## North star:

An agent-friendly local ticket system for development in disconnected environments.

## Milestones

- Feature freeze. Let's not spend more time on that tooling this month, we have a conference to prepare. Only correctness fixes.

## Stats

- Tickets: 137 closed, 0 open
- Tests: green — ok git-erg (37.3% coverage)
- Open PRs: none

## Notes

- **erg validate vs erg check**: validate is per-file; check is corpus-level.
- **CI**: bootstrap binary rebuilt automatically on every push to main changing `src/go/`.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Test fixture IDs ≥9000 reserved for unclaimed-ticket tests.

## Deferred ideas, nice to have or never-ending
- feat: O(1) everything, tickets store cache.
- feat: Pre-create 00XX-is-next-ticket.erg.
- feat: erg list + filters.
- feat: Flag blocking tags, make erg ready an alias of erg list not blocking.
- feat: erg new with body on the line.
- audit: usage in idh.
- audit: UX friendliness for beginner, for expert trying out, for other agents, for developper
- audit security: never loose data, never leak keys
- feat: tests guaranteeing fundamental design criteria (disconnected, agnostic, discoverable)
- feat: AI script to realign docs and code
- audit: go coding practices, lint, smells

## Status
<!-- generated 2026-05-12T13:52Z -->

**Recent commits:**
  d1dcb0d docs: update erg-manual.md Generated-from header to current binary
  c1881d3 chore(0131): close — YAGNI, shell test suites already cover CLI surface
  759df32 chore: refresh STATE.md — 135 closed, 1 deferred, 0 open
  9d05413 chore: archive 0136 (closed via Closed: header in earlier commit)
  2377a8b docs(0129): document URL-shortcut Blocked-by ref grammar (spec-only) (#144)
