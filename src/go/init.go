package main

import (
	"fmt"
	"os"
	"path/filepath"
)

var initAssetPaths = []string{
	"tickets/.ergrc",
	"tickets/AGENTS.md",
	"tickets/spec-erg-v1.md",
	"tickets/integration.md",
}

// summaryInit is the one-liner printed by printUsage via the commands registry.
const summaryInit = "Unpack .ergrc, AGENTS.md, spec-erg-v1.md, integration.md into tickets/"

const helpInit = `## erg init [DIR]

Unpack embedded bootstrap assets into the project.

Writes (or refreshes) four files relative to DIR (default: current directory):

  - tickets/.ergrc — project configuration (tag vocabulary, update URL).
  - tickets/AGENTS.md — agent operating instructions for the ticket workflow.
  - tickets/spec-erg-v1.md — the %erg 0.1 format specification.
  - tickets/integration.md — setup guide for the pre-commit hook and CI integration.

Requires tickets/erg (the binary) to already exist in the project; the command
refuses if it is absent. This requirement ensures that agents do not accidentally
initialize an empty directory that was never meant to be a ticket store.

Each asset is compared byte-for-byte with the embedded version; unchanged files
are skipped and counted separately from newly created or refreshed files.
`

// cmdInit implements `erg init [dir]`. See helpInit for the user-facing summary.
func cmdInit(args []string) int {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	binaryPath := filepath.Join(root, "tickets", "erg")
	if _, err := os.Stat(binaryPath); err != nil {
		fmt.Fprintf(os.Stderr, "init: binary not found at %s\n", binaryPath)
		fmt.Fprintln(os.Stderr, "Place the erg binary in tickets/ before running init.")
		return 1
	}

	created := 0
	refreshed := 0
	unchanged := 0

	for _, rel := range initAssetPaths {
		content, ok := bootstrapAsset(rel)
		if !ok {
			fmt.Fprintf(os.Stderr, "init: missing embedded asset: %s\n", rel)
			return 1
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "init: cannot create directory for %s: %v\n", rel, err)
			return 1
		}
		existing, err := os.ReadFile(target)
		exists := err == nil
		if exists && string(existing) == content {
			unchanged++
			continue
		}
		if err := os.WriteFile(target, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "init: cannot write %s: %v\n", rel, err)
			return 1
		}
		if exists {
			refreshed++
		} else {
			created++
		}
	}

	fmt.Printf("init: %d created, %d refreshed, %d unchanged\n", created, refreshed, unchanged)
	fmt.Println("Next: follow tickets/integration.md to set up the pre-commit hook and agent instructions.")
	return 0
}
