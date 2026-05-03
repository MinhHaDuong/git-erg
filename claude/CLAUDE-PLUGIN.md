# Ticket system

This project uses `%erg v1` local tickets for work coordination.
You read and write `.erg` text files directly.

## Prerequisites

`erg` must be on PATH. If missing: `make install-erg-binary` from the repo root.

## Commands

- `/ticket-new [title]` — create a ticket
- `/ticket-ready` — list unblocked tickets
- `/ticket-close [id]` — close a ticket

## Workflow

1. `/ticket-ready` to see what's available
2. Pick a ticket and do the work
3. `/ticket-close 0042` when done

## Notes

- The validator lives in `tickets/tools/go/` with its own `go.mod` — this is isolated from any project-level Go modules.

## Format spec

@.claude/rules/tickets.md
