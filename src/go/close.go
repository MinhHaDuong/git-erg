package main

import (
	"fmt"
	"os"
	"path/filepath"
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

	if strings.TrimSpace(reason) == "" {
		fmt.Fprintln(os.Stderr, "close: reason is required and must be non-empty")
		return 1
	}

	// Resolve to a file path and the corpus to scan for dependents (step 3).
	// For the FILE form with no explicit DIR, the corpus is the store the file
	// actually lives in — inferred from its directory — not an auto-discovered
	// store, which would otherwise leave Blocked-by edges dangling in the
	// file's real store (mirrors the cmdRm guard).
	var ticketPath, ticketDir string
	var err error
	if strings.HasSuffix(idOrFile, ".erg") {
		ticketPath = idOrFile
		scanArg := explicit
		if scanArg == "" {
			scanArg = filepath.Dir(ticketPath)
		}
		ticketDir, err = resolveDir(scanArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "close: %v\n", err)
			return 1
		}
	} else {
		ticketDir, err = resolveDir(explicit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "close: %v\n", err)
			return 1
		}
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
	content = string(collapseHeaderBlanks([]byte(content)))

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
// appends a log entry recording the removal. Thin wrapper over
// clearBlockedByRefs: close only ever rewrites OPEN dependents (a closed
// dependent's historical Blocked-by line is left intact).
func removeBlockedByRef(ticketDir, closedID, timestamp, author string) {
	logLine := fmt.Sprintf("%s %s note blocker %s closed — Blocked-by removed.", timestamp, author, closedID)
	clearBlockedByRefs(ticketDir, closedID, logLine, false)
}

// clearBlockedByRefs scans ticketDir for tickets whose Blocked-by header
// references targetID, strips those lines, and appends logLine to each
// rewritten ticket. Uses loadErgs for consistent parsing/filtering, then
// re-reads only the matching files to perform the byte-level rewrite. When
// includeClosed is false, already-closed dependents are skipped (close's
// behaviour: their historical refs are preserved). When true, closed
// dependents are rewritten too (rm's behaviour: a dangling ref left in a
// closed ticket would still trip the unknown-ref corpus check). The rewrite
// machinery — removeBlockedByLine, appendLogLine, collapseHeaderBlanks — is
// shared with close. Iterates all matching tickets; idempotent but not atomic.
func clearBlockedByRefs(ticketDir, targetID, logLine string, includeClosed bool) {
	tickets, _ := loadErgs(ticketDir)
	for i := range tickets {
		t := &tickets[i]
		if !includeClosed && t.IsClosed() {
			continue
		}
		found := false
		for _, ref := range t.BlockedBys {
			if ref.MatchesLocalID(targetID) {
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
		updated := removeBlockedByLine(string(data), targetID)
		updated = appendLogLine(updated, logLine)
		updated = string(collapseHeaderBlanks([]byte(updated)))
		if err := os.WriteFile(t.Path, []byte(updated), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot write %s: %v\n", t.Path, err)
		}
	}
}

// removeBlockedByLine removes Blocked-by header lines matching id from content.
// It parses each line with parseHeaderLine — the same parser that populated
// Erg.BlockedBys for dependency detection — so it strips every form the parser
// tolerates (e.g. "Blocked-by : 0001" with whitespace before the colon or
// trailing whitespace), not just the canonical "Blocked-by: <id>" spelling.
// Detecting a dependent but failing to remove its edge would leave a dangling
// ref that `erg check` then flags.
func removeBlockedByLine(content, id string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if key, val, ok := parseHeaderLine(line); ok && key == "Blocked-by" {
			if ref, err := parseRef(val); err == nil && ref.MatchesLocalID(id) {
				continue
			}
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
