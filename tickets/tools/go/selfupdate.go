package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const updateURL = "https://raw.githubusercontent.com/MinhHaDuong/git-erg/main/tickets/tools/go/erg"

// selfHash returns the hex-encoded sha256 of the file at path.
func selfHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// cmdVersion implements `erg version`: print the sha256 of this binary.
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

// cmdUpdate implements `erg update`: fetch the upstream binary and replace
// this executable atomically when the hashes differ. Network errors exit 0
// so callers can chain `erg update && erg validate` without flaking offline.
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

	if hasStaleAssets(".") {
		fmt.Println("erg: repo assets may be behind this binary — run:")
		fmt.Println("  erg init .")
	}

	// Detect tickets still carrying `Status:` headers and emit a hint.
	// Migration is explicit: the user runs `erg migrate`, reviews the diff,
	// and commits separately. erg update never mutates ticket files.
	ticketDir := os.Getenv("ERG_TICKET_DIR")
	if ticketDir == "" {
		ticketDir = "tickets"
	}
	if info, err := os.Stat(ticketDir); err == nil && info.IsDir() && hasStatusHeader(ticketDir) {
		fmt.Printf("erg: detected Status: headers in %s — run:\n", ticketDir)
		fmt.Printf("  erg migrate %s\n", ticketDir)
		fmt.Println("  git diff tickets/")
		fmt.Println("  git commit -m 'chore: migrate to Closed: header'")
	}
	return 0
}

// hasStaleAssets reports whether any managed bootstrap asset on disk
// differs from the copy embedded in this binary.
func hasStaleAssets(root string) bool {
	for _, rel := range managedAssetPaths {
		expected, ok := bootstrapAsset(rel)
		if !ok {
			continue
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		data, err := os.ReadFile(target)
		if err != nil {
			return true // file missing or unreadable
		}
		if string(data) != expected {
			return true // content differs
		}
	}
	return false
}
