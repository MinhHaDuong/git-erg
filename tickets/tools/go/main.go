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
	"time"
)

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
