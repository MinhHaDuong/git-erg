# State — git-erg

_Last updated: 2026-06-04T20:45Z — Today: spec authority preamble ratified (0228, #273). Guards raid closed 0225 + 0230 + 0233–0236 (PRs #276–#283, all APPROVED r1): docs drift guard live in CI, CMDS-coverage meta-test, hook fixtures install the shipped hook, offline-grep scoped to Go source, charter contracts promoted to AGENTS.md/README. Earlier waves: raid 219–224 and 0223/0224/0229 (#268–#275)._

## North star:

An agent-friendly local ticket system for development in disconnected environments.

## Milestones

- Audits, dogfood migration (0216), and guard sweep (raid 2026-06-04) complete.
  Queue: 0237 only. Bar for new work stays "verified empirical need" per AGENTS.md.

## Notes

- **erg validate vs erg check**: validate is per-file; check is corpus-level.
- **CI**: bootstrap binary rebuilt automatically on every push to main changing `src/go/`.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Test fixture IDs ≥9000 reserved for unclaimed-ticket tests.

## Deferred ideas

Premature, unproven, or waiting on evidence. Do not promote without AGENTS.md bar met.

- feat: O(1) everything, tickets store cache.
- feat: Pre-create 00XX-is-next-ticket.erg.
- feat: Flag blocking tags as a first-class concept.
- feat: erg new with body on the line.
- audit: usage in idh.
- feat: AI script to realign docs and code (partial: 0232)

## Status
<!-- generated 2026-06-04T20:40Z -->

**Tickets:** 1 ready · 0 blocked — `erg ready tickets/` for full list
**Recent commits:**
  448064b Merge pull request #283 from MinhHaDuong/t0230
  0d7a314 ticket(0230): close and archive — PR #283
  a34bf35 ticket(0237): guard for install-only-outside-mutator invariant
  7314ed8 doc(0230): promote charter-only contracts to AGENTS.md and README
  f830046 ticket(0230): import raid annotations
