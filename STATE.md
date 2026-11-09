# State — git-erg

_Last updated: 2026-11-09T12:50Z — ID claims are required (0307, #393): `erg check` fails a store without `tickets/.claims/`, `erg new` refuses there, and `erg migrate --claims` is the single repair. This store is migrated. The rollout to the adopter repos on padme and doudou is driven by the harness session (its tracker; 1472 for the harness itself), pinned to binary sha256 513cd3c4 (revision bf71ab5); sen-causal-reading and padme are migrated. A signed release follows once that binary has run a while._

## North star:

An agent-friendly local ticket system for development in disconnected environments.

## Milestones

- Audits (incl. fang-audit 2026-06-05), dogfood migration (0216), guard sweep,
  scope confinement (0237–0239), robustness raid (0248–0253), asset-machinery
  hardening (0278–0297), `erg sync` (0298–0303) complete.
- Dyne phase 1 (0271–0275) closed unbuilt in favour of Git notes review
  (#377, #380, #381). Ticket-number claims (0306, #386) shipped, required in
  every store since 0307 (#393); the managed pre-commit block no longer
  refuses `tickets/erg` on a branch (0311, #399).
- Bar for new work: "verified empirical need" (AGENTS.md).

## Notes

- **erg validate vs erg check**: validate is per-file; check is corpus-level. CI rebuilds the bootstrap binary on every push to main changing `src/go/`.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Test fixture IDs ≥9000 reserved for unclaimed-ticket tests.
- **Prove a guard bites before trusting its silence**: revert the hunk, watch the specific test redden.
- **`erg sync` needs git 2.24 or newer** (`--end-of-options`); below it sync is
  an exit-0 no-op. Every known host runs 2.43 or later.
- **ID allocation**: `erg new` returns max+1 over the store, sibling worktrees
  and every branch tip (claims included), never a gap. Claims detect a
  collision at merge or rebase as an add/add conflict.
- **Pre-sync binaries**: adopters vendoring a binary older than `erg sync`
  import with `ERG_UPDATE_URL=https://github.com/MinhHaDuong/git-erg.git
  tickets/erg update`.

## Deferred ideas

Premature, unproven, or waiting on evidence. Do not promote without AGENTS.md bar met.

- feat: O(1) everything, tickets store cache.
- feat: Flag blocking tags as a first-class concept.
- feat: erg new with body on the line.
- audit: usage in idh.
- feat: AI script to realign docs and code (partial: 0232)

Live counts: `erg ready tickets/` · `erg list` · `/dashboard`.
