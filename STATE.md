# State — git-erg

_Last updated: 2026-05-12T10:40Z — Merged PR #135 (.ergrc tag config) and PR #136 (resolveDir/resolveTicketByID helpers). 4 ready, 1 claimed, 1 blocked._

## Stats

- Tickets: 127 closed, 6 open
- Tests: green — ok git-erg (34.2% coverage)
- Open PRs: none

## Ready to work

- **0127** — Audit spec/PEP/code alignment for post-0096 drift.
- **0128** — Define and enforce text file encoding stance (BOM, EOL, UTF-8).
- **0129** — Reconsider Ref as URL with shortcut semantics.
- **0133** — Downgrade format version from v1 to 0.1 with MAJOR.MINOR scheme.

## Claimed

- **0130** — Add erg tag/untag CLI commands. Branch exists.

## Blocked

- **0131** — Blocked by 0130.

## Notes

- **0116 chain plan**: 0116 → 0117 → 0118. Each delivers value standalone. Endpoint: schema-pure `Erg` with `Created time.Time`, `BlockedBys []Ref`, zero accessor methods except `IsClosed()`. All three closed.
- **erg validate vs erg check**: validate is per-file; check is corpus-level.
- **CI**: bootstrap binary rebuilt automatically on every push to main changing `src/go/`.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Test fixture IDs ≥9000 reserved for unclaimed-ticket tests.

Autonomous-run policy is maintained in `AGENTS.md`.

## Status
<!-- generated 2026-05-12T10:40Z -->

**Tickets:** 4 ready · 2 blocked — `erg ready tickets/` for full list
**Recent commits:**
  f8876ef chore: rebuild bootstrap binary [skip ci]
  7b5f094 refactor(0132): extract resolveDir and resolveTicketByID helpers (#136)
  9a24f9a chore: rebuild bootstrap binary [skip ci]
  d11f311 feat(0126): configurable tag vocabulary via tickets/.ergrc (#135)
  9da21f1 chore: open ticket 0133 — downgrade format version to %erg 0.1 (#134)
