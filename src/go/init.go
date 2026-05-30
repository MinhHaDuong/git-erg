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

  - tickets/.ergrc — project configuration (label vocabulary, update remote).
  - tickets/AGENTS.md — agent operating instructions for the ticket workflow.
  - tickets/spec-erg-v1.md — the %erg 0.1 format specification.
  - tickets/integration.md — setup guide for the pre-commit hook and CI integration.

Requires tickets/erg (the binary) to already exist in the project; the command
refuses if it is absent. This requirement ensures that agents do not accidentally
initialize an empty directory that was never meant to be a ticket store.

Each asset is compared byte-for-byte with the embedded version; unchanged files
are skipped and counted separately from newly created or refreshed files.
`

// installAssets unpacks the embedded bootstrap assets under root, returning
// counts of how many files were newly created, refreshed (overwritten with
// different content), or left unchanged (byte-identical to the embedded
// copy). Shared by `erg init` and by `erg migrate`'s layout sweep. Error
// messages are unwrapped — no "init:" prefix — so each caller can label
// them with its own command name.
func installAssets(root string) (created, refreshed, unchanged int, err error) {
	for _, rel := range initAssetPaths {
		content, ok := bootstrapAsset(rel)
		if !ok {
			return created, refreshed, unchanged, fmt.Errorf("missing embedded asset: %s", rel)
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		if mkErr := os.MkdirAll(filepath.Dir(target), 0755); mkErr != nil {
			return created, refreshed, unchanged, fmt.Errorf("cannot create directory for %s: %w", rel, mkErr)
		}
		existing, readErr := os.ReadFile(target)
		exists := readErr == nil
		if exists && string(existing) == content {
			unchanged++
			continue
		}
		if wErr := os.WriteFile(target, []byte(content), 0644); wErr != nil {
			return created, refreshed, unchanged, fmt.Errorf("cannot write %s: %w", rel, wErr)
		}
		if exists {
			refreshed++
		} else {
			created++
		}
	}
	return created, refreshed, unchanged, nil
}

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

	created, refreshed, unchanged, err := installAssets(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init: %v\n", err)
		return 1
	}

	fmt.Printf("init: %d created, %d refreshed, %d unchanged\n", created, refreshed, unchanged)
	fmt.Println("Next: follow tickets/integration.md to set up the pre-commit hook and agent instructions.")
	return 0
}
