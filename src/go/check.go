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
		hasClosed := t.Closed != ""

		if inClosedDir && !hasClosed {
			warnings = append(warnings, fmt.Sprintf(
				"WARN %s: open ticket in closed/ directory", t.Filename()))
		}
		if !inClosedDir && hasClosed {
			warnings = append(warnings, fmt.Sprintf(
				"WARN %s: closed ticket not in closed/ directory", t.Filename()))
		}
	}
	return warnings
}

// staleBlockedBy warns about open tickets whose Blocked-by local refs
// point to tickets that are already closed.
func staleBlockedBy(tickets []Erg) []string {
	closedIDs := make(map[string]bool)
	for i := range tickets {
		id := tickets[i].FilenameID()
		if id != "" && tickets[i].IsClosed() {
			closedIDs[id] = true
		}
	}

	var warnings []string
	for i := range tickets {
		t := &tickets[i]
		if t.IsClosed() {
			continue
		}
		for _, ref := range t.BlockedBys {
			if ref.Kind != RefLocal {
				continue
			}
			if closedIDs[ref.ID] {
				warnings = append(warnings, fmt.Sprintf(
					"WARN %s: Blocked-by %s is already closed — remove the stale Blocked-by line from %s",
					t.Filename(), ref.ID, t.Filename()))
			}
		}
	}
	return warnings
}

// strayGoSource warns when Go source files (*.go, go.mod, go.sum) are found
// in dir itself or in the legacy dir/tools/go/ subdirectory.
func strayGoSource(dir string) []string {
	toScan := []string{dir, filepath.Join(dir, "tools", "go")}
	for _, d := range toScan {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			n := e.Name()
			if strings.HasSuffix(n, ".go") || n == "go.mod" || n == "go.sum" {
				return []string{"WARN: Go source files found in " + d + " — only the binary is needed; remove *.go, go.mod, go.sum"}
			}
		}
	}
	return nil
}

// summaryCheck is the one-liner printed by printUsage via the commands registry.
const summaryCheck = "Corpus-level checks (duplicate IDs, cycles, refs)"

const helpCheck = `## erg check [DIR]

Corpus-level integrity checks across the full ticket store.

Unlike erg validate (which checks individual files), check loads all .erg files
under DIR recursively and verifies invariants that require a global view:

  - No duplicate ticket IDs across the corpus.
  - All Blocked-by local refs point to tickets that exist in the corpus.
  - No dependency cycles among Blocked-by edges.
  - All per-ticket format rules (delegates to validateCorpus, which folds in parser-emitted errors).

Additionally emits warnings (non-fatal) for:

  - Folder/header mismatch: open ticket in closed/ or closed ticket not in closed/.
  - Stray Go source files (*.go, go.mod, go.sum) inside the ticket store directory.

Exit codes: 0 on pass (warnings are printed but do not affect exit code), 1 on any violation.
`

// cmdCheck implements `erg check [dir]`. See helpCheck for the user-facing summary.
func cmdCheck(args []string) int {
	var dir string
	if len(args) > 0 {
		dir = args[0]
	} else {
		var err error
		dir, err = findTicketsDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
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

	tickets, parseErrs := loadErgs(dir)
	if len(tickets) == 0 {
		fmt.Println("No .erg files found.")
		return 0
	}

	cfg, cfgErr := loadConfig(dir)
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "check: cannot read .ergrc: %v\n", cfgErr)
		return 1
	}

	errors := validateCorpus(tickets, parseErrs, cfg)
	warnings := folderClosure(tickets)
	warnings = append(warnings, staleBlockedBy(tickets)...)
	warnings = append(warnings, strayGoSource(dir)...)

	hasErrors := len(errors) > 0
	if hasErrors {
		errWord := "errors"
		if len(errors) == 1 {
			errWord = "error"
		}
		fmt.Printf("ERG CHECK FAILED (%d %s):\n", len(errors), errWord)
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
		warnWord := "warnings"
		if len(warnings) == 1 {
			warnWord = "warning"
		}
		fmt.Printf(", %d %s", len(warnings), warnWord)
	}
	fmt.Println(")")
	return 0
}
