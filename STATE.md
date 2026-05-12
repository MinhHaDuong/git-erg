# State — git-erg

_Last updated: 2026-05-12T15:30Z — Raid closed 0 open tickets (0129, 0131 deferred→0, 0134, 0135, 0136, 0137). PRs #141-#144 merged. 1 deferred remains._

## Stats

- Tickets: 135 closed, 1 open (deferred)
- Tests: green — ok git-erg (37.3% coverage)
- Open PRs: none

## Ready to work

_(none)_

## Deferred

- **0131** — Add CLI integration tests via TestMain exec pattern. YAGNI: 19 shell test suites already cover CLI surface. Revisit if specific debugging gap arises.

## Notes

- **0116 chain plan**: 0116 → 0117 → 0118. Each delivers value standalone. Endpoint: schema-pure `Erg` with `Created time.Time`, `BlockedBys []Ref`, zero accessor methods except `IsClosed()`. All three closed.
- **erg validate vs erg check**: validate is per-file; check is corpus-level.
- **CI**: bootstrap binary rebuilt automatically on every push to main changing `src/go/`.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Test fixture IDs ≥9000 reserved for unclaimed-ticket tests.
- **Blocked-by grammar**: now supports `path-ref` (e.g. `module/0042`) for cross-module refs in multi-module monorepos; resolver implementation deferred.

Autonomous-run policy is maintained in `AGENTS.md`.

## Status
<!-- generated 2026-05-12T15:30Z -->

**Tickets:** 0 ready · 1 deferred — `erg ready tickets/` for full list
**Recent commits:**
  2377a8b docs(0129): document URL-shortcut Blocked-by ref grammar (spec-only) (#144)
  ef6419b docs(0135,0136): fix PEP section 12 Tags→Tag drift and update Modified date (#143)
  d5c3d60 chore(0137): fix stale source comments (Tags→Tag, rules/tickets.md refs) (#142)
  a45cf8f docs(0134): reconcile src/go/assets/spec-erg-v1.md with tickets copy (#141)
  39b19ed feat(0130): add erg tag/untag CLI commands (#140)
