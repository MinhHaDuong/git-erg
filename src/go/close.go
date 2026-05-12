package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// summaryClose is the one-liner printed by printUsage via the commands registry.
const summaryClose = "Close a ticket atomically"

const helpClose = `## erg close ID|FILE REASON [DIR]

Atomically close a ticket.

Closing a ticket is a three-step operation:

  1. Inserts a Closed: REASON header at the end of the preamble (before ` + "`--- log ---`" + `).
  2. Appends a timestamped log line: ` + "`TIMESTAMP AUTHOR closed — REASON`" + `.
  3. Scans every open ticket in DIR for Blocked-by: ID and removes those lines,
     appending a log entry to each modified ticket:
     ` + "`TIMESTAMP AUTHOR note blocker ID closed — Blocked-by removed.`" + `
     Already-closed tickets that reference the ID are not modified. If a ticket
     has multiple Blocked-by: ID lines, all are removed in one pass.
     Step 3 iterates all open tickets; it is idempotent but not atomic.

ID may be a 4-digit ticket ID or a full filename (e.g. 0042-some-title.erg).
REASON must be non-empty. The operation is idempotent (safe to call twice for
the same ticket): closing an already-closed ticket prints 'CLOSED (already)' and
exits 0. Step 3 (Blocked-by removal) is also idempotent; re-running close on
an already-closed ticket does not re-scan dependents.
`

// cmdClose implements `erg close ID|FILE REASON [DIR]`. See helpClose for the user-facing summary.
func cmdClose(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: erg close ID|FILE REASON [DIR]")
		return 1
	}

	idOrFile := args[0]
	reason := args[1]
	var explicit string
	if len(args) >= 3 {
		explicit = args[2]
	}
	ticketDir, err := resolveDir(explicit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "close: %v\n", err)
		return 1
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
		ticketPath, err = resolveTicketByID(ticketDir, idOrFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "close: %v\n", err)
			return 1
		}
	}

	data, err := os.ReadFile(ticketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "close: cannot read %s: %v\n", ticketPath, err)
		return 1
	}

	ticket, _ := parseErgBytes(data, ticketPath)

	// Idempotent: already closed (Closed: header present or path test fires).
	if ticket.IsClosed() {
		fmt.Println("CLOSED (already)")
		return 0
	}

	now := time.Now().UTC().Format("2006-01-02T15:04Z")
	closedHeader := "Closed: " + reason
	author := resolveAuthor()
	logLine := fmt.Sprintf("%s %s closed — %s", now, author, reason)

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
	// Reuse the already-parsed ticket — FilenameID only uses Path.
	closedID := ticket.FilenameID()
	if closedID != "" {
		removeBlockedByRef(ticketDir, closedID, now, author)
	}

	fmt.Println("CLOSED")
	return 0
}

// removeBlockedByRef scans ticketDir for open tickets that have a
// Blocked-by header referencing closedID, removes those lines, and
// appends a log entry recording the removal. Uses loadErgs for
// consistent parsing/filtering, then re-reads only the matching files
// to perform the byte-level rewrite.
func removeBlockedByRef(ticketDir, closedID, timestamp, author string) {
	tickets, _ := loadErgs(ticketDir)
	for i := range tickets {
		t := &tickets[i]
		if t.IsClosed() {
			continue
		}
		found := false
		for _, ref := range t.BlockedBys {
			if ref.ID == closedID || ref.Raw == closedID {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		data, err := os.ReadFile(t.Path)
		if err != nil {
			continue
		}
		updated := removeBlockedByLine(string(data), closedID)
		logLine := fmt.Sprintf("%s %s note blocker %s closed — Blocked-by removed.", timestamp, author, closedID)
		updated = appendLogLine(updated, logLine)
		if err := os.WriteFile(t.Path, []byte(updated), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "close: warning: cannot write %s: %v\n", t.Path, err)
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
		if strings.TrimSpace(line) == separatorLog {
			logIdx = i
			break
		}
	}
	if logIdx < 0 {
		return "", fmt.Errorf("missing '%s' separator", separatorLog)
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
	bodyIdx := strings.Index(content, "\n"+separatorBody)
	if bodyIdx < 0 {
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		return content + logLine + "\n"
	}
	return content[:bodyIdx] + "\n" + logLine + content[bodyIdx:]
}
