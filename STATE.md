# State — git-erg

_Last updated: 2026-06-04T08:10Z — Since 05-30: the 0204 imagine-charter raid landed (0205–0215, PRs #239–#254: init/install split, spec/integration print-on-demand, erg-github forge layer, .erg-assets manifest, drift warning, version role). The 0216 dogfood campaign migrated all 8 machine stores to rev 9c23c37 (closed, #257). gofmt smart-quote policy settled (0217). Open: 0219 (hook hardcodes main), 0220 (migrate folds wrapped log lines) — both evidence-backed from the campaign._

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
- feat: tests guaranteeing fundamental design criteria (disconnected, agnostic, discoverable)
- feat: AI script to realign docs and code
- audit: go coding practices, lint, smells

## Status
<!-- generated 2026-06-04T08:08Z -->
**Tickets:** 2 ready · 0 blocked — `erg ready tickets/` for full list
**Recent commits:**
  493ecfd Merge pull request #234 from MinhHaDuong/worktree-housekeeping-state-2026-05-30
  6284b53 Merge pull request #257 from MinhHaDuong/t0216-close
  3e8a074 ticket(0216): close and archive — dogfood migration campaign complete
