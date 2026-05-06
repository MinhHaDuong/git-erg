package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// cmdLog implements `erg log ID LINE [DIR]` — append a timestamped entry to a ticket's log section.
//
// Resolves the ticket by 4-digit ID in DIR (default: auto-discovered tickets/), then
// prepends the current UTC timestamp (YYYY-MM-DDThh:mmZ) to LINE and inserts the
// resulting line at the end of the log section, just before the --- body --- separator.
//
// The resulting log entry format is:
//
//	YYYY-MM-DDThh:mmZ LINE
//
// LINE must be non-empty. It should follow the log-line convention of
// `actor verb [detail]` (e.g. "claude note retried with narrower scope"), but this
// is not enforced — only format is validated on read by erg validate.
//
// Prints "LOGGED" on success. Exits non-zero if the ticket is not found or has no
// --- body --- separator (which would indicate a malformed file).
func cmdLog(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: erg log ID LINE [DIR]")
		return 1
	}

	id := args[0]
	line := args[1]
	ticketDir, err := findTicketsDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(args) >= 3 {
		ticketDir = args[2]
	}

	if strings.TrimSpace(line) == "" {
		fmt.Fprintln(os.Stderr, "log: line is required and must be non-empty")
		return 1
	}

	// Resolve to file path.
	pattern := filepath.Join(ticketDir, fmt.Sprintf("%s-*.erg", id))
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		fmt.Fprintf(os.Stderr, "log: no ticket found for ID %s in %s\n", id, ticketDir)
		return 1
	}
	if len(matches) > 1 {
		fmt.Fprintf(os.Stderr, "log: ambiguous ID %s — matches: %s\n", id, strings.Join(matches, ", "))
		return 1
	}
	ticketPath := matches[0]

	data, err := os.ReadFile(ticketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "log: cannot read %s: %v\n", ticketPath, err)
		return 1
	}

	if !strings.Contains(string(data), "\n--- body ---") {
		fmt.Fprintf(os.Stderr, "log: %s has no --- body --- separator — refusing to write\n", ticketPath)
		return 1
	}

	now := time.Now().UTC().Format("2006-01-02T15:04Z")
	logLine := now + " " + line

	content := appendLogLine(string(data), logLine)

	if err := os.WriteFile(ticketPath, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "log: cannot write %s: %v\n", ticketPath, err)
		return 1
	}

	fmt.Println("LOGGED")
	return 0
}
