package main

import (
	"fmt"
	"os"
	"path/filepath"
)

var initAssetPaths = []string{
	"tickets/README.md",
	"tickets/spec-erg-v1.md",
	"tickets/integration.md",
}

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
