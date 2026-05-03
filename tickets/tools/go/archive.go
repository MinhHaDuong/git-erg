package main

// This file implements `erg archive`: DAG-safe archival of old closed tickets.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// parseLogTimestamp extracts a time from a log line's ISO-8601 prefix.
func parseLogTimestamp(line string) (time.Time, bool) {
	line = strings.TrimSpace(line)
	if len(line) < 16 {
		return time.Time{}, false
	}
	tsStr := line
	if idx := strings.IndexByte(line[16:], ' '); idx >= 0 {
		tsStr = line[:16+idx]
	}
	tsStr = strings.TrimRight(tsStr, "Z")

	if t, err := time.Parse("2006-01-02T15:04:05", tsStr); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02T15:04", tsStr); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// cmdArchive implements `erg archive [dir] [--days N] [--execute]`.
func cmdArchive(args []string) int {
	execute := false
	days := 90
	ticketDir := "tickets"

	var filtered []string
	for _, a := range args {
		if a == "--execute" {
			execute = true
		} else {
			filtered = append(filtered, a)
		}
	}

	for i := 0; i < len(filtered); i++ {
		a := filtered[i]
		if strings.HasPrefix(a, "--days=") {
			if n, err := strconv.Atoi(a[7:]); err == nil {
				days = n
			}
		} else if a == "--days" && i+1 < len(filtered) {
			if n, err := strconv.Atoi(filtered[i+1]); err == nil {
				days = n
			}
			i++
		} else if !strings.HasPrefix(a, "--") {
			ticketDir = a
		}
	}

	info, err := os.Stat(ticketDir)
	if err != nil || !info.IsDir() {
		fmt.Printf("Directory not found: %s\n", ticketDir)
		return 1
	}

	tickets := loadErgs(ticketDir)
	cutoff := time.Now().UTC().AddDate(0, 0, -days)

	// Collect all IDs referenced by Blocked-by in live tickets
	referencedIDs := make(map[string]bool)
	allErgs := append([]Erg{}, tickets...)
	archiveDir := filepath.Join(ticketDir, "archive")
	if info, err := os.Stat(archiveDir); err == nil && info.IsDir() {
		allErgs = append(allErgs, loadErgs(archiveDir)...)
	}
	for i := range allErgs {
		for _, ref := range allErgs[i].BlockedBy() {
			if !strings.HasPrefix(ref, "gh#") {
				referencedIDs[ref] = true
			}
		}
	}

	var archivable, dagProtected []Erg
	for i := range tickets {
		t := &tickets[i]
		if t.Status() != "closed" {
			continue
		}

		// Determine age from last log line or Created header
		var lastTime time.Time
		var hasTime bool
		if len(t.LogLines) > 0 {
			lastTime, hasTime = parseLogTimestamp(t.LogLines[len(t.LogLines)-1])
		}
		if !hasTime {
			if created, ok := t.Headers["Created"]; ok && len(created) > 0 {
				if ct, err := time.Parse("2006-01-02", created[0]); err == nil {
					lastTime = ct
					hasTime = true
				}
			}
		}
		if !hasTime || !lastTime.Before(cutoff) {
			continue
		}

		id := t.FilenameID()
		if referencedIDs[id] {
			dagProtected = append(dagProtected, *t)
		} else {
			archivable = append(archivable, *t)
		}
	}

	if len(dagProtected) > 0 {
		var ids []string
		for _, t := range dagProtected {
			ids = append(ids, t.FilenameID())
		}
		fmt.Printf("DAG-protected (skipping %d): %s\n", len(dagProtected), strings.Join(ids, ", "))
	}

	if len(archivable) == 0 {
		fmt.Printf("Nothing to archive (threshold: %d days).\n", days)
		return 0
	}

	var ids []string
	for _, t := range archivable {
		ids = append(ids, t.FilenameID())
	}
	fmt.Printf("Will archive %d ticket(s): %s\n", len(archivable), strings.Join(ids, ", "))

	if !execute {
		fmt.Println("Dry run. Pass --execute to proceed.")
		return 0
	}

	os.MkdirAll(archiveDir, 0755)

	for _, t := range archivable {
		dest := filepath.Join(archiveDir, t.Filename())
		cmd := exec.Command("git", "mv", t.Path, dest)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "git mv failed for %s\n", t.Filename())
			return 1
		}
		fmt.Printf("  moved %s\n", t.Filename())
	}

	msg := fmt.Sprintf("archive %d closed tickets (>%d days, DAG-safe)", len(archivable), days)
	cmd := exec.Command("git", "commit", "-m", msg)
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "git commit failed")
		return 1
	}
	fmt.Printf("Committed: %s\n", msg)
	return 0
}
