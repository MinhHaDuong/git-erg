# State — git-erg

_Last updated: 2026-05-10 — Housekeeping: pruned 4 orphaned verify worktrees; all else clean._

## Stats

- Tickets: 111 closed, 0 open
- Tests: green — ok git-erg 0.018s (24.4% coverage)
- Open PRs: none

## Ready to work

- No open tickets. Open a new one to continue.

## Blocked

- Nothing blocked.

## Notes

- **Raid wave complete** (2026-05-08, PRs #110–#118): closed all 9 open tickets (0097–0103, 0105, 0106, 0110) plus umbrella 0096. Work includes: staleBlockedBy check in `erg check`, ticket.go Headers→headers encapsulation with typed accessors, help.go/main.go prose fixes, spec-erg-v1.md + pep-erg-v1.md sync, close.go trailing period fix, ticket 0106 wontfix (Tags rename wrong direction).
- **erg validate vs erg check**: validate is per-file; check is corpus-level.
- **CI**: bootstrap binary rebuilt automatically on every push to main changing `src/go/`.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Test fixture IDs ≥9000 reserved for unclaimed-ticket tests.

Autonomous-run policy is maintained in `AGENTS.md`.
