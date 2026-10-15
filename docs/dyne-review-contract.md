# Dyne review contract — design amendment

Date: 2026-10-15

Scope: adopted design amendment for tickets 0271-0275 and IDH 1100; not the complete %dyne file specification or a claim of implemented behavior.

## Author decisions — Dyne and evolving reviews (2026-10-15)

Recorded from the author's discussion; documentation only, not implementation.
This section supersedes earlier conflicting storage/transport choices.

### Layout and independence

- Fixed project-relative locations: `tickets/merges/<branch>.dyne`
  and `tickets/reviews/<base-short>..<head-short>/`. No new path
  configuration is introduced. Full reviewed Git object IDs remain in reports;
  abbreviations are extended for uniqueness.
- The shared `tickets/` parent is a layout convention, not a dependency:
  Dyne and review execution must work without erg installed, without .erg
  files, and without a forge. Ticket references are optional and opaque.
- Retain the planned Go implementation, common internal machinery and
  independently invocable erg/dyne commands; no third review binary.
- Dyne represents the merge proposal. Its body carries the evolving review
  plan and references to evidence; it is not merely a review report.
- Merging the proposal archives its Dyne record through Git history/DAG.
  The author also requested moving completed review bundles out of the active
  reviews area. Exact archive naming (merged versus closed), abandonment and
  relocation of references remain to settle; do not encode Dyne status by path.

### Plan ownership and execution

- Hunt owns and revises the review plan; gaze executes it, collects reports
  and returns a synthesis. verify-gate remains separate; merge is a caller
  decision. Gaze does not grant final APPROVED.
- Review -> fix -> review is an evolving plan, not an automatic fresh battery.
  Preserve previous passes and explain reuse, invalidation, unresolved findings
  and the scope of the next pass.
- Hunt passes the Dyne file to gaze. Each execution must identify the exact
  plan revision and full base/head IDs it consumed; the serialization and
  interrupted-run/resume contract remain open.
- Each per-review file has two logical parts: the frozen request (range,
  scope, instructions), then the returned report and execution metadata.
  Gaze/controller creates the request before dispatch and is the sole writer
  of the final artifact. Reviewers read the request and return text via stdout,
  inline results, JSON or the available transport; the controller validates
  and appends the result without rewriting the request.
- Preserve reviewer/model identity, execution ID, duration and turn count
  when available; mark unavailable measurements explicitly. Execution failure
  or interruption is distinct from the review findings/verdict.
- Keep one distinct artifact per execution so later passes do not overwrite
  earlier reports; the precise collision-free naming remains to specify.
- Track and commit reports. Report-only commits do not alone invalidate a
  code review, but changes to relevant code/context do. Do not exempt executable
  or policy changes merely because they are placed under a review directory.
  Final checks and gates remain tied to the delivered revision.
- Cover simultaneous work on Padme and doudou and disposable development
  containers. Committed work must reach durable storage before container loss.
  Git transport preserves evidence; it does not provide work reservation.

### Open questions — recorded, not newly approved policy

1. Evidence reuse: distinguish tests from reviewer judgments; define scope,
   command/environment/dependency identity, criteria/policy identity and
   invalidation rules. Preserve the existing conservative contract until
   explicitly revised; avoid redundant runs without claiming all checks reusable.
2. Stop rule: define sufficient evidence, time/pass budget and escalation.
   Exhaustion must not silently become approval; no numeric budget was chosen.
3. Concurrency: prevent duplicate dispatch and conflicting plan updates;
   choose reservation/ownership and recovery without smuggling doing/claimed
   state into erg tickets.
4. Integration: account for a moving target and validate the resulting merge
   with proportionate checks.
5. Specify plan snapshots, report collection failure, resumption, archive
   references and the still-unrecalled third deployment scenario.

Acceptance work: demonstrate a Git-only/no-erg run, immutable requests and
retained passes, report-only commit handling, relevant-change invalidation,
two-host concurrent execution and interrupted disposable-container recovery.
These are implementation criteria to refine, not tests claimed to have run.

### Delivery phases — author request, 2026-10-15

This is multi-day work. No reliable duration estimate was established.
Documentation commits do not authorize claiming implementation complete.

1. **Record and reconcile design (now).** Update 1100 and Dyne 0271-0275;
   capture the layout and request/response contract, and list unresolved
   questions. Discuss blind spots with the author after this capture.
2. **Settle the minimal contract.** Resolve evidence reuse, stopping,
   concurrent ownership, plan snapshots/resume and integration checks.
   Produce the Dyne rationale/format (0271 then 0272) before implementation.
3. **Build the offline Dyne core.** 0273 then 0274: Go CLI, atomic file
   operations, validation, embedded docs and fixed layout. Prove operation
   without erg or a forge. No review orchestration in the format binary.
4. **Connect Hunt and gaze.** Map 1100's evidence-core and gaze children to
   the Dyne plan, controller-written request/response files and execution
   metadata. Verify immutable passes and proportionate re-review.
5. **Exercise deployment and integration.** Two hosts, disposable container,
   interrupted collection, moving target and report-only commits; implement
   the outbound forge rendering in 0275. Measure timings before promising gains.
6. **Roll out callers and retire obsolete paths.** Preserve 1100's child-0
   prerequisite before child-3 retirement; measured raid, conditional prose
   fold only if justified, then bounded-time follow-up 1099.

Phases are delivery slices, not newly created child tickets. File bounded
implementation children before coding and retain existing dependencies.
