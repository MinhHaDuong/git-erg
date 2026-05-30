# Security Red-Team Process

Repeatable AI-assisted red-team QA for git-erg's security posture. Agents
run the procedure; humans gate findings and set severity. Scope, threat
model, and the item-by-item checklist live in dedicated docs (linked below)
-- this file is the orchestration layer: when to run, how to prompt the
agent, and how findings become tickets.

## When to run

- Before tagging a release.
- After any change to a high-risk surface: the parser (`erg.go`), path/ID
  resolution (`ref.go`, `new.go`, `rm.go`, `archive.go`), the update path
  (`update.go`, `main.go`), or the build pipeline (`Makefile`, `go.mod`).

## Scope and threat model

[`docs/threat-model.md`](docs/threat-model.md) -- assets, threat actors,
attack surfaces (S1--S5), severity multiplier, and the controls table.
Read it before running; do not restate it in the run.

## Procedure

The agent runs the items in
[`docs/red-team-checklist.md`](docs/red-team-checklist.md) (S1--S5) and
records results in the run-log section of that file. The checklist is
designed to be cheap: most items delegate to the existing CI test suites
(`tests/test_security.sh`, `tests/test_update.sh`, `make verify`).

Cold prompt for the red-team agent:

> You are a security auditor red-teaming a small Go CLI tool (`erg`) whose
> binary is committed to every repo that uses it. The threat model is at
> `docs/threat-model.md`; the checklist is at `docs/red-team-checklist.md`.
>
> Task: run every item on the checklist against the binary built by
> `make build` (at `build/erg`). For each item, record PASS or FAIL with
> one-line evidence. If you find a new attack vector not covered by the
> checklist, document it as an appendix item. Append a dated run-log
> section to `red-team-checklist.md` (see the existing run-log for format).
>
> Do not treat a passing checklist as proof of safety -- your value is in
> the re-run and in surfacing what the checklist misses.

## Output format

The run-log in `docs/red-team-checklist.md` is the output. Each run
appends a section with:

- Date, binary revision, and platform.
- A table: item | result (PASS/FAIL) | evidence (one line).
- A findings paragraph: new vectors, regressions, or items that need human
  judgment.
- A next-run note.

## After the run

1. A human reviews the run-log findings and assigns severity (High /
   Medium / Low).
2. High-severity findings become their own fix tickets immediately:
   `erg new "<concise title>"` with the relevant evidence pasted into the
   body.
3. Medium findings get tickets unless the fix is trivial enough to land
   in the same PR as the run-log update.
4. Log the run on the parent tracker (ticket 0151):
   `erg log 0151 "Claude red-team run -- N items, M findings: <severities and ticket IDs>"`
5. Threat-model changes (new surfaces, revised severity, control
   additions) are human-gated -- the agent proposes edits to
   `docs/threat-model.md` but does not merge them unattended.

## What CI checks independently

These run on every push and do not need to be re-run manually:

- **Path/ID injection + input DoS** (26 checks): `tests/test_security.sh`
  (ticket 0157).
- **Update-channel integrity** (14 checks): `tests/test_update.sh`
  (ticket 0148).
- **Reproducible build**: `make verify` rebuilds from the embedded
  revision and byte-diffs (ticket 0156).
- **No network code**: `tests/test_update.sh` asserts no `net/http` /
  `crypto/tls` in source.

The red-team covers judgment and novel vectors; CI covers regressions.
Both are needed.
