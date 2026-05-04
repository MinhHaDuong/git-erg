# State — git-erg

_Last updated: 2026-05-04 — merged PR #46 (ticket 0041: validate/check split); tickets 0037 and 0041 closed; spec-erg-v1.md updated; PRs #48 and #49 open._

## Stats

- Tickets: 41 total — 27 closed, 14 open (4 ready + fixtures/archived)
- Tests: green (140 tests)
- Open PRs: #48 (0038 fix test_ready.sh), #49 (0036 regression doc)

## Ready to work

| #    | Title                                                              | Notes |
|------|--------------------------------------------------------------------|-------|
| 0036 | Add standing regression check for shell test binary defaults       | PR #49 open — awaiting merge |
| 0038 | Fix dead grep and missing trap cleanup in test_ready.sh            | PR #48 open — awaiting merge |
| 0039 | Add `erg log <id> <line>` command                                  | New — append log entries atomically |
| 0040 | Add `erg new` command — create ticket file atomically              | New — companion to 0039 |

## Blocked

None.

## Sequencing

1. **#48 / 0038** (fix test_ready.sh) — already in PR, merge first.
2. **#49 / 0036** (regression doc) — already in PR, merge second.
3. **0039** (`erg log`) and **0040** (`erg new`) — independent, can run in parallel.
4. **0008** (`erg pick` command) — body references deprecated workflow; needs refresh before executing.

## Notes

- **`erg validate` is now file-only.** As of PR #46, `erg validate <files>` takes explicit file args; `erg check [dir]` does corpus-level validation (IDs, cycles, refs, folder closure). All callers updated.
- **`Status:` is no longer part of the format.** Closure is derived from the path component test or a `Closed:` preamble header. `erg validate` rejects any `Status:` line; `erg migrate` is the only command that tolerates it (to convert it).
- **Fixture tickets 0001/0002** live in `tickets/archive/` without `Closed:` headers — `erg ready` may surface them. Leave as-is; not actionable work.

Autonomous-run policy is maintained in `AGENTS.md`.
