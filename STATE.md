# State — git-erg

_Last updated: 2026-09-16T13:10Z — Raid over the six open non-dyne tickets (0277, 0278, 0282, 0283, 0285, 0287) landed five of them plus one child. `make check` runs end to end again (0287): a stray empty `.git` above `$TMPDIR` blinded the offline negative control, and eleven suites after `test-contract` had not executed for an unknown period. The asset-machinery family closed out: 0283 reports a stampless store, 0282 gives the vendored `erg-github` a staleness channel, 0278's whole-class invariant is mechanized as `tests/test_assetinvariant.sh`, and 0285 bounds the closure path test to the store root. 0277 became a tracker: 0288 (lore out of the resident asset into `erg integration`) merged; 0289 (the ban) is unblocked now that harness 0906 restored its copy, and carries the author's read-only proposal. Open from this line: 0289, 0292 (PR #360, in review), 0296._

## North star:

An agent-friendly local ticket system for development in disconnected environments.

## Milestones

- Audits (incl. fang-audit 2026-06-05: FANG-AUDIT.md, all 3 gaps closed), dogfood
  migration (0216), guard sweep, scope confinement (0237–0239), robustness raid
  (0248–0253), dyne design (0270), asset-machinery hardening (0278 and children
  0279/0280/0281/0283, plus 0282/0285/0287) complete.
- Queue: dyne phase 1 (0271–0275, 0271 ready) — untouched by the September raid.
  Asset-machinery remainder: 0292 (PR #360), 0296, then 0289 closes 0277.
- Bar for new work: "verified empirical need" (AGENTS.md). Reaffirmed
  2026-09-16: the raid opened six tickets to close three at mid-point; two were
  closed wontfix on audit against the severity floor and three were dropped in
  favour of one-line fixes.

## Notes

- **erg validate vs erg check**: validate is per-file; check is corpus-level.
- **CI**: bootstrap binary rebuilt automatically on every push to main changing `src/go/`.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Test fixture IDs ≥9000 reserved for unclaimed-ticket tests.
- **Prove a guard bites before trusting its silence.** The 2026-09-16 raid found
  eight controls whose "all clear" could not be told from "I could not look":
  an arm vacuous when `git` was absent from `PATH`, the same arm defeated by an
  ambient `GOFLAGS` and again by a persistent `go env -w`, a byte-ceiling
  control testing operator polarity, a literal-string scan a paraphrase evades,
  a docs-drift gate whose two sides shared one omission, a test that never
  exercised the command it claimed to guard, and `test_assetinvariant.sh`
  itself, which passed a `downgrade := false` mutant and was inert on a binary
  built without `-X main.buildDate`. The fixes were mostly right first time; the
  guards around them were not. Revert the hunk and watch the specific test
  redden — per case, not per suite.
- **`erg new` is not blind across branches.** It returns max+1 over three
  sources: the store, sibling worktrees at the same relative path, and every
  `refs/heads` and `refs/remotes` tip. Shipped documentation said otherwise
  until 2026-09-16. What remains: an unfetched ref, a 200 ms scan deadline that
  warns on stderr, and a scan/commit race. Because it is max+1 and never
  first-gap, a skipped ID is never reissued.

## Deferred ideas

Premature, unproven, or waiting on evidence. Do not promote without AGENTS.md bar met.

- feat: O(1) everything, tickets store cache.
- feat: Pre-create 00XX-is-next-ticket.erg.
- feat: Flag blocking tags as a first-class concept.
- feat: erg new with body on the line.
- audit: usage in idh.
- feat: AI script to realign docs and code (partial: 0232)
- test: opt-in `make mutate-assetinvariant` to mechanize the mutation controls
  `test_assetinvariant.sh` currently records in prose.

## Status
<!-- generated 2026-09-16T13:10Z -->

**Recent commits:**
  871804f Merge pull request #359 from MinhHaDuong/note-0289
  4a842a6 Merge pull request #358 from MinhHaDuong/t0296-rollback-arm
  9c52b1d test(0297): correct a carried-forward count, and close the ticket
  5d8b2d6 test(0297): a fourth arm, because the suite passed a mutant it should have killed
  b9d0199 ticket(0289): accept the read-only proposal, as friction and not as a lock
