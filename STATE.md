# State — git-erg

_Last updated: 2026-09-17T13:50Z — `erg update` became `erg sync` (0298, #368) and the rename drew five follow-up PRs in one day (#369 to #373, tickets 0299 to 0303): every non-origin source now fetches in a throwaway bare repository, a repo-local `insteadOf` is honoured, environmental failures exit 0, and a source string starting with `-` is refused before git sees it — the decorrelated review found that a committed `tickets/.ergrc [update] url` of `--upload-pack=<cmd>` executed the command on `erg sync`, exit 0. Surveyed the same day: zero forks, sixteen local adopters, none with an `[update]` url, so the channel was never exposed. The September raid is closed; `make check` runs end to end again (0287); an edit to the shipped `tickets/AGENTS.md` is a hard error (0289). Open: the dyne queue only._

## North star:

An agent-friendly local ticket system for development in disconnected environments.

## Milestones

- Audits (incl. fang-audit 2026-06-05: FANG-AUDIT.md, all 3 gaps closed), dogfood
  migration (0216), guard sweep, scope confinement (0237–0239), robustness raid
  (0248–0253), dyne design (0270), asset-machinery hardening (0278 and children
  0279/0280/0281/0283, plus 0282/0285/0287/0288/0289/0292/0296/0297) complete.
- Queue: dyne phase 1 (0271–0275, 0271 ready) — untouched by the September raid
  and now the only open work.
- Bar for new work: "verified empirical need" (AGENTS.md). Reaffirmed
  2026-09-16: the raid opened six tickets to close three at mid-point; two were
  closed wontfix on audit against the severity floor and three were dropped in
  favour of one-line fixes.

## Notes

- **erg validate vs erg check**: validate is per-file; check is corpus-level.
- **CI**: bootstrap binary rebuilt automatically on every push to main changing `src/go/`.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Test fixture IDs ≥9000 reserved for unclaimed-ticket tests.
- **Prove a guard bites before trusting its silence.** The 2026-09-16 raid found
  thirteen controls whose "all clear" could not be told from "I could not look"
  (arms vacuous without `git` on `PATH` or under an ambient `GOFLAGS`, a docs
  gate whose two sides shared one omission, `test_assetinvariant.sh` passing a
  `downgrade := false` mutant, checks skipped by an empty-corpus early return
  behind a fixture helper). Revert the hunk and watch the specific test redden,
  per case, not per suite; read the fixture helper before the assertions.
- **`erg sync` needs git 2.24 or newer** (`--end-of-options` on the fetch,
  since #373). Below it the fetch fails and sync is an exit-0 no-op; README
  states the floor, nothing detects it at run time. Every known host runs 2.43
  or later, so this is recorded, not owed.
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

## Status
<!-- generated 2026-09-17T13:05Z · as of e8f2e5b -->

**Tickets:** 1 ready · 4 blocked — `erg ready tickets/` for full list
  next: 0271 Write pep-dyne-v1.md, the dyne design rationale
**In flight:** no open PRs · CI main: success
**Recent (first-parent):**
  e8f2e5b chore: rebuild bootstrap binary [skip ci]
  cc69cbb Merge pull request #373 from MinhHaDuong/finition-372
  fc7a9fa chore: rebuild bootstrap binary [skip ci]
