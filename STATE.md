# State — git-erg

_Last updated: 2026-06-05T07:55Z — Today: scope-confinement guard landed end-to-end across three execution modes: 0237 via overnight remote routine (#285), 0238 via raid (#287, snapshot now content-aware via cksum), 0239 direct (#288, hermetic overwrite control). All APPROVED r1, zero bumps. Yesterday: spec authority ratified (0228, #273); guards raid 0225+0230+0233–0236 (#276–#283)._

## North star:

An agent-friendly local ticket system for development in disconnected environments.

## Milestones

- Audits, dogfood migration (0216), guard sweep, scope confinement (0237–0239)
  complete. Queue: empty. Bar for new work: "verified empirical need" (AGENTS.md).

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
<!-- generated 2026-06-05T07:55Z -->

**Tickets:** 0 ready · 0 blocked — `erg ready tickets/` for full list
**Recent commits:**
  32f5baf Merge pull request #288 from MinhHaDuong/t0239
  f5ef748 ticket(0239): close and archive — PR #288
  9d9ecd9 Merge pull request #287 from MinhHaDuong/t0238
  80a5a26 Merge pull request #286 from MinhHaDuong/ticket-snapshot-digest
  8331a94 Merge pull request #285 from MinhHaDuong/t0237
