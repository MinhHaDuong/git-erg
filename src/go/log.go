package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// summaryLog is the one-liner printed by printUsage via the commands registry.
const summaryLog = "Append a timestamped log entry to a ticket"

const helpLog = `## erg log ID LINE [DIR] [--author NAME]

Append a timestamped entry to a ticket's log section.

Resolves the ticket by 4-digit ID in DIR (default: auto-discovered tickets/), then
prepends the current UTC timestamp (YYYY-MM-DDThh:mmZ) AND the resolved author to
LINE, and inserts the resulting line at the end of the log section, just before
the ` + "`--- body ---`" + ` separator.

The resulting log entry format is:

  ` + "`YYYY-MM-DDThh:mmZ AUTHOR LINE`" + `

So LINE supplies ` + "`VERB [detail]`" + ` -- NOT the author. A bare verb is a
complete entry ("reopened"); LINE must simply be non-empty.

The author is resolved exactly as ` + "`erg new`" + ` resolves it: --author NAME wins,
else $ERG_AUTHOR, else git config user.name, else $USER, else "unknown".

CONTRACT CHANGE (ticket 0276). Before this release LINE carried the actor too,
and nothing checked that it did: ` + "`erg log ID \"note fixed it\"`" + ` wrote
` + "`<ts> note fixed it`" + `, putting a verb in the actor slot. That could not be
validated after the fact -- the format is positionally ambiguous, so
` + "`<ts> A B ...`" + ` parses whether A is an actor or a verb, and erg validate
(rule 11) passed such lines. The actor is now supplied rather than policed, which
removes the failure mode at its source and needs no verb vocabulary.

Callers written against the old contract must drop the actor from LINE, or pass
it as --author. Passing it in LINE now doubles it.

Prints "LOGGED" on success. Exits non-zero if the ticket is not found or has no
` + "`--- body ---`" + ` separator (which would indicate a malformed file).
`

// cmdLog implements `erg log ID LINE [DIR]`. See helpLog for the user-facing summary.
func cmdLog(args []string) int {
	var positional []string
	var authorFlag string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--author":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "log: --author requires a value")
				return 1
			}
			i++
			authorFlag = args[i]
		case strings.HasPrefix(a, "--author="):
			authorFlag = strings.TrimPrefix(a, "--author=")
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "log: unknown flag %q\nUsage: erg log ID LINE [DIR] [--author NAME]\n", a)
			return 1
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: erg log ID LINE [DIR] [--author NAME]")
		return 1
	}

	id := positional[0]
	line := positional[1]
	var explicit string
	if len(positional) >= 3 {
		explicit = positional[2]
	}
	ticketDir, err := resolveDir(explicit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "log: %v\n", err)
		return 1
	}

	if strings.TrimSpace(line) == "" {
		fmt.Fprintln(os.Stderr, "log: line is required and must be non-empty")
		return 1
	}

	ticketPath, err := resolveTicketByID(ticketDir, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "log: %v\n", err)
		return 1
	}

	data, err := os.ReadFile(ticketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "log: cannot read %s: %v\n", ticketPath, err)
		return 1
	}

	if !strings.Contains(string(data), "\n"+separatorBody) {
		fmt.Fprintf(os.Stderr, "log: %s has no %s separator -- refusing to write\n", ticketPath, separatorBody)
		return 1
	}

	// The actor is supplied, not policed (ticket 0276). Policing it after the
	// fact is impossible: the format is positionally ambiguous, so logLineRE
	// cannot tell `<ts> actor verb` from `<ts> verb detail` and passes both.
	// Supplying it removes the failure mode at its source, and reuses the
	// resolution every sibling verb already shares.
	author := sanitizeAuthor(authorFlag)
	if author == "" {
		author = resolveAuthor()
	}

	now := time.Now().UTC().Format("2006-01-02T15:04Z")
	logLine := now + " " + author + " " + line

	// A state-altering command must never write a line the validator rejects.
	// Timestamp and actor are ours; this confirms LINE supplied a verb.
	if !logLineRE.MatchString(logLine) {
		fmt.Fprintf(os.Stderr,
			"log: %q is not a valid log entry -- LINE must be 'verb [detail]', at least one word "+
				"(the author is supplied automatically; pass --author NAME to override)\n", line)
		return 1
	}

	content := appendLogLine(string(data), logLine)

	if err := writeTicketAtomic(ticketDir, ticketPath, []byte(content)); err != nil {
		fmt.Fprintf(os.Stderr, "log: cannot write %s: %v\n", ticketPath, err)
		return 1
	}

	fmt.Println("LOGGED")
	return 0
}
