# Claims rollout across the adopter repos; the binary-commit hook rule dropped

2026-11-09, git-erg session (Claude Opus 5.5), after PRs #397 (0309), #398 (0310) and #399 (0311).

## Context
The author asked for a rollout plan for required ID claims (0307) across every erg store on padme and
doudou, noting that some repos had live sessions, some had none, some had no upstream remote.

## Events
- Inventory: 14 non-scratch stores on padme and 20 on doudou, about 16 distinct GitHub repos plus no-remote
  repos (arena on padme; askMinh, fs-housekeeper, matrice-hybride, html on doudou; html pushes to a bare repo
  on padme). Scratch copies and arena/workloads (frozen benchmark snapshots) were excluded.
- Dry run of `erg migrate --claims` with the new binary on scratch copies of every store: 25 stores, zero
  blockers (no shared numbers, no ignored claim paths). sen-causal-reading's `.gitattributes` marks
  `tickets/` export-ignore, so `git archive` produced an empty store; a `git clone --shared` copy worked.
- The new binary prints hundreds of advisory WARNs per large store (the displaced-log-entry lint of
  check.go, deliberate; git-erg itself shows 264); the vendored older binaries showed none.
- padme clones of AEDIST-technical-report and chemin-de-voix have `core.hooksPath` set to directories that
  no longer exist; padme's `~/.local/bin/erg` is rev 2c490b7, without claims. Filed by the padme session
  as padme ticket 0110.
- Author decisions: the harness session drives the rollout; live sessions file their own tickets;
  the binary is pinned by sha256 rather than a signed release ("we do want to go to release, but not so
  fresh").
- The recipe's `tickets/erg sync --upstream` failed in padme and JETP-observer: 10 of 13 padme stores
  vendor binaries older than `sync` (revs 63b5e92, 616b023, 5158508, 8331a94). Their `update` verb with
  `ERG_UPDATE_URL=https://github.com/MinhHaDuong/git-erg.git` imported the pinned binary (tested on a
  scratch padme clone). README and CHANGELOG now name it (#399).
- JETP-observer reported that the managed pre-commit block refused committing `tickets/erg` on a
  non-default branch ("CI rebuilds the binary after merge"), a rule true in git-erg only; the same rule had
  forced `--no-verify` in chemin-de-voix in June (0219). With the author's go-ahead it was dropped (0311,
  #399). JETP also reported that with an absolute `core.hooksPath` into the primary checkout's tracked
  `.githooks/`, `install --hooks` run from a worktree writes into the primary checkout; judged git
  semantics, no guard added.
- Merging #399 moved git-erg main's binary; the author chose to merge and move the pin from
  dafb1d48 (rev 8d24e74) to 6a104262 (rev 2d0dbea, committed ab519a3).

## Outcome at the time of writing
- Migrated: sen-causal-reading (#165, 111 claims), padme (#244, 85 claims; hooks tested).
- JETP-observer #285 in its gate on dafb1d48; climate-finance-het approved by the author, waiting on a
  referee run; the harness (ticket 1472) and the remaining repos are driven by the harness session.
- The doudou clones still need a pull and `install --hooks`; the doudou codex session in
  sen-causal-reading was not reachable from here.
