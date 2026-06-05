# State — git-erg

_Last updated: 2026-06-05T19:45Z — Today pm: fang-audit on the Go suite (19/19 files, canary PASS, 86 caught / 3 toothless): report committed (#293), the 3 fangs applied (0240, #294 — gaze r1 caught my counter-fang being itself toothless; replaced with a scaling-tag timing guard); 3 Workflow-runtime bugs in the fang-audit skill fixed (IDH 0223/#309). Today am: scope-confinement trilogy 0237–0239; auto-merge enabled; binary propagated; rtk 0.42.1._

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
<!-- generated 2026-06-05T19:42Z -->

**Recent commits:**
  818fd84 chore: rebuild bootstrap binary [skip ci]
  a8f4bef Merge pull request #294 from MinhHaDuong/t0240
  d152cb6 ticket(0240): close and archive — PR #294
  d0cf7cc test: move idExists timing fang behind //go:build scaling
  df8d1d9 test: replace toothless per-call counter with timing fang for idExists
