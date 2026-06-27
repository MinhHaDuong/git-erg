# State — git-erg

_Last updated: 2026-06-18T10:11Z — Raid closed both #305-roar follow-ups: 0245 (#308) generalized the init-unpack ASCII guard over every unpacked asset (.ergrc, not just AGENTS.md); 0246 (#309) added a `check-store` gate (`erg check tickets/`) folded into `make test`, so store-shape violations fail locally not just in CI. Filed 0247 (#310, open): CONTRIBUTING `make test` description drifted after that gate. Prior: 0242 (#301) build-system-agnostic hook remediation. Queue: 0247 pending review._

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
<!-- generated 2026-06-18T10:11Z -->

**Recent commits:**
  74cc065 Merge pull request #309 from MinhHaDuong/t0246-regen-assets-check-store
  28c7a64 ticket(0246): close and archive — PR #309
  06101ae Merge pull request #308 from MinhHaDuong/t0245-asset-ascii-guard-ergrc
  451b3b3 ticket(0245): close and archive — PR #308
  afc74db ticket(0246): local check-store gate for ticket-store shape
