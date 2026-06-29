package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// summaryRm is the one-liner printed by printUsage via the commands registry.
const summaryRm = "Delete a ticket (DAG-checked; --force clears dependents)"

const helpRm = `## erg rm ID|FILE [DIR] [--force]

Delete a ticket file outright -- no Closed: header, no archive, no record.

Use rm only for tickets that should never have existed: a duplicate, a
typo-titled file, a fat-fingered draft, spam. For work that was done or
abandoned with history worth keeping, use 'erg close' (keeps the file, adds
a Closed: header) or 'erg archive' (moves it under closed/). Only rm removes
the record entirely.

Deletion is destructive and irreversible from the tool's side, so rm verifies
the dependency graph before touching the filesystem:

  - By default, if any ticket in the corpus (open OR closed) has a Blocked-by:
    referencing the target ID, rm refuses: it prints each dependent and exits
    non-zero WITHOUT deleting anything. The closed tickets are scanned too -- a
    closed ticket may carry a historical Blocked-by: line, and deleting its
    blocker would leave a dangling ref that 'erg check' flags.
  - With --force, rm deletes the target and strips the now-dangling Blocked-by:
    lines from every dependent (open or closed), appending a log entry to each:
    ` + "`TIMESTAMP AUTHOR note blocker ID removed \u2014 ticket deleted.`" + `

ID may be a 4-digit ticket ID or a full filename (e.g. 0042-some-title.erg).
A non-existent or ambiguous ID is reported with the usual resolver error.
`

// cmdRm implements `erg rm ID|FILE [DIR] [--force]`. See helpRm for the
// user-facing summary.
func cmdRm(args []string) int {
	force := false
	var positional []string
	for _, a := range args {
		switch a {
		case "--force", "-f":
			force = true
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(os.Stderr, "rm: unknown flag %q (usage: erg rm ID|FILE [DIR] [--force])\n", a)
				return 1
			}
			positional = append(positional, a)
		}
	}

	if len(positional) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: erg rm ID|FILE [DIR] [--force]")
		return 1
	}

	idOrFile := positional[0]
	var explicit string
	if len(positional) >= 2 {
		explicit = positional[1]
	}

	// Resolve to a file path, reusing the same ID/FILE resolution and error
	// messages as the other mutating commands.
	var ticketPath, ticketDir string
	var err error
	if strings.HasSuffix(idOrFile, ".erg") {
		ticketPath = idOrFile
		if _, err := os.Stat(ticketPath); err != nil {
			fmt.Fprintf(os.Stderr, "rm: no ticket found at %s\n", ticketPath)
			return 1
		}
		// Scan the corpus the file actually lives in for dependents. An
		// explicit DIR wins; otherwise infer the store from the file's own
		// directory rather than auto-discovering an unrelated store, which
		// would bypass the DAG guard for a FILE path outside the default store.
		scanArg := explicit
		if scanArg == "" {
			scanArg = filepath.Dir(ticketPath)
		}
		ticketDir, err = resolveDir(scanArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "rm: %v\n", err)
			return 1
		}
	} else {
		ticketDir, err = resolveDir(explicit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "rm: %v\n", err)
			return 1
		}
		ticketPath, err = resolveTicketByID(ticketDir, idOrFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "rm: %v\n", err)
			return 1
		}
	}

	// Confine the delete to the resolved store, matching the write path's
	// fail-safe rail (0149 writeTicketAtomic/withinStore). rm's FILE form
	// resolved ticketPath then called os.Remove directly, never withinStore --
	// so an explicit store DIR the FILE escapes (e.g. `erg rm /tmp/outside.erg
	// <store>`) would delete a file outside it. Delete is as irreversible as a
	// write, so the same rail applies. This is a fat-finger guard, not a
	// security boundary (a determined caller passes the file's own dir as DIR);
	// it stops the common mistake before it lands. The ID form resolves within
	// ticketDir already, so the gate is a no-op there.
	if ok, werr := withinStore(ticketDir, ticketPath); werr != nil {
		fmt.Fprintf(os.Stderr, "rm: %v\n", werr)
		return 1
	} else if !ok {
		absStore, _ := filepath.Abs(ticketDir)
		fmt.Fprintf(os.Stderr, "rm: refusing to delete %s: target is outside the ticket store %s "+
			"(pass the directory that contains the file, or omit DIR to infer it)\n", ticketPath, absStore)
		return 1
	}

	target, _ := parseErg(ticketPath)
	targetID := target.FilenameID()

	// Scan the whole corpus (open AND closed) for dependents: tickets with a
	// local Blocked-by ref resolving to the target ID. Non-local refs never point
	// at local tickets, so only RefLocal matches.
	all, _ := loadErgs(ticketDir)
	cleanTarget := filepath.Clean(ticketPath)
	var dependents []string
	for i := range all {
		t := &all[i]
		if filepath.Clean(t.Path) == cleanTarget {
			continue
		}
		for _, ref := range t.BlockedBys {
			if ref.MatchesLocalID(targetID) {
				dependents = append(dependents, t.Filename())
				break
			}
		}
	}

	// Refuse by default when dependents exist: deletion is irreversible, so
	// the safe default is to stop rather than silently rewrite other tickets.
	if len(dependents) > 0 && !force {
		fmt.Fprintf(os.Stderr, "rm: refusing to delete %s -- %d ticket(s) depend on it:\n",
			target.Filename(), len(dependents))
		for _, d := range dependents {
			fmt.Fprintf(os.Stderr, "  %s: Blocked-by: %s\n", d, targetID)
		}
		fmt.Fprintln(os.Stderr, "Re-run with --force to delete and clear these Blocked-by lines.")
		return 1
	}

	if err := os.Remove(ticketPath); err != nil {
		fmt.Fprintf(os.Stderr, "rm: cannot delete %s: %v\n", ticketPath, err)
		return 1
	}

	// With --force, clear the now-dangling Blocked-by edges from dependents,
	// reusing close's rewrite machinery (includeClosed=true so historical refs
	// in closed dependents are cleared too, keeping `erg check` clean).
	if force && len(dependents) > 0 && targetID != "" {
		now := time.Now().UTC().Format("2006-01-02T15:04Z")
		author := resolveAuthor()
		logLine := fmt.Sprintf("%s %s note blocker %s removed \u2014 ticket deleted.", now, author, targetID)
		clearBlockedByRefs(ticketDir, targetID, logLine, true)
	}

	fmt.Printf("DELETED %s\n", target.Filename())
	return 0
}
