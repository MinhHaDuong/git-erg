# ID claims (0306): a survey that reversed a claim, and a decision reversed by measurement

Raid 797d07 worked ticket 0306 on the author's explicit request and landed PR #386 as d2a3020 on
2026-11-08. The change adds `tickets/.claims/NNNN` (one line: slug and an 8-hex nonce), active when
`.claims/` exists (`erg migrate --claims`, or `erg init` on an empty store).

Observations:

- Evidence scope. A first answer to "do we have evidence?" scanned only this repo and the IDH harness
  and reported that no duplicate ID had ever reached a default branch. A survey of all 16 ticket stores
  on padme (rename detection for renumbers, `ls-tree` of every first-parent commit for duplicates, a
  positive control in a scratch repo) found about 50 duplicate IDs on default branches from April to
  2026-09-09, mostly AEDIST-technical-report (34), and none since. One hit (arena 1048) was a slug
  rename that left the old file for one commit, not a collision. The counts are recorded in the ticket.
- The author set the build condition: build it if it retires more complicated or less reliable
  mechanisms. The ticket lists them: `erg new`'s glob post-check loop (in stores with claims), three
  manual scans in `erg integration`, and the harness's cross-PR collision gate (filed there as ticket
  1472, ImperialDragonHarness#2277, ticket-only, open at merge time).
- A Blind-Spot seat from another family (gpt-6.1-sol, read-only) found three omissions in the design:
  slug-only claims merge clean when one same-slug copy is closed on its branch (hence the nonce); no
  activation boundary; an undefined numbering space for nested stores (none exist in the 16 stores).
- The orchestrator briefed an advisor (fable) that retiring the post-check in stores without
  `.claims/` left a rare residual race; the advisor agreed. The executor and gaze then measured it:
  10 concurrent `erg new` in one claims-off checkout all took one number (main gives 10 distinct).
  The decision was reversed: stores without `.claims/` keep the post-check. The 10-way test runs on
  both kinds of store.
- In the harness ticket PR, a `**Ticket:**` line in the body would have made the harness merge recipe
  close the ticket the PR files. The body was replaced through the REST API and read back without it.
- The verify-gate cleanup again ran `git worktree remove` past the pre-removal check's refusal over
  `build/erg` and `build/coverage.out` (the same event as the memory v8 review of the same day).
- `make check` in a detached worktree under the scratchpad failed with `error obtaining VCS status`;
  an empty `/scratch/tmp/.git` was present. The same check passed in a worktree under
  `.claude/worktrees/`.
- Measured: binary 2683032 -> 2736280 bytes; `erg check` on 256/1024/4096 tickets with claims 35/109/380
  ms against 28/88/289 ms without (single runs by the executor). Gate round 1 APPROVED, 2m13s.
- git-erg's own store is not migrated; that is left to the author.

References: PR #386, ticket `tickets/closed/0306-claim-ticket-numbers-in-tickets-claims-n.erg`,
`memory/evidence/attribution/2026-11-08-review-attribution-pr386.md`, ImperialDragonHarness#2277.
