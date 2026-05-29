package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

// readVersionInfo executes binaryPath version with a 2-second timeout and
// parses the revision: and built: lines from its stdout.
// Returns empty strings on any failure.
func readVersionInfo(binaryPath string) (revision, date, arch string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, "version")
	cmd.Env = append(os.Environ(), "ERG_VERSION_NO_DISCOVER=1")
	out, err := cmd.Output()
	if err != nil {
		return "", "", ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "revision:") {
			revision = strings.TrimSpace(strings.TrimPrefix(trimmed, "revision:"))
		} else if strings.HasPrefix(trimmed, "built:") {
			date = strings.TrimSpace(strings.TrimPrefix(trimmed, "built:"))
		} else if strings.HasPrefix(trimmed, "arch:") {
			arch = strings.TrimSpace(strings.TrimPrefix(trimmed, "arch:"))
		}
	}
	return revision, date, arch
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

// summaryVersion is the one-liner printed by printUsage via the commands registry.
const summaryVersion = "Print version, path, build date, and obsolescence info"

const helpVersion = `## erg version

Print self-diagnostic info and discover other erg binaries.

Prints the following fields for the running binary:

  - path:     resolved absolute path (symlinks followed).
  - sha256:   full 64-char hex SHA-256 of the binary file; recompute and verify
              with stock tools, e.g. ` + "`sha256sum tickets/erg`" + `.
  - built:    build date injected at compile time via -ldflags (or "[unknown]").
  - revision: VCS commit hash injected at compile time via -ldflags (if present).
  - arch:     GOOS/GOARCH of the running binary.

After printing the running binary info, ` + "`erg version`" + ` discovers other erg binaries
in well-known locations (./build/erg, ./tickets/erg, ~/.local/bin/erg, and PATH
entries), compares VCS revisions and build dates against each discovered copy, and
prints the update command for any outdated copy it finds.

Set ERG_VERSION_NO_DISCOVER=1 to suppress discovery (used internally by version
comparison to avoid recursion).
`

// cmdVersion implements `erg version`. See helpVersion for the user-facing summary.
func cmdVersion(_ []string) int {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "version: cannot resolve executable: %v\n", err)
		return 1
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	} else {
		fmt.Fprintf(os.Stderr, "version: symlink resolution failed, using raw path: %v\n", err)
	}

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
	fmt.Printf("  sha256:  %s\n", h)
	if buildDate != "" {
		fmt.Printf("  built:   %s\n", buildDate)
	} else {
		fmt.Printf("  built:   [unknown — no build metadata]\n")
	}
	if vcsRevision != "" {
		fmt.Printf("  revision: %s\n", vcsRevision)
	}
	fmt.Printf("  arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	if strings.HasSuffix(self, "tickets/erg") {
		fmt.Println("  verify:  sha256sum tickets/erg")
	}

	if os.Getenv("ERG_VERSION_NO_DISCOVER") != "" {
		return 0
	}

	// Discover other binaries
	type candidate struct {
		path string
		hint string
	}

	home, _ := os.UserHomeDir()
	candidates := []candidate{
		{"./build/erg", "make build"},
		{"./tickets/erg", "cp build/erg tickets/erg"},
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

		otherRevision, otherDate, otherArch := readVersionInfo(resolved)

		var label string
		if vcsRevision != "" && otherRevision == vcsRevision {
			// Same source commit — not outdated regardless of hash difference.
		} else if vcsRevision != "" && otherRevision != "" {
			if buildDate != "" && otherDate != "" && buildDate > otherDate {
				label = fmt.Sprintf("[outdated: run: %s]", c.hint)
			} else {
				label = "[different version]"
			}
		}

		entry := fmt.Sprintf("  %s\n    sha256:   %s", resolved, ch)
		if otherDate != "" {
			entry += fmt.Sprintf("\n    built:    %s", otherDate)
		}
		if otherRevision != "" {
			entry += fmt.Sprintf("\n    revision: %s", otherRevision)
		}
		if otherArch != "" {
			entry += fmt.Sprintf("\n    arch:     %s", otherArch)
		}
		if label != "" {
			entry += fmt.Sprintf("\n    %s", label)
		}

		others = append(others, entry)
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
