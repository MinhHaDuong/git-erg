package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// cmdArchive implements `erg archive [id...] [dir]` — move closed tickets to dir/closed/.
//
// With no IDs, scans the top-level of dir (default: tickets/) for tickets that
// have a non-empty Closed: header and are not already inside a closed/ directory,
// then moves each eligible ticket to dir/closed/. With IDs given, archives only
// the named tickets.
//
// A ticket is skipped (with a SKIPPED message) if any open ticket in dir still
// has a Blocked-by: pointing to its ID; archiving would silently break that ref.
// Run `erg close ID REASON` (which removes Blocked-by refs automatically) before
// archiving, or manually delete the stale Blocked-by line.
//
// The command creates dir/closed/ if it does not exist. It will not overwrite
// an existing file at the destination.
func cmdArchive(args []string) int {
	ticketDir, err := findTicketsDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var ids []string

	for _, a := range args {
		if len(a) == 4 && allDigits(a) {
			ids = append(ids, a)
		} else if !strings.HasPrefix(a, "-") {
			ticketDir = a
		}
	}

	info, err := os.Stat(ticketDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: %v\n", err)
		return 1
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "archive: %s is not a directory\n", ticketDir)
		return 1
	}

	// Load all tickets recursively (includes closed/) for building the
	// reverse blocker index.
	allTickets := loadErgs(ticketDir)

	// Build reverse index: for each open ticket X, for each local Blocked-by
	// ref R in X, record that R.ID is blocking X.FilenameID().
	// blockedBy[closedTicketID] = list of open ticket IDs that mention it.
	blockedBy := make(map[string][]string)
	for i := range allTickets {
		t := &allTickets[i]
		if t.Closed() {
			continue
		}
		refs, errs := t.BlockedByRefs()
		for j, ref := range refs {
			if errs[j] != nil || ref.Kind != RefLocal {
				continue
			}
			blockedBy[ref.ID] = append(blockedBy[ref.ID], t.FilenameID())
		}
	}

	// Collect target tickets.
	var targets []Erg

	if len(ids) > 0 {
		// ID mode: resolve each ID to a file in the top-level ticketDir only.
		for _, id := range ids {
			pattern := filepath.Join(ticketDir, fmt.Sprintf("%s-*.erg", id))
			matches, err := filepath.Glob(pattern)
			if err != nil || len(matches) == 0 {
				fmt.Fprintf(os.Stderr, "archive: no ticket found for ID %s in %s\n", id, ticketDir)
				continue
			}
			if len(matches) > 1 {
				fmt.Fprintf(os.Stderr, "archive: ambiguous ID %s — matches: %s\n", id, strings.Join(matches, ", "))
				continue
			}
			t := parseErg(matches[0])
			targets = append(targets, t)
		}
	} else {
		// Default mode: scan top-level ticketDir for tickets with a non-empty
		// Closed: header that are NOT already in a closed/ directory.
		entries, err := os.ReadDir(ticketDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "archive: cannot read %s: %v\n", ticketDir, err)
			return 1
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".erg") {
				continue
			}
			path := filepath.Join(ticketDir, entry.Name())
			t := parseErg(path)
			targets = append(targets, t)
		}
	}

	closedDir := filepath.Join(ticketDir, "closed")
	exitCode := 0

	for _, t := range targets {
		// Skip tickets without a non-empty Closed: header.
		// We use header-only check (not t.Closed()) to avoid re-processing
		// tickets that are already path-closed.
		if !t.ClosedHeader() {
			continue
		}

		tid := t.FilenameID()

		// Skip if any open ticket is blocking on this one.
		if blockers := blockedBy[tid]; len(blockers) > 0 {
			fmt.Printf("SKIPPED %s (needed by %s)\n", t.Filename(), strings.Join(blockers, ", "))
			continue
		}

		// Ensure destination directory exists.
		if err := os.MkdirAll(closedDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "archive: cannot create %s: %v\n", closedDir, err)
			exitCode = 1
			continue
		}

		dst := filepath.Join(closedDir, t.Filename())

		// Check for collision.
		if _, err := os.Stat(dst); err == nil {
			fmt.Fprintf(os.Stderr, "archive: destination already exists, skipping: %s\n", dst)
			continue
		}

		if err := os.Rename(t.Path, dst); err != nil {
			fmt.Fprintf(os.Stderr, "archive: cannot move %s: %v\n", t.Filename(), err)
			exitCode = 1
			continue
		}

		fmt.Printf("ARCHIVED %s\n", t.Filename())
	}

	return exitCode
}
