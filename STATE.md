# State — git-erg

_Last updated: 2026-06-08T12:45Z — Raid closed 0241 (close-without-archive escape, #298): erg check now treats folder/header closure as an error (caught by pre-commit hook + CI), and `erg install --hooks` installs the pre-push hook too; end-to-end regression test in test_hook.sh. Queue empty again. Prior (2026-06-05): fang-audit on the Go suite (#293, fangs in #294/0240); scope-confinement trilogy 0237–0239._

## North star:

An agent-friendly local ticket system for development in disconnected environments.

## Milestones

- Audits (incl. fang-audit 2026-06-05: FANG-AUDIT.md, all 3 gaps closed), dogfood
  migration (0216), guard sweep, scope confinement (0237–0239) complete.
  Queue: empty. Bar for new work: "verified empirical need" (AGENTS.md).

## Notes

- **erg validate vs erg check**: validate is per-file; check is corpus-level.
- **CI**: bootstrap binary rebuilt automatically on every push to main changing `src/go/`.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Test fixture IDs ≥9000 reserved for unclaimed-ticket tests.

## Deferred ideas

Premature, unproven, or waiting on evidence. Do not promote without AGENTS.md bar met.

- feat: O(1) everything, tickets store cache.
- feat: Pre-create 00XX-is-next-ticket.erg.
- feat: Flag blocking tags as a first-class concept.
- feat: erg new with body on the line.
- audit: usage in idh.
- feat: AI script to realign docs and code (partial: 0232)

## Status
<!-- generated 2026-06-08T12:45Z -->

**Recent commits:**
  ac76d08 chore: rebuild bootstrap binary [skip ci]
  a3bbc92 Merge pull request #298 from MinhHaDuong/t0241
  223972a test(0241): end-to-end regression test for the escape path
  0cf2b06 ticket(0241): close and archive — PR forthcoming
  3c11a37 feat(0241): close the close-without-archive escape
