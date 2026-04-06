# Ticket system

This project uses `%erg v1` local tickets for work coordination.

## Quick reference

- Tickets live in `tickets/` as `.erg` text files
- Spec: `.claude/rules/tickets.md` (read this first if unsure about format)
- Validator: `tickets/tools/go/erg` (build with `cd tickets/tools/go && go build -o erg .`)

## Slash commands

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

## Key rules

- You read and write `.erg` files directly — no CLI needed for normal operations
- The Go binary is a guardrail (pre-commit hook), not your interface
- IDs are 4-digit zero-padded sequential numbers
- Append to the log section, never edit existing log lines
- Check `.git/ticket-wip/{ID}.wip` before claiming to avoid conflicts
