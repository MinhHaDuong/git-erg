# UX audit findings — first dry-run (2026-05-29)

DRAFT — severity ratings are agent-proposed, pending human review.

Procedure: `docs/ux-audit-procedure.md`. Four AI personas (beginner,
expert, agent, developer) walked through the first-five-minutes script
in fresh temp directories. Findings independently verified where marked.

## Verified bugs (reproducible, falsifiable)

### F01 — README quick-start example is wrong
- **Personas**: beginner, expert
- **Verified**: yes (independently reproduced)
- **Step**: README line 119: `tickets/erg validate 01`
- **Expected**: validates ticket 0001
- **Actual**: `WARNING: skipping 01 (stat 01: no such file or directory)` + exit 0. `validate` takes file paths, not IDs. The README's first binary example silently does nothing.
- **Draft severity**: P1 — first binary interaction fails; exit 0 masks the failure
- **Fix**: change to `tickets/erg validate tickets/0001-add-auth.erg`

### F02 — Corrupted character in embedded AGENTS.md asset
- **Personas**: beginner
- **Verified**: yes (hexdump: `ef bf bd 69` at `src/go/assets/AGENTS.md:28`)
- **Step**: `erg init` unpacks AGENTS.md with `jitter �i 500ms` where source file has `jitter ≤ 500ms`
- **Expected**: `≤` character preserved
- **Actual**: U+FFFD replacement character. Every user who runs `erg init` gets corrupted text.
- **Draft severity**: P1 — corrupted text in the first document a user reads after init
- **Fix**: replace corrupted bytes in `src/go/assets/AGENTS.md:28` with `≤`

### F03 — Stray high-ID file poisons all future ticket creation
- **Personas**: beginner
- **Verified**: yes (reproduced: `9999-bad.erg` → `next-id` returns 10000 → `erg new` creates 5-digit filename → validate rejects it)
- **Step**: place any file matching `NNNN-*.erg` with high ID in tickets/
- **Expected**: next-id ignores invalid files or caps at 4-digit range
- **Actual**: cascading failure — every subsequent `erg new` produces invalid filenames
- **Draft severity**: P1 — one stray file breaks ticket creation permanently until manually removed
- **Fix**: skip files failing validation in next-id scan, or cap ID range

## Friction findings (UX quality, not bugs)

### F04 — `erg` no-args usage goes to stdout, not stderr
- **Personas**: beginner
- **Verified**: yes
- **Actual**: usage on no-args and unknown-command goes to stdout. Breaks `erg | ...` pipelines.
- **Draft severity**: P2

### F05 — `list` command help formatting — no space separator
- **Verified**: yes
- **Actual**: `[--json]List tickets` runs together in help output (long args string overflows column)
- **Draft severity**: P3

### F06 — `erg close` does not hint about `erg archive`
- **Personas**: expert
- **Actual**: after close, `erg check` warns about closed ticket not in `closed/`. No breadcrumb from close to archive.
- **Draft severity**: P2

### F07 — AGENTS.md (init-unpacked) omits close command and pick-work workflow
- **Personas**: agent
- **Verified**: yes (grep confirms zero mentions of `erg close` or `erg ready` in init-unpacked AGENTS.md)
- **Actual**: agent-facing AGENTS.md mentions log format, Blocked-by, and tag vocabulary but never shows `erg close ID REASON` signature or `erg ready` for work-picking. First close attempt always errors.
- **Draft severity**: P1

### F08 — `erg validate` error messages don't show expected format
- **Personas**: agent
- **Actual**: `malformed log line: <bad line>` names file and line but doesn't show the expected `YYYY-MM-DDThh:mmZ actor verb [detail]` format. Not self-correcting.
- **Draft severity**: P2

### F09 — `--json` silently ignored or misparsed on validate/check/new/next-id
- **Personas**: agent
- **Actual**: `erg validate --json` treats `--json` as a filename, warns about skipping it. `erg check --json` hard-errors. `erg new --json` ignores it. Inconsistent behavior.
- **Draft severity**: P2

