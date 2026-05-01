# State — git-erg

_Last updated: 2026-05-01 — post tickets-pipeline coherence pass_

## Stats

- Tickets: 14 total — 7 closed, 7 open (5 ready, 2 blocked)
- Tests: 31 passing (validate 18, ready 9, archive 4) — recount when 0014 lands
- Open PRs: #10 — `tickets: 0013 rewrite + close 0006 + file 0014`

## Ready to work

| #    | Title                                                           | Notes |
|------|-----------------------------------------------------------------|-------|
| 0007 | Install erg binary on PATH                                      | Independent — land anytime |
| 0012 | Drop Status header — derive closed-ness from path or marker     | Proposal; design questions still open |
| 0013 | Drop .wip files and claim/release                               | Proposal in #10; implementation is the next work item |
| 0014 | Modularize Go source + godoc                                    | Prereq for 0008 |

0001 is the open half of the validator fixture pair (0001 + 0002) — leave open
indefinitely; it has no actionable work.

## Blocked

| #    | Title                       | Blocked by |
|------|-----------------------------|------------|
| 0002 | Sample blocked              | 0001 (intentional fixture) |
| 0008 | Add erg pick command        | 0014 |

## Sequencing

1. **Implement 0013** — 12 changesites in code, spec, design doc, skills, plugin, install/README. Mechanical. Atomic switch (no transition window). Companion IDH ticket already drafted; gets filed in IDH's tracker.
2. **Resolve 0012's open design questions** — `pending` fate, path convention vs. `Closed:` marker, migration shape. Then either file an implementation ticket or reject with rationale.
3. **0007** runs in parallel with the above — no dependency.
4. **0014** (modularize) unblocks 0008.
5. **0008** (`erg pick`) lands on top of 0014.

## Notes

- **Status spec gap to address before 0008 implementation:** `needs-human` is referenced as a Status value in 0008, but `rules/tickets.md` only defines `open|doing|closed|pending`. Decide whether `needs-human` is a tag (new mechanism in v1), a Status enum extension, or something the body carries informally. Pre-existing — not introduced by current work.
- **Downstream coordination.** IDH (and other consumers — climate-finance, AEDIST) read `erg ready --json` and use the plugin's skill files. 0013 changes both. Coordination ticket drafted for IDH; sequencing is "land git-erg first, IDH follows."
