// erg — validate, ready, archive, graph, close %erg v1 files.
// No external dependencies (stdlib only).
//
// Usage:
//
//	erg validate [dir|file ...]
//	erg ready    [dir] [--json]
//	erg archive  [dir] [--days N] [--execute]
//	erg graph    [dir] [--json]
//	erg next-id  [dir]
//	erg close    <id|file> <reason> [dir]
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Next-ID — print the next available ticket ID
// ---------------------------------------------------------------------------

func cmdNextID(args []string) int {
	ticketDir := "tickets"
	if len(args) > 0 {
		ticketDir = args[0]
	}

	maxID := 0

	// Scan both tickets/ and tickets/archive/
	for _, dir := range []string{ticketDir, filepath.Join(ticketDir, "archive")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".erg") {
				continue
			}
			stem := strings.TrimSuffix(e.Name(), ".erg")
			if idx := strings.Index(stem, "-"); idx > 0 {
				stem = stem[:idx]
			}
			if n, err := strconv.Atoi(stem); err == nil && n > maxID {
				maxID = n
			}
		}
	}

	fmt.Printf("%04d\n", maxID+1)
	return 0
}

// ---------------------------------------------------------------------------
// Close — atomic ticket closure
// ---------------------------------------------------------------------------

var statusLineRE = regexp.MustCompile(`(?m)^Status:.*$`)

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

// ---------------------------------------------------------------------------
// Main dispatch
// ---------------------------------------------------------------------------

const updateURL = "https://raw.githubusercontent.com/MinhHaDuong/git-erg/main/tickets/tools/go/erg"

func selfHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func cmdVersion(_ []string) int {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "version: cannot resolve executable: %v\n", err)
		return 1
	}
	h, err := selfHash(self)
	if err != nil {
		fmt.Fprintf(os.Stderr, "version: cannot hash self: %v\n", err)
		return 1
	}
	fmt.Println(h)
	return 0
}

func cmdUpdate(_ []string) int {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "update: cannot resolve executable: %v\n", err)
		return 1
	}

	localHash, err := selfHash(self)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update: cannot hash self: %v\n", err)
		return 1
	}

	url := os.Getenv("ERG_UPDATE_URL")
	if url == "" {
		url = updateURL
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update: offline or unreachable — %v\n", err)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "update: server returned %d\n", resp.StatusCode)
		return 0
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update: failed to read response: %v\n", err)
		return 0
	}

	sum := sha256.Sum256(body)
	remoteHash := hex.EncodeToString(sum[:])

	if localHash == remoteHash {
		fmt.Println("erg: already up to date")
		return 0
	}

	tmp := self + ".tmp"
	if err := os.WriteFile(tmp, body, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "update: cannot write temp file: %v\n", err)
		return 1
	}
	if err := os.Rename(tmp, self); err != nil {
		_ = os.Remove(tmp)
		fmt.Fprintf(os.Stderr, "update: cannot replace binary: %v\n", err)
		return 1
	}

	fmt.Printf("erg: updated (%s → %s)\n", localHash[:12], remoteHash[:12])
	return 0
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: erg <command> [args...]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  validate [dir|files...]   Validate erg v1 ticket files")
	fmt.Fprintln(os.Stderr, "  ready [dir] [--json]      Show tickets ready for work")
	fmt.Fprintln(os.Stderr, "  archive [dir] [--days N] [--execute]  Archive old closed tickets")
	fmt.Fprintln(os.Stderr, "  graph [dir] [--json]      Show ticket dependency DAG")
	fmt.Fprintln(os.Stderr, "  next-id [dir]             Print the next available ticket ID")
	fmt.Fprintln(os.Stderr, "  close <id|file> <reason> [dir]  Close a ticket atomically")
	fmt.Fprintln(os.Stderr, "  version                   Print sha256 of this binary")
	fmt.Fprintln(os.Stderr, "  update                    Fetch and replace binary from origin")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	rest := os.Args[2:]

	var exitCode int
	switch cmd {
	case "validate":
		exitCode = cmdValidate(rest)
	case "ready":
		exitCode = cmdReady(rest)
	case "archive":
		exitCode = cmdArchive(rest)
	case "graph":
		exitCode = cmdGraph(rest)
	case "next-id":
		exitCode = cmdNextID(rest)
	case "close":
		exitCode = cmdClose(rest)
	case "version":
		exitCode = cmdVersion(rest)
	case "update":
		exitCode = cmdUpdate(rest)
	case "-h", "--help", "help":
		printUsage()
		exitCode = 0
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		exitCode = 1
	}
	os.Exit(exitCode)
}
