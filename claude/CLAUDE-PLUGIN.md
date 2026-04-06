# Ticket system

This project uses `%erg v1` local tickets for work coordination.
You read and write `.erg` text files directly — no CLI needed.

## Commands

- `/ticket-new [title]` — create a ticket
- `/ticket-ready` — list unblocked, unclaimed tickets
- `/ticket-claim [id]` — claim a ticket for work
- `/ticket-close [id]` — close a ticket
- `/ticket-release [id]` — release a claimed ticket

## Workflow

1. `/ticket-ready` to see what's available
2. `/ticket-claim 0042` to start work
3. Do the work
4. `/ticket-close 0042` when done

## Notes

- The validator lives in `tickets/tools/go/` with its own `go.mod` — this is isolated from any project-level Go modules.
- Build it with `cd tickets/tools/go && go build -o erg .`

## Format spec

@.claude/rules/tickets.md
