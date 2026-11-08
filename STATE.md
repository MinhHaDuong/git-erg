# State — git-erg

_Last updated: 2026-11-08T22:40Z — ticket-number claims shipped (0306, #386): `tickets/.claims/NNNN` (slug and nonce) makes two branches that take one number conflict in git, offline, on any forge; on in a store once `erg migrate --claims` creates `.claims/`. Built on a 16-store survey (about 50 duplicates reached default branches until 2026-09-09) and on retiring less reliable checks; the harness's cross-PR gate retires by its ticket 1472. No store is migrated yet, this one included: adoption per repo is the author's call._

## North star:

An agent-friendly local ticket system for development in disconnected environments.

## Milestones

- Audits (incl. fang-audit 2026-06-05), dogfood migration (0216), guard sweep,
  scope confinement (0237–0239), robustness raid (0248–0253), asset-machinery
  hardening (0278–0297), `erg sync` (0298–0303) complete.
- Dyne phase 1 (0271–0275) closed unbuilt in favour of Git notes review
  (#377, #380, #381). Ticket-number claims (0306, #386) shipped.
- Bar for new work: "verified empirical need" (AGENTS.md).

## Notes

- **erg validate vs erg check**: validate is per-file; check is corpus-level. CI rebuilds the bootstrap binary on every push to main changing `src/go/`.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Test fixture IDs ≥9000 reserved for unclaimed-ticket tests.
- **Prove a guard bites before trusting its silence**: revert the hunk, watch the specific test redden.
- **`erg sync` needs git 2.24 or newer** (`--end-of-options`); below it sync is
  an exit-0 no-op. Every known host runs 2.43 or later.
- **ID allocation**: `erg new` returns max+1 over the store, sibling worktrees
  and every branch tip (claims included), never a gap. Claims detect a
  collision at merge; a store without `.claims/` keeps the glob post-check.

## Deferred ideas

Premature, unproven, or waiting on evidence. Do not promote without AGENTS.md bar met.

- feat: O(1) everything, tickets store cache.
- feat: Pre-create 00XX-is-next-ticket.erg.
- feat: Flag blocking tags as a first-class concept.
- feat: erg new with body on the line.
- audit: usage in idh.
- feat: AI script to realign docs and code (partial: 0232)

Live counts: `erg ready tickets/` · `erg list` · `/dashboard`.
