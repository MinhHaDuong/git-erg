# State — git-erg

_Last updated: 2026-08-26T10:45Z — Design of the local merge-request sibling is complete and closed. Maintainer arbitrated 0270's four open decisions (#328): log inline in one erg-shaped file (log = one-liners, body = prose discussion, opening post = description, `merge=union` on the store); no archive numbering — the merge SHA is the MR's name; grammar mini-spec deferred; and the tool is **dyne** (CGS force: erg = dyne × cm — an MR is a force applied to main, work done when main moves), standalone binary, `.dyne` files, `.dynerc`, store `merges/`. 0270 closed with phase 1 filed: 0271 (PEP) → 0272 (%dyne spec) → 0273 (binary + open/show/list/check) → 0274 (init, embedded docs, guards), 0275 (outbound PR-body bridge). Queue: 0271 ready, rest chained by Blocked-by._

## North star:

An agent-friendly local ticket system for development in disconnected environments.

## Milestones

- Audits (incl. fang-audit 2026-06-05: FANG-AUDIT.md, all 3 gaps closed), dogfood
  migration (0216), guard sweep, scope confinement (0237–0239), robustness raid
  (0248–0253), dyne design (0270) complete. Queue: dyne phase 1 (0271–0275,
  0271 ready). Bar for new work: "verified empirical need" (AGENTS.md).

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
<!-- generated 2026-08-26T10:45Z -->

**Recent commits:**
  c6fcef5 Merge pull request #328 from MinhHaDuong/claude/repo-orientation-uzwkkt
  c795131 ticket(0270): name the MR tool dyne
  4279566 ticket(0270): maintainer arbitrates the four open decisions
  7cc538d Merge pull request #327 from MinhHaDuong/claude/repo-orientation-uzwkkt
  6990111 docs(state): refresh after 0248-0253 raid and 0270 design round
