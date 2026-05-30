# State — git-erg

_Last updated: 2026-05-30T21:00Z — Mega session: UX audit (0152) completed and merged (#221); security process doc (0201) added (#228); cross-worktree ticket-store rejection (0200) shipped (#232/#233); signed release tag 2026-05-30 (0151, #204). All 202 tickets closed and archived. Queue is empty — evidence-first before promoting anything._

## North star:

An agent-friendly local ticket system for development in disconnected environments.

## Milestones

- All audits complete (UX: 0152, security: 0151, data-safety: 0149). Queue is empty.
  Bar for new work: "verified empirical need" per AGENTS.md. Audits before features.

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
- feat: tests guaranteeing fundamental design criteria (disconnected, agnostic, discoverable)
- feat: AI script to realign docs and code
- audit: go coding practices, lint, smells

## Status
<!-- generated 2026-05-30T21:00Z (housekeeping sweep) -->
**Tickets:** 0 ready · 202 closed — queue empty · tests green
**Recent commits:**
  fd0a619 fix: drop stale erg log step in UX-PROCESS.md; archive 0195/0202
  8867787 ticket(0200): close and archive — PR #232
  d9daf8f ticket(0201): close and archive — PR #228
