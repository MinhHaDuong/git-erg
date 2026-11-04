# Dyne and review evidence — design reassessment

Date: 2026-11-04

Status: historical reassessment. Later on 2026-11-04 the author closed
tickets 0271–0275 without implementing Dyne and requested a technical note
for HAL and a practical post instead (harness ticket 1445). The proposed
next decision and open-ticket status below record the earlier position.

This reassesses the 2026-10-15 proposal for tickets 0271–0275 against the
review workflow now used by the harness. It is a design assessment, not a
format specification or evidence that Dyne is implemented.

## Decision and current execution

The author confirmed on 2026-11-04 that Git notes are the home for review
evidence. The earlier proposal to commit reports under `tickets/reviews/`
is superseded. Do not create a parallel report store or require report-only
commits. Mechanical merge-gate records remain separate from review notes.

The harness now records review requests, answers and gate verdicts through
`review_notes.py` in `refs/notes/review`. A review is anchored independently
of a branch commit; entries identify the reviewed head, and retained
`refs/review-heads/` preserve examined commits. Gaze records its answer,
verify-gate records its verdict, and landing publishes the evidence. The
workflow supports a branch without a forge and also associates reviews with
PRs. See the harness's `skills/gaze/SKILL.md`, `skills/verify-gate/SKILL.md`,
`skills/merge/SKILL.md` and `conception/2026-10-27-notes-review-spec.md`;
the skills describe current operation where the earlier design spec has
since been amended.

This is evidence of a working harness workflow, not evidence that Git notes
alone supplies a general-purpose merge-request interface, that every planned
resume or concurrency scenario works, or that every project has the harness.
Git notes and retained heads require explicit transport; a branch push alone
is not evidence that review records reached another clone.

## Rejudge the original ideas

- **Independent, offline review:** retained as a goal. The harness already
  provides a no-forge path; determine what a user without the harness actually
  lacks before proposing a separate tool.
- **Evolving review and independent judgment:** retained. A plan should state
  its scope; prior findings and the reviewed revision must remain traceable.
  Gaze reviews, verify-gate judges criteria, and the caller decides to merge.
  Reuse must follow the current head and evidence rules, not a blanket
  exemption for files in a directory.
- **Immutable request and response history:** retain the intent, using the
  notes workflow rather than committed per-review files. Distinguish failed
  or missing execution from a finding. Do not claim that every proposed
  per-seat resume mechanism is already implemented.
- **A Dyne file as a second merge-proposal object:** unproven. Branch review
  requests and notes already cover some of the proposed function. A tracked
  `tickets/merges/<branch>.dyne`, its discussion and its plan would introduce
  another authority unless a concrete remaining need and ownership boundary
  justify it. The fixed path, grammar, union merge, standalone binary, hooks,
  and generated forge body are design candidates, not authorization to build.
- **Archive by merge and DAG-derived status:** still a useful candidate for
  branch state, but ancestry alone does not describe a rejected or abandoned
  proposal. Notes have their own retention and transport; merging a branch does
  not by itself publish or archive them.
- **Cross-host and disposable-container recovery:** still needs evidence for
  the particular workflow. Notes transport and retained heads address part of
  the problem; publication, interruptions and concurrent writers must be
  tested before claiming the whole scenario is solved.

## Next decision

Ticket 0271 must first compare the current no-forge workflow with a concrete
user task and identify a demonstrated gap. Recommend the smallest remedy,
including no Dyne, and explain authority, data safety, transport, concurrency
and costs. Only if a separate proposal object earns its place should 0271
settle its design and 0272 specify it; 0273–0275 remain conditional on that
decision. The historical ticket scopes preserve the original candidates, not
current implementation instructions. No ticket is closed by this assessment.
