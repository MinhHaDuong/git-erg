# Regressive log timestamp warning removed from erg check

The author asked whether the `erg check` warning on chronological log order
was noise and whether the spec says MUST or SHOULD. The spec's Log section
(src/go/assets/spec-erg-v1.md) says only "Append-only" and "Lines are never
edited or deleted": no RFC 2119 keyword, no ordering rule. The warning
(added by 0304, advisory, exit 0) fired on 44 files of this store. The author
asked to remove it in a full ticket, raid, roar cycle.

Raid aa12e2 worked ticket 0308 and landed PR #394 as 9ab13c9 on 2026-11-09.
`logShapeIssues` became `logHasDisplacedEntry` (one bool); the help text and
docs/erg-manual.md lost the regressive bullet; check_test.go inverted the
regressive case into a no-warning case. Post-merge `make check` on main
passed; `erg check` on this store reports 0 regressive and 264
displaced-entry warnings.

Observations:

- The Imagine seat searched this repository only. The Blind-Spot seat found
  a consumer in the harness: `tests/test_erg_log_placement.py:103-111`
  asserts the regressive WARN and reddens when the harness syncs the new
  binary. Filed there as harness ticket 1475, ticket-only PR
  MinhHaDuong/ImperialDragonHarness#2282, following the precedent of #2277.
- Committing that harness ticket ran the harness's own `erg check` hook,
  which printed 88 warnings, 87 of them regressive-timestamp.
- The commit message of d03176e said the harness test was "ticketed there"
  before the ticket existed; gaze and the gate both flagged it as a
  `consider`. The ticket was filed after the gate read the harness store.
- The orchestrator launched verify-gate in parallel with the step that
  minted the gate session UUID and passed `session=PENDING`; the gate minted
  its own session (586671cd-48cf-4586-bb53-fce39e3da4a2) and ruled round 1
  APPROVED on 0ac1910.
- The residual the change accepts: a hand edit inserting a line mid-log,
  forbidden by append-only, is no longer flagged by any check. erg itself
  stamps the current time and never produces that shape.
- `erg migrate` does not repair displaced entries, so the 264 remaining
  warnings have a manual remedy only.
