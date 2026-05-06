# State — git-erg

_Last updated: 2026-05-06 — Session: PRs #100–#102 merged (0089–0091). Spec-as-code wave: 3 tickets remain._

## Stats

- Tickets: 94 total — 91 closed/archived, 3 open (0088, 0092, 0093)
- Tests: green — ALL TESTS PASSED (validate, check, ready, update, close, migrate, nextid, log, new, init, main, archive, pipeline, help, version, hook, godoc, docs) + unit tests
- Open PRs: none

## Ready to work

- 0088 spec-as-code umbrella (no blockers; closes when 0092 + 0093 done)
- 0092 cull spec-erg-v1.md to file-format sections only (no blockers)

## Blocked

- 0093 spec-as-code foundation: ergspecv1.go (blocked by 0088)

## Notes

- **Spec-as-code plan** (tickets 0088–0093): spec-erg-v1.md shrinks to format-only (0092); command docs live in Go doc comments (done); `erg COMMAND --help` serves per-command help (done); `erg --help --all` assembles full manual (done); `make docs` generates docs/erg-manual.md (done); ergspecv1.go provides a Go struct representation (0093).
- **Per-command help** (PR #101): `helpText` map in `src/go/help.go`; 12 commands; `commandOrder` slice drives `--help --all` iteration order.
- **`erg --help --all`** (PR #102): prints all 12 sections. `make docs` target writes `docs/erg-manual.md`. `tests/test_docs.sh` and `tests/test_godoc.sh` added.
- **Pre-commit hook** (PR #98): rejects `tickets/erg` commits on non-main branches. Feature PRs must use `make build` / `build/erg`. CI rebuilds bootstrap binary after merge to main.
- **`erg validate` vs `erg check`**: validate is per-file (format/headers/refs/cycles); check is corpus-level (duplicate IDs, cross-file cycles, folder closure).
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Test fixture IDs ≥9000 reserved for unclaimed-ticket tests (avoid branch-name collision).
- **CI**: bootstrap binary (`tickets/erg`) rebuilt automatically on every push to main changing `src/go/`. No manual step needed.

Autonomous-run policy is maintained in `AGENTS.md`.
