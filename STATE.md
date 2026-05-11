# State — git-erg

_Last updated: 2026-05-11T11:40Z — Housekeeping: stale worktree + branch (t0115) cleaned; 3 open tickets staged for next refactor wave._

## Stats

- Tickets: 115 closed, 3 open
- Tests: green — ok git-erg (24.4% coverage)
- Open PRs: none

## Ready to work

- **0116** — Erg struct: schema-literal fields, drop accessors and the headers map. Completes the unfinished half of 0105. Bundles three changes deliberately: struct refactor + spec rule-2 tightening (required headers non-empty) + spec rule-11 relaxation (separator first-occurrence). Heavily reviewed (4-angle review folded in); endpoint shape documented.
- **0117** — Merge parseErg and validateErg into a single validating-parser pass. Blocked by 0116.
- **0118** — Tighten Erg field types: `BlockedBys []Ref`, `Created time.Time`. Blocked by 0117.

## Blocked

- Nothing externally blocked. 0117 and 0118 chain on 0116.

## Notes

- **Refactor wave 2026-05-11** (PRs #119–#122): 0113 renamed non-command Go files (model/erg, helptext, identity); 0114 moved help strings into command files; 0115 split Ref into ref.go; 0106 (reopened with corrected rationale) renamed `Tags:` → `Tag:` per repeatable-header convention.
- **0116 chain plan**: 0116 → 0117 → 0118. Each delivers value standalone. Endpoint: schema-pure `Erg` with `Created time.Time`, `BlockedBys []Ref`, zero accessor methods except `IsClosed()`.
- **erg validate vs erg check**: validate is per-file; check is corpus-level.
- **CI**: bootstrap binary rebuilt automatically on every push to main changing `src/go/`.
- **Testing policy**: Go unit tests own pure-function correctness; shell integration tests own CLI black-box behavior. Test fixture IDs ≥9000 reserved for unclaimed-ticket tests.

Autonomous-run policy is maintained in `AGENTS.md`.
