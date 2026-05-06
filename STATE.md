# State — git-erg

_Last updated: 2026-05-06 — Raid: PRs #93–#98 merged (tickets 0083–0094). 8 open tickets in spec-as-code wave._

## Stats

- Tickets: 94 total — 86 closed/archived, 8 open (0088–0093 spec-as-code wave)
- Tests: green — ALL TESTS PASSED (validate, check, ready, update, close, migrate, nextid, log, new, init, main, archive, pipeline, help, version, hook) + unit tests
- Open PRs: none

## Ready to work

- 0088 spec-as-code umbrella (no blockers)
- 0089 enrich Go doc comments (no blockers)
- 0092 cull spec to format-only (no blockers)

## Blocked

- 0090 per-command --help (blocked by 0089)
- 0091 erg --help --all + make docs (blocked by 0090)
- 0093 spec-as-code foundation: ergspecv1.go (blocked by 0088)

## Notes

- **Pre-commit hook** (PR #98): now also rejects `tickets/erg` commits on non-main branches. CI rebuilds the binary after merge; feature PRs must use `make build` and test with `build/erg`. Error message names `--no-verify` override. `tests/test_hook.sh` covers the guard.
- **`erg migrate`** (PR #94): upgrades full project layout — removes `tickets/tools/`, `tickets/FORMAT.md`; renames `archive/` → `closed/`; refreshes init assets via `cmdInit`. Runs only when dir is named `tickets`. Idempotent.
- **`erg version`** (PR #95): now discovers `./tickets/erg` as a candidate when run from the project root.
- **`erg --help`** (PR #96): usage text goes to stdout; header shows `[--help]`; `erg COMMAND --help` exits 0. Per-command detail deferred to ticket 0090.
- **`erg init` simplified (PR #92)**: unpacks 3 files (AGENTS.md, spec-erg-v1.md, integration.md); requires `tickets/erg` present first.
- **Spec-as-code plan**: spec-erg-v1.md shrinks to format-only; command docs move to Go doc comments; `erg COMMAND --help` serves per-command help; `erg --help --all` assembles full manual; `make docs` generates static erg-manual.md. Tickets 0088–0093.
- **`erg validate` vs `erg check`**: validate is per-file (format/headers/refs), check is corpus-level (duplicate IDs, cycles, folder closure).
- **`erg next-id`** scans `tickets/closed/` and `tickets/archive/` recursively.
- **`erg version`** reports path, hash, build date, OS/arch, and detects obsolete copies.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior.
- **CI**: bootstrap binary (`tickets/erg`) is rebuilt automatically on every push to main that changes Go source (`src/go/`) — no manual step needed.

Autonomous-run policy is maintained in `AGENTS.md`.
