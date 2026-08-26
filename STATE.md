# State — git-erg

_Last updated: 2026-08-26T09:42Z — Two rounds since the June refresh. (1) Red-team robustness raid closed 0248–0253: `erg close` auto-archives into closed/ (#313, #317), archive destination-collision is a hard error (#322), next-id no longer misses IDs on symlink paths / `/HEAD` branches / ±stems (#320), `erg close` arg-parsing and state robustness (#321), and refs became URI-references with capability resolution (#323, #324) — superseding 0252 (#319); docs verified against behavior along the way (#318). 0247 CONTRIBUTING drift closed (#310). (2) Design round: 0270 filed the git-mr vision — a composable local merge-request sibling — settled as one codebase / two specs (#326, merged). Queue: 0270 open, `needs-human` — implementation blocked on maintainer arbitration of the ticket's open decisions._

## North star:

An agent-friendly local ticket system for development in disconnected environments.

## Milestones

- Audits (incl. fang-audit 2026-06-05: FANG-AUDIT.md, all 3 gaps closed), dogfood
  migration (0216), guard sweep, scope confinement (0237–0239), robustness raid
  (0248–0253) complete. Queue: 0270 (`needs-human`). Bar for new work:
  "verified empirical need" (AGENTS.md).

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
<!-- generated 2026-08-26T09:42Z -->

**Recent commits:**
  2b74db9 Merge pull request #326 from MinhHaDuong/t0270-design-git-mr
  b8b91ee ticket(0270): apply decorrelated review fixes (FIX-FIRST round)
  5d67322 ticket(0270): consign the implementation scheme; settle (c) one codebase, two specs
  1c1eb6e ticket(0270): design git-mr, a composable local merge-request sibling
  3a2c0cc chore: rebuild bootstrap binary [skip ci]
