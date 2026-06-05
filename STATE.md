# State — git-erg

_Last updated: 2026-06-05T08:50Z — Today: scope-confinement guard landed end-to-end (0237 remote #285, 0238 raid #287, 0239 direct #288 — all APPROVED r1, zero bumps). Ops: repo auto-merge enabled; traveling binary 8331a94 propagated to all 5 consuming repos via PRs; .claude/tmp gitignored (#290); morning-healthcheck routine retired; rtk 0.42.1. Yesterday: spec authority ratified (0228); guards raid 0225+0230+0233–0236._

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
<!-- generated 2026-06-05T08:38Z -->

**Recent commits:**
  66ae418 Merge pull request #290 from MinhHaDuong/gitignore-claude-tmp
  6805f60 chore: gitignore .claude/tmp/
  adb55aa Merge pull request #289 from MinhHaDuong/housekeeping-20260605
  91986af chore: housekeeping -- refresh STATE.md after scope-confinement trilogy
  32f5baf Merge pull request #288 from MinhHaDuong/t0239
