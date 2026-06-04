# State — git-erg

_Last updated: 2026-06-04T09:45Z — Since 05-30: the 0204 imagine-charter raid landed (0205–0215, PRs #239–#254: init/install split, spec/integration print-on-demand, erg-github forge layer, .erg-assets manifest, drift warning, version role). The 0216 dogfood campaign migrated all 8 machine stores to rev 9c23c37 (closed, #257). gofmt smart-quote policy settled (0217). Open: 0219 (hook hardcodes main), 0220 (migrate folds wrapped log lines) — both evidence-backed from the campaign._

## North star:

An agent-friendly local ticket system for development in disconnected environments.

## Milestones

- Audits complete (UX 0152, security 0151, data-safety 0149); dogfood migration
  complete machine-wide (0216). Queue: 0219, 0220 — both empirically motivated.
  Bar for new work stays "verified empirical need" per AGENTS.md.

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
<!-- generated 2026-06-04T09:45Z -->
**Tickets:** 11 ready · 2 blocked — `erg ready tickets/` for full list
**Recent commits:**
  1643ddb ticket(0225-0232): foundations audit follow-ups -- guards, reconciliations, standing QA
  52cda8f ticket(0223,0224): update&&init delivery doc; migrate must stop clobbering .ergrc
  88bca90 ticket(0221): close and archive — PR #262
