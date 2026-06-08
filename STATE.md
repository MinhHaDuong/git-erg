# State — git-erg

_Last updated: 2026-06-08T13:21Z — Closed 0242 (#301): pre-commit hook remediation text now build-system-agnostic — works for vendored-binary adopters (IDH) with no `make build` target; filed from cross-repo IDH tracker 0231 (IDH re-vendors on its own). Prior: 0241 close-without-archive escape (#298, erg check + pre-push hook); fang-audit (#293/#294/0240); scope-confinement trilogy 0237–0239. Queue empty._

## North star:

An agent-friendly local ticket system for development in disconnected environments.

## Milestones

- Audits (incl. fang-audit 2026-06-05: FANG-AUDIT.md, all 3 gaps closed), dogfood
  migration (0216), guard sweep, scope confinement (0237–0239) complete.
  Queue: empty. Bar for new work: "verified empirical need" (AGENTS.md).

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
<!-- generated 2026-06-08T13:21Z -->

**Recent commits:**
  8e825b3 chore: rebuild bootstrap binary [skip ci]
  616b023 Merge pull request #301 from MinhHaDuong/t0242
  d65e309 ticket(0242): close and archive
  917056b feat(0242): build-system-agnostic hook remediation text
  3046aea Merge pull request #300 from MinhHaDuong/chore/hook-template-agnostic-remediation
