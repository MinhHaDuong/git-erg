# State — git-erg

_Last updated: 2026-05-06 — Raid: PRs #100–#102 merged (tickets 0089–0091). 5 open tickets; spec-as-code wave advancing._

## Stats

- Tickets: 94 total — 91 closed/archived, 3 open (0088, 0092, 0093)
- Tests: green — ALL TESTS PASSED (validate, check, ready, update, close, migrate, nextid, log, new, init, main, archive, pipeline, help, version, hook, godoc, docs) + unit tests
- Open PRs: none

## Ready to work

- 0088 spec-as-code umbrella (no blockers)
- 0092 cull spec to format-only (no blockers)

## Blocked

- 0093 spec-as-code foundation: ergspecv1.go (blocked by 0088)

## Notes

- **Pre-commit hook** (PR #98): now also rejects `tickets/erg` commits on non-main branches. CI rebuilds the binary after merge; feature PRs must use `make build` and test with `build/erg`. Error message names `--no-verify` override. `tests/test_hook.sh` covers the guard.
- **`erg migrate`** (PR #94): upgrades full project layout — removes `tickets/tools/`, `tickets/FORMAT.md`; renames `archive/` → `closed/`; refreshes init assets via `cmdInit`. Runs only when dir is named `tickets`. Idempotent.
- **`erg version`** (PR #95): now discovers `./tickets/erg` as a candidate when run from the project root.
- **`erg --help`** (PR #96): usage text goes to stdout; header shows `[--help]`; `erg COMMAND --help` exits 0.
- **Per-command help** (PR #101): `erg COMMAND --help` now prints full per-command text from `helpText` map in `help.go`. Unknown command falls back to global usage.
- **`erg --help --all`** (PR #102): prints all 12 command help sections in declaration order. `make docs` generates `docs/erg-manual.md`. Test suite adds `godoc` and `docs` suites.
- **`erg init` simplified (PR #92)**: unpacks 3 files (AGENTS.md, spec-erg-v1.md, integration.md); requires `tickets/erg` present first.
- **Spec-as-code plan**: spec-erg-v1.md shrinks to format-only; command docs move to Go doc comments; `erg COMMAND --help` serves per-command help; `erg --help --all` assembles full manual; `make docs` generates static erg-manual.md. Tickets 0088–0093.
- **`erg validate` vs `erg check`**: validate is per-file (format/headers/refs), check is corpus-level (duplicate IDs, cycles, folder closure).
- **`erg next-id`** scans `tickets/closed/` and `tickets/archive/` recursively.
- **`erg version`** reports path, hash, build date, OS/arch, and detects obsolete copies.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior.
- **CI**: bootstrap binary (`tickets/erg`) is rebuilt automatically on every push to main that changes Go source (`src/go/`) — no manual step needed.

Autonomous-run policy is maintained in `AGENTS.md`.
