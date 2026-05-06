# State — git-erg

_Last updated: 2026-05-06 — Session: PRs #103–#104 merged (0092, 0095); 0088 umbrella closed; manual enriched; panel review → tickets 0096–0103._

## Stats

- Tickets: 103 total — 94 closed/archived, 9 open (0093, 0096–0103)
- Tests: green — ALL TESTS PASSED
- Open PRs: none

## Ready to work

- 0093 spec-as-code foundation: ergspecv1.go, struct cleanup, golden fixtures (no blockers)
- 0096 Umbrella: align spec, PEP, and manual (no blockers; close when 0097–0103 done)

## Blocked

- 0097–0103 all blocked by 0096 (open the umbrella to release them)

## Notes

- **Spec-as-code wave complete** (tickets 0088–0092, 0095): spec-erg-v1.md is now format-only (147 lines); command docs live in `erg COMMAND --help`; `erg --help --all` generates `docs/erg-manual.md` with H1 title, author, build metadata, and intro paragraph.
- **Align spec/PEP/manual wave** (tickets 0096–0103): opened after panel review of the manual. Covers one code bug (trailing period, 0097), manual contradictions and prose (0098–0101), spec sync (0102), and PEP sync (0103).
- **0093 spec-as-code foundation**: new `src/go/ergspecv1.go` with all format constants, struct cleanup (drop HasLog/HasBody), tighter parseRef, and golden test fixtures. Substantial ticket — consider splitting if it runs long.
- **erg validate vs erg check**: validate is per-file; check is corpus-level.
- **CI**: bootstrap binary rebuilt automatically on every push to main changing `src/go/`.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Test fixture IDs ≥9000 reserved for unclaimed-ticket tests.

Autonomous-run policy is maintained in `AGENTS.md`.
