package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// buildDate is set at compile time via -ldflags "-X main.buildDate=..."
var buildDate string

// vcsRevision is set at compile time via -ldflags "-X main.vcsRevision=..."
var vcsRevision string

const updateURL = "https://raw.githubusercontent.com/MinhHaDuong/git-erg/main/tickets/tools/go/erg"

// readVersionInfo executes binaryPath version with a 2-second timeout and
// parses the revision: and built: lines from its stdout.
// Returns empty strings on any failure.
func readVersionInfo(binaryPath string) (revision, date string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, "version")
	out, err := cmd.Output()
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "revision:") {
			revision = strings.TrimSpace(strings.TrimPrefix(trimmed, "revision:"))
		} else if strings.HasPrefix(trimmed, "built:") {
			date = strings.TrimSpace(strings.TrimPrefix(trimmed, "built:"))
		}
	}
	return revision, date
}

// selfHash returns the hex-encoded sha256 of the file at path.
func selfHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// cmdVersion implements `erg version`: print self-diagnostic info and
// discover other erg binaries, marking outdated ones.
func cmdVersion(_ []string) int {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "version: cannot resolve executable: %v\n", err)
		return 1
	}
	self, _ = filepath.EvalSymlinks(self)

	selfInfo, err := os.Stat(self)
	if err != nil {
		fmt.Fprintf(os.Stderr, "version: cannot stat executable: %v\n", err)
		return 1
	}

	h, err := selfHash(self)
	if err != nil {
		fmt.Fprintf(os.Stderr, "version: cannot hash self: %v\n", err)
		return 1
	}

	// Print running binary info
	fmt.Println("erg version")
	fmt.Printf("  path:    %s\n", self)
	fmt.Printf("  hash:    %s\n", h[:12])
	if buildDate != "" {
		fmt.Printf("  built:   %s\n", buildDate)
	} else {
		fmt.Printf("  built:   [unknown — no build metadata]\n")
	}
	if vcsRevision != "" {
		fmt.Printf("  revision: %s\n", vcsRevision)
	}
	fmt.Printf("  arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)

	// Discover other binaries
	type candidate struct {
		path string
		hint string
	}

	home, _ := os.UserHomeDir()
	candidates := []candidate{
		{"./build/erg", "make build"},
	}
	if home != "" {
		candidates = append(candidates, candidate{
			filepath.Join(home, ".local", "bin", "erg"),
			"make install-erg-binary",
		})
	}

	// Walk PATH for additional erg entries
	pathDirs := filepath.SplitList(os.Getenv("PATH"))
	for _, dir := range pathDirs {
		p := filepath.Join(dir, "erg")
		candidates = append(candidates, candidate{p, "cp build/erg " + p})
	}

	// Deduplicate and print
	seen := make(map[string]bool)
	var others []string

	for _, c := range candidates {
		abs, err := filepath.Abs(c.path)
		if err != nil {
			continue
		}
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			// File doesn't exist or can't be resolved — skip silently
			continue
		}

		if seen[resolved] {
			continue
		}
		seen[resolved] = true

		// Skip if this is the running binary
		info, err := os.Stat(resolved)
		if err != nil {
			continue
		}
		if os.SameFile(selfInfo, info) {
			continue
		}

		ch, err := selfHash(resolved)
		if err != nil {
			continue
		}

		line := fmt.Sprintf("  %s  hash=%s", resolved, ch[:12])

		// Determine outdated label using VCS revision when available.
		otherRevision, otherDate := readVersionInfo(resolved)
		if vcsRevision != "" && otherRevision == vcsRevision {
			// Same source commit — not outdated regardless of hash difference.
		} else if vcsRevision != "" && otherRevision != "" {
			// Both have revisions but they differ — compare build dates.
			if buildDate != "" && otherDate != "" && buildDate > otherDate {
				line += fmt.Sprintf("  [outdated: run: %s]", c.hint)
			} else {
				line += "  [different version]"
			}
		} else {
			// Cannot determine — fall back to showing hash only, no label.
		}

		others = append(others, line)
	}

	if len(others) > 0 {
		fmt.Println()
		fmt.Println("other erg binaries:")
		for _, l := range others {
			fmt.Println(l)
		}
	}

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
