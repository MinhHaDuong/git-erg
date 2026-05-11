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

// updateURL is the default value for ERG_UPDATE_URL: the upstream binary
// location used by `erg update` when the environment variable is unset.
const updateURL = "https://raw.githubusercontent.com/MinhHaDuong/git-erg/main/tickets/erg"

// summaryUpdate is the one-liner printed by printUsage via the commands registry.
const summaryUpdate = "Fetch and replace binary from origin"

const helpUpdate = `## erg update

Fetch the upstream binary and replace this executable atomically.

Downloads the binary from ERG_UPDATE_URL (default: the main branch of the
upstream GitHub repo). If the downloaded hash matches the running binary,
prints "already up to date" and exits 0. Otherwise replaces the binary via
an atomic rename (write to .tmp, then rename over self).

Network and HTTP errors exit 0 so that 'erg update && erg validate' chains
do not fail in offline or isolated environments.

After a successful update, checks whether any .erg files in the ticket store
still carry legacy Status: headers. If found, prints explicit migration
guidance: 'erg migrate DIR', 'git diff tickets/', 'git commit'. The update
command never mutates ticket files itself — migration is a separate, reviewable step.
`

// cmdUpdate implements `erg update`. See helpUpdate for the user-facing summary.
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

	client := &http.Client{Timeout: 120 * time.Second}
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
	if len(body) == 0 {
		fmt.Fprintf(os.Stderr, "update: server returned empty body\n")
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

	// Detect tickets still carrying `Status:` headers and emit a hint.
	// Migration is explicit: the user runs `erg migrate`, reviews the diff,
	// and commits separately. erg update never mutates ticket files.
	ticketDir := os.Getenv("ERG_TICKET_DIR")
	if ticketDir == "" {
		d, findErr := findTicketsDir()
		if findErr != nil {
			return 0
		}
		ticketDir = d
	}
	if info, err := os.Stat(ticketDir); err == nil && info.IsDir() && hasStatusHeader(ticketDir) {
		fmt.Printf("erg: detected Status: headers in %s — run:\n", ticketDir)
		fmt.Printf("  erg migrate %s\n", ticketDir)
		fmt.Println("  git diff tickets/")
		fmt.Println("  git commit -m 'chore: migrate to Closed: header'")
	}
	return 0
}