### F10 — No binary download link in README
- **Personas**: beginner
- **Actual**: "Drop the erg binary into it — use a prebuilt one" with no URL or download instructions.
- **Draft severity**: P1

### F11 — Pre-commit hook hard-fails without binary
- **Personas**: beginner, expert
- **Verified**: yes (integration.md line 36: `exit 1` on missing binary)
- **Actual**: Hook in integration.md exits 1 with "Run `make build` first" when binary is absent. Contradicts README's "POSIX path works without binary."
- **Draft severity**: P1

### F12 — `not TAG` quoting trap in list filters
- **Personas**: expert
- **Actual**: `erg list "not needs-human"` silently returns 0 results. Must be two separate args: `erg list not needs-human`.
- **Draft severity**: P2

### F13 — Unicode title silently drops accented characters from slug
- **Personas**: expert
- **Actual**: `erg new "Améliorer le système"` → slug `am-liorer-le-syst-me`. No warning.
- **Draft severity**: P2

### F14 — `erg update` passes through locale-dependent git errors
- **Personas**: beginner
- **Actual**: on French locale, error is in French (`fatal: ni ceci ni aucun...`). erg speaks English, git speaks the locale.
- **Draft severity**: P3

### F15 — `erg list`/`ready` show malformed files with blank titles
- **Personas**: beginner
- **Actual**: a malformed `.erg` file appears as a ready ticket with empty title. No warning.
- **Draft severity**: P2

### F16 — No CONTRIBUTING.md or contributor onboarding
- **Personas**: developer
- **Actual**: no guide for human contributors. AGENTS.md covers agent policy, not how a human submits a patch, names a branch, or adds a subcommand.
- **Draft severity**: P1

### F17 — Adding a subcommand requires reading 4 separate files
- **Personas**: developer
- **Actual**: helptext.go, main.go switch, tests/test_*.sh, Makefile TEST_SUITES — the pattern is discoverable but undocumented. TestDispatchRegistrySync is a good safety net, but the workflow is nowhere.
- **Draft severity**: P1

### F18 — Pre-commit hook uses tickets/erg but `make build` produces build/erg
- **Personas**: developer
- **Verified**: yes (integration.md line 27: `erg_bin="tickets/erg"`)
- **Actual**: hook error says "Run `make build` first" but make build won't fix it (wrong path). Contradicts AGENTS.md rule 7 (tests must use build/erg).
- **Draft severity**: P1

### F19 — tests/README.md references nonexistent fixture directories
- **Personas**: developer
- **Actual**: says "Use static fixtures under `tests/fixtures/valid/` and `tests/fixtures/invalid/`" — these don't exist. All tests use heredocs or mktemp.
- **Draft severity**: P2

### F20 — Go version not stated in README
- **Personas**: developer
- **Actual**: "Go needed for this step only" with no version. go.mod says 1.21.
- **Draft severity**: P2

### F21 — `make test` output buries the summary under 600+ lines of coverage
- **Personas**: developer
- **Actual**: "ALL TESTS PASSED" appears only at line 659. Must scroll past full coverage-by-function table.
- **Draft severity**: P2

### F22 — No fast development-loop target
- **Personas**: developer
- **Actual**: no `make check-fast` equivalent. `make test-<cmd>` exists but isn't in the Makefile usage header.
- **Draft severity**: P3

## False positives (not reproducible)

### FP01 — "README POSIX example omits `--- body ---`"
- **Personas**: expert (claimed)
- **Verified**: NOT reproducible. README lines 14–27 include `--- body ---`. The expert persona hallucinated this finding.

## Summary

| Draft severity | Count | Verified bugs | Friction items |
|----------------|-------|---------------|----------------|
| P0             | 0     | —             | —              |
| P1             | 9     | F01, F02, F03, F07, F11, F18 | F10, F16, F17 |
| P2             | 10    | —             | F04,F06,F08,F09,F12,F13,F15,F19,F20,F21 |
| P3             | 3     | —             | F05, F14, F22  |

**Next step**: human reviews severity ratings, confirms which items
become tickets. Each confirmed item should reference this report and
attach the relevant transcript section as evidence.
