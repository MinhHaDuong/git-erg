package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// folderClosure warns about tickets whose open/closed state conflicts with
// their directory placement.
func folderClosure(tickets []Erg) []string {
	var warnings []string
	for i := range tickets {
		t := &tickets[i]
		inClosedDir := pathIsClosed(filepath.Dir(t.Path))
		hasClosed := false
		if vs, ok := t.Headers["Closed"]; ok {
			for _, v := range vs {
				if strings.TrimSpace(v) != "" {
					hasClosed = true
					break
				}
			}
		}

		if inClosedDir && !hasClosed {
			warnings = append(warnings, fmt.Sprintf(
				"WARNING %s: open ticket in closed/ directory", t.Filename()))
		}
		if !inClosedDir && hasClosed {
			warnings = append(warnings, fmt.Sprintf(
				"WARNING %s: closed ticket not in closed/ directory", t.Filename()))
		}
	}
	return warnings
}

// cmdCheck implements `erg check [dir]` — corpus-level validation.
func cmdCheck(args []string) int {
	dir := "tickets/"
	if len(args) > 0 {
		dir = args[0]
	}

	info, err := os.Stat(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s: %v\n", dir, err)
		return 1
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "ERROR: %s is not a directory; use 'erg validate' for individual files\n", dir)
		return 1
	}

	tickets := loadErgs(dir)
	if len(tickets) == 0 {
		fmt.Println("No .erg files found.")
		return 0
	}

	errors := validateAll(tickets)
	warnings := folderClosure(tickets)

	hasErrors := len(errors) > 0
	if hasErrors {
		fmt.Printf("ERG CHECK FAILED (%d error(s)):\n", len(errors))
		for _, e := range errors {
			fmt.Printf("  VIOLATION %s\n", e)
		}
	}
	for _, w := range warnings {
		fmt.Printf("  %s\n", w)
	}

	if hasErrors {
		return 1
	}

	fmt.Printf("ERG CHECK: PASS (%d tickets", len(tickets))
	if len(warnings) > 0 {
		fmt.Printf(", %d warning(s)", len(warnings))
	}
	fmt.Println(")")
	return 0
}
