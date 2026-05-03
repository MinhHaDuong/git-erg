// `erg close` — atomic ticket closure.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var statusLineRE = regexp.MustCompile(`(?m)^Status:.*$`)

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

	// Resolve to file path
	var ticketPath string
	if strings.HasSuffix(idOrFile, ".erg") {
		// Provided a file path directly
		ticketPath = idOrFile
	} else {
		// Provided a 4-digit ID — glob for it under ticketDir
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

	// Read and parse
	data, err := os.ReadFile(ticketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "close: cannot read %s: %v\n", ticketPath, err)
		return 1
	}

	ticket := parseErg(ticketPath)

	// Idempotent: already closed
	if ticket.Status() == "closed" {
		fmt.Println("ALREADY_CLOSED")
		return 0
	}

	// Replace Status line only within the header section (before "--- log ---").
	// Splitting the file at the log separator bounds the regex so a "Status:"
	// line inside the body (e.g. inside a code fence or quoted example) is
	// never rewritten.
	raw := string(data)
	logSep := "\n--- log ---"
	logIdx := strings.Index(raw, logSep)
	var content string
	if logIdx < 0 {
		// No log separator yet — operate on the whole file (validator will
		// reject it later, but we should still behave deterministically).
		content = statusLineRE.ReplaceAllString(raw, "Status: closed")
	} else {
		header := statusLineRE.ReplaceAllString(raw[:logIdx], "Status: closed")
		content = header + raw[logIdx:]
	}

	// Append log line before "--- body ---"
	now := time.Now().UTC().Format("2006-01-02T15:04Z")
	logLine := fmt.Sprintf("%s claude status closed — %s", now, reason)

	bodyIdx := strings.Index(content, "\n--- body ---")
	if bodyIdx < 0 {
		// Fallback: append at end
		content = content + "\n" + logLine + "\n"
	} else {
		// Insert log line before the body separator
		content = content[:bodyIdx] + "\n" + logLine + content[bodyIdx:]
	}

	// Write back
	if err := os.WriteFile(ticketPath, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "close: cannot write %s: %v\n", ticketPath, err)
		return 1
	}

	fmt.Println("CLOSED")
	return 0
}
