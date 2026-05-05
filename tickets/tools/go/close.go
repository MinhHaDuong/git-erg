package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// cmdClose implements `erg close <id|file> <reason> [dir]`.
func cmdClose(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: erg close <id|file> <reason> [dir]")
		return 1
	}

	idOrFile := args[0]
	reason := args[1]
	ticketDir := "tickets"
	if len(args) >= 3 {
		ticketDir = args[2]
	}

	if strings.TrimSpace(reason) == "" {
		fmt.Fprintln(os.Stderr, "close: reason is required and must be non-empty")
		return 1
	}

	// Resolve to file path.
	var ticketPath string
	if strings.HasSuffix(idOrFile, ".erg") {
		ticketPath = idOrFile
	} else {
		pattern := filepath.Join(ticketDir, fmt.Sprintf("%s-*.erg", idOrFile))
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			fmt.Fprintf(os.Stderr, "close: no ticket found for ID %s in %s\n", idOrFile, ticketDir)
			return 1
		}
		if len(matches) > 1 {
			fmt.Fprintf(os.Stderr, "close: ambiguous ID %s — matches: %s\n", idOrFile, strings.Join(matches, ", "))
			return 1
		}
		ticketPath = matches[0]
	}

	data, err := os.ReadFile(ticketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "close: cannot read %s: %v\n", ticketPath, err)
		return 1
	}

	ticket := parseErg(ticketPath)

	// Idempotent: already closed (Closed: header present or path test fires).
	if ticket.Closed() {
		fmt.Println("CLOSED (already)")
		return 0
	}

	now := time.Now().UTC().Format("2006-01-02T15:04Z")
	closedHeader := "Closed: " + reason
	logLine := fmt.Sprintf("%s claude closed — %s", now, reason)

	content, err := insertClosedHeader(string(data), closedHeader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "close: %v\n", err)
		return 1
	}
	content = appendLogLine(content, logLine)

	if err := os.WriteFile(ticketPath, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "close: cannot write %s: %v\n", ticketPath, err)
		return 1
	}

	// Step 3: remove Blocked-by: <id> lines from dependent open tickets.
	t2 := parseErg(ticketPath)
	closedID := t2.FilenameID()
	if closedID != "" {
		removeBlockedByRef(ticketDir, closedID, now)
	}

	fmt.Println("CLOSED")
	return 0
}

// removeBlockedByRef scans ticketDir for open tickets that have a
// Blocked-by header referencing closedID, removes those lines, and
// appends a log entry recording the removal.
func removeBlockedByRef(ticketDir, closedID, timestamp string) {
	entries, err := os.ReadDir(ticketDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".erg") {
			continue
		}
		path := filepath.Join(ticketDir, entry.Name())
		t := parseErg(path)
		if t.Closed() {
			continue
		}
		refs, ok := t.Headers["Blocked-by"]
		if !ok {
			continue
		}
		found := false
		for _, r := range refs {
			if strings.TrimSpace(r) == closedID {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "close: warning: cannot read %s: %v\n", path, err)
			continue
		}
		updated := removeBlockedByLine(string(data), closedID)
		logLine := fmt.Sprintf("%s claude note blocker %s closed — Blocked-by removed", timestamp, closedID)
		updated = appendLogLine(updated, logLine)
		if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "close: warning: cannot write %s: %v\n", path, err)
		}
	}
}

// removeBlockedByLine removes "Blocked-by: <id>" lines matching id from content.
func removeBlockedByLine(content, id string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	prefix := "Blocked-by: " + id
	for _, line := range lines {
		if strings.TrimRight(line, " \t") == prefix {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// insertClosedHeader inserts a `Closed: …` header at the end of the
// preamble — after the last existing header line, before the blank line
// that precedes `--- log ---`. The preamble bound is the literal
// `--- log ---` separator on its own line.
func insertClosedHeader(content, headerLine string) (string, error) {
	lines := strings.Split(content, "\n")
	logIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "--- log ---" {
			logIdx = i
			break
		}
	}
	if logIdx < 0 {
		return "", fmt.Errorf("missing '--- log ---' separator")
	}

	// Find the last non-blank line before the log separator. That's the
	// last header. Insert immediately after it; any trailing blank lines
	// between it and the separator are preserved.
	insertAt := logIdx
	for insertAt > 0 && strings.TrimSpace(lines[insertAt-1]) == "" {
		insertAt--
	}

	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:insertAt]...)
	out = append(out, headerLine)
	out = append(out, lines[insertAt:]...)
	return strings.Join(out, "\n"), nil
}

// appendLogLine inserts a log line at the end of the log section, just
// before the `--- body ---` separator. If the file lacks a body separator,
// appends to the end of the file.
func appendLogLine(content, logLine string) string {
	bodyIdx := strings.Index(content, "\n--- body ---")
	if bodyIdx < 0 {
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		return content + logLine + "\n"
	}
	return content[:bodyIdx] + "\n" + logLine + content[bodyIdx:]
}
