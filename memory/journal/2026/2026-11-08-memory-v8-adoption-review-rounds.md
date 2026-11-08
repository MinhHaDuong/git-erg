# Memory v8 adoption: seven gaze rounds on one inventory file

Raid 8b9a41 worked ticket 0305 (dispatched by the harness on 2026-10-05) and
landed PR #384 as 64c32a2 on 2026-11-08. The change added a `## Project memory`
section to AGENTS.md, `memory/MEMORY.md` (25 lines) and `memory/SOURCES.md`.

Observations:

- Imagine mapped two exit criteria onto the convention as it changed after
  dispatch: `memory/DREAM.md` dropped (harness e309df17) and a 40-line index
  cap (harness af95a78f). The change was logged on the ticket before the gate,
  and the gate ruled both criteria ADDRESSED on that log.
- Gaze ran seven rounds on PR #384: four FINDINGS, three REVIEWED. Each
  FINDINGS round named a `memory/SOURCES.md` claim that evidence already on
  the host contradicted; two were "unknown (not probed)" hedges. The only
  other blocking finding was a missing blank line in AGENTS.md. Three of those claims came from the orchestrator's own
  corrections, one of them copying the previous round's suggested text.
- The PR's load probe (Claude Code 2.1.319) started in a worktree. Its
  transcript was filed under a worktree-keyed project directory with no
  `memory/`. Its `instructions` attachment carried an `AutoMem` entry for the
  repository root's native `MEMORY.md`. That is one session; the rule
  Claude Code applies is not established.
- 11 Codex rollouts have a cwd under this checkout, all from 2026-09-17.
- During the gate's cleanup, the pre-removal check refused the gate worktree
  because of two ignored files, `build/erg` and `build/coverage.out`, that its
  own `make test` had produced. The removal ran anyway. Nothing else was in
  that tree.
- The executor read "do not write attribution lines" as covering commit
  trailers. It amended its commit and force-pushed with lease, so the two
  adoption commits carry no `Co-Authored-By` trailer.

References: PR #384, review rv-20261108-pr-384-2f8f on notes, ticket
`tickets/closed/0305-d-ployer-la-m-moire-v8.erg`, harness audit
`conception/2026-10-29-memory-system-audit.md` § Correction.
