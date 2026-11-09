# Pre-commit WARN flood: adopter ticket ingested and fixed the same day

The JETP-observer session filed ticket 0313 in this repository as a
ticket-only PR (#404): the managed pre-commit block ran `tickets/erg check
tickets/` uncaptured, printing every corpus WARN (236 there, about 40 KB) on
each ticket commit. The author asked to ingest it, then `/raid 313`.

PR #404 landed as a ticket-only change after gaze (REVIEWED, one consider:
installed hooks keep the old text until `erg install --hooks` is rerun, added
to the ticket's Notes) and verify-gate (APPROVED; one consider: the hand-typed
log timestamp 13:30Z was 39 minutes after its commit, corrected to 12:51Z
before merge).

Raid f314e3 then worked 0313 and landed PR #405 at 9284b14 (merge 4f2326e).
`hookBody` now captures check's combined output: a failing check prints it
all, a passing one prints only the `ERG CHECK` line. No `check` flag was added,
because `install --hooks` never refreshes the vendored binary and `cmdCheck`
exits 1 on an unknown flag. Post-merge `make check` on main passed.

Observations:

- The commit that added the upgrade note to 0313 printed 267 lines from this
  repository's own hook: 264 WARNs of the displaced-log-blank class.
- The Blind-Spot seat found two things the Imagine seat missed: count-only
  also hides the asset-drift WARN/NOTE lines that integration.md presented as
  the upgrade nudge (kept count-only per the exit criterion's letter;
  integration.md reworded; the alternative left to the author), and the
  planned assertion `^WARN ` could not fail, as warnings print indented two
  spaces in four spellings (`WARN x:`, `WARN:`, `WARNING:`, `NOTE`).
- The Plan seat corrected the Imagine seat: a byte-level drift guard between
  `hookBody` and integration.md exists (tests/test_selfcoherence.sh (iv)).
- `erg close 313` failed with "no ticket found"; `erg close 0313` worked.
  `resolveTicketByID` (src/go/main.go:180) globs `<id>-*.erg` with no padding,
  while `erg ready` prints the padded form. Not ticketed (below the floor).
- Gaze's first answer on #405 and the verify-gate exit on #404 were both
  refused cleanup by `build/` left from `make test`/`make validate` in the
  review worktree; each removed it and re-ran.
- Gaze's REVIEWED answer on #405 ran a script seat only (`agents=0`, 15 s).
  Verify-gate reproduced its red and mutation controls itself before
  approving. The review attribution capture for #405 failed: `not derivable:
  reviewers`; no record was written under memory/evidence/attribution/.
- The #404 attribution record names this session as writer: the tool's
  default when no execute writer is on notes. The original commit 118933d
  was the JETP-observer session's, model not recorded.
- This repository's installed pre-commit keeps the noisy block until
  `tickets/erg install . --hooks` is rerun.

References: PR #404, PR #405, tickets/closed/0313-*.erg, raid log f314e3.
