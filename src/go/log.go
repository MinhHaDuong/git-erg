package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// summaryLog is the one-liner printed by printUsage via the commands registry.
const summaryLog = "Append a timestamped log entry to a ticket"

const helpLog = `## erg log ID LINE [DIR]

Append a timestamped entry to a ticket's log section.

Resolves the ticket by 4-digit ID in DIR (default: auto-discovered tickets/), then
prepends the current UTC timestamp (YYYY-MM-DDThh:mmZ) to LINE and inserts the
resulting line at the end of the log section, just before the ` + "`--- body ---`" + ` separator.

The resulting log entry format is:

  ` + "`YYYY-MM-DDThh:mmZ LINE`" + `

LINE must be non-empty. It must contain at least two whitespace-separated tokens
(e.g. "claude note retried with narrower scope"). The timestamp is prepended
automatically; erg validate (rule 11) enforces the structural format -- timestamp
followed by at least two tokens. By convention the first token is an actor
(who) and the second is a verb (what), but those names are not machine-checked.

Prints "LOGGED" on success. Exits non-zero if the ticket is not found or has no
` + "`--- body ---`" + ` separator (which would indicate a malformed file).
`

// cmdLog implements `erg log ID LINE [DIR]`. See helpLog for the user-facing summary.
func cmdLog(args []string) int {
	var positional []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			fmt.Fprintf(os.Stderr, "log: unknown flag %q\nUsage: erg log ID LINE [DIR]\n", a)
			return 1
		}
		positional = append(positional, a)
	}
	if len(positional) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: erg log ID LINE [DIR]")
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

	now := time.Now().UTC().Format("2006-01-02T15:04Z")
	logLine := now + " " + line

	// A state-altering command must never write a line the validator rejects.
	// The timestamp is ours; reuse logLineRE (rule 11's source of truth) to
	// confirm LINE supplies at least an actor and a verb before we commit it.
	if !logLineRE.MatchString(logLine) {
		fmt.Fprintf(os.Stderr,
			"log: %q is not a valid log entry -- LINE must be 'actor verb [detail]' (at least two words)\n", line)
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
