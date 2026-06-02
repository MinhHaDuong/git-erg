package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var initAssetPaths = []string{
	"tickets/.ergrc",
	"tickets/AGENTS.md",
}

// orphanAssetPaths lists assets that older erg versions deposited during init
// but are now served on demand via erg spec / erg integration. If a file at
// one of these paths matches the current embedded content exactly, init
// removes it as an orphan.
var orphanAssetPaths = []string{
	"tickets/spec-erg-v1.md",
	"tickets/integration.md",
}

const summaryInit = "Unpack .ergrc and AGENTS.md into tickets/"

const helpInit = `## erg init [DIR] [-n|--dry-run] [--force]

Unpack embedded bootstrap assets into the project.

Writes two files relative to DIR (default: current directory):

  - tickets/.ergrc -- project configuration (label vocabulary, update remote).
  - tickets/AGENTS.md -- agent operating instructions for the ticket workflow.

The format specification and setup guide are available on demand via
erg spec and erg integration respectively.

Requires tickets/erg (the binary) to already exist in the project; the command
refuses if it is absent. This requirement ensures that agents do not accidentally
initialize an empty directory that was never meant to be a ticket store.

Each asset is compared byte-for-byte with the embedded version; unchanged files
are skipped and counted separately from newly created files. If an existing file
differs from the embedded version (indicating local edits), it is skipped with a
message on stderr and the command exits 2 (local edits are never overwritten).

Flags:

  -n, --dry-run   Preview what init would create, refresh, skip, or leave
                  unchanged without writing or removing any file.
  --force         Overwrite files that differ from the embedded version
                  instead of skipping them. Use with care: local edits are
                  replaced.

If tickets/spec-erg-v1.md or tickets/integration.md exist from a previous init
and match the current embedded content, they are removed as orphaned assets.
Files that have been edited locally are preserved.

After a successful run (not in dry-run), init chains a read-only corpus check
and prints any warnings, but its exit code reflects the init outcome only --
the chained warnings never change it.

Exit codes: 0 success; 1 a hard error (bad flag, missing binary, write
failure); 2 local edits were preserved and skipped (run with --force to
overwrite). See "Exit codes" in erg --help --all.
`

// installAssets unpacks the embedded bootstrap assets under root, returning
// counts of how many files were newly created, refreshed (overwritten with
// different content), skipped (differed from embedded but refuseDiverged was
// set), or left unchanged (byte-identical to the embedded copy). Shared by
// `erg init` and by `erg migrate`'s layout sweep. Error messages are
// unwrapped -- no "init:" prefix -- so each caller can label them with its own
// command name.
//
// When refuseDiverged is true, files that differ from the embedded asset are
// skipped with a message on stderr instead of being overwritten; the skipped
// count is incremented. When false, differing files are overwritten (refresh).
//
// When dryRun is true, no directory is created, no file is written, and the
// orphan sweep is not performed; instead a preview line is printed for each
// asset describing the action that would be taken. The returned counts are the
// same as a real run would produce.
func installAssets(root string, refuseDiverged, dryRun bool) (created, refreshed, skipped, unchanged int, err error) {
	for _, rel := range initAssetPaths {
		content, ok := bootstrapAsset(rel)
		if !ok {
			return created, refreshed, skipped, unchanged, fmt.Errorf("missing embedded asset: %s", rel)
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		existing, readErr := os.ReadFile(target)
		exists := readErr == nil
		if exists && string(existing) == content {
			unchanged++
			if dryRun {
				fmt.Printf("  unchanged  %s\n", rel)
			}
			continue
		}
		if exists && refuseDiverged {
			skipped++
			if dryRun {
				fmt.Printf("  would skip (local edits)  %s\n", rel)
			} else {
				fmt.Fprintf(os.Stderr, "init: %s has local edits -- skipping\n", rel)
			}
			continue
		}
		if exists {
			refreshed++
		} else {
			created++
		}
		if dryRun {
			verb := "would create "
			if exists {
				verb = "would refresh"
			}
			fmt.Printf("  %s  %s\n", verb, rel)
			continue
		}
		if mkErr := os.MkdirAll(filepath.Dir(target), 0755); mkErr != nil {
			return created, refreshed, skipped, unchanged, fmt.Errorf("cannot create directory for %s: %w", rel, mkErr)
		}
		if wErr := os.WriteFile(target, []byte(content), 0644); wErr != nil {
			return created, refreshed, skipped, unchanged, fmt.Errorf("cannot write %s: %w", rel, wErr)
		}
	}
	return created, refreshed, skipped, unchanged, nil
}

// cmdInit implements `erg init [dir] [-n|--dry-run] [--force]`. See helpInit
// for the user-facing summary. Exit codes: 0 success; 1 hard error; 2 local
// edits skipped.
func cmdInit(args []string) int {
	var positional []string
	dryRun := false
	force := false
	for _, a := range args {
		switch a {
		case "-n", "--dry-run":
			dryRun = true
		case "--force":
			force = true
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(os.Stderr, "init: unknown flag %q\nUsage: erg init [DIR] [-n|--dry-run] [--force]\n", a)
				return 1
			}
			positional = append(positional, a)
		}
	}
	root := "."
	if len(positional) > 0 {
		root = positional[0]
	}

	binaryPath := filepath.Join(root, "tickets", "erg")
	if _, err := os.Stat(binaryPath); err != nil {
		fmt.Fprintf(os.Stderr, "init: binary not found at %s\n", binaryPath)
		fmt.Fprintln(os.Stderr, "Place the erg binary in tickets/ before running init.")
		return 1
	}

	// --force overwrites divergent files; without it, they are preserved.
	refuseDiverged := !force
	created, refreshed, skipped, unchanged, err := installAssets(root, refuseDiverged, dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init: %v\n", err)
		return 1
	}

	cleanOrphanAssets(root, dryRun)

	if dryRun {
		fmt.Printf("init (dry-run): %d to create, %d to refresh, %d to skip (local edits), %d unchanged\n", created, refreshed, skipped, unchanged)
		if skipped > 0 {
			return 2
		}
		return 0
	}

	fmt.Printf("init: %d created, %d refreshed, %d skipped (local edits), %d unchanged\n", created, refreshed, skipped, unchanged)
	if skipped > 0 {
		return 2
	}

	// Chain a read-only corpus check: print warnings, but the exit code
	// reflects init only -- warnings never change it.
	ticketsDir := filepath.Join(root, "tickets")
	chained, _ := loadErgs(ticketsDir)
	for _, w := range corpusWarnings(chained, ticketsDir) {
		fmt.Fprintln(os.Stderr, w)
	}

	fmt.Println("Next: erg install --hooks to set up the pre-commit hook.")
	return 0
}

// cleanOrphanAssets removes assets that older erg versions deposited but are
// now served on demand (erg spec / erg integration), only when they match the
// current embedded content. Divergent files (possible user data) are always
// preserved. In dryRun mode it prints what it would remove without removing.
func cleanOrphanAssets(root string, dryRun bool) {
	for _, rel := range orphanAssetPaths {
		embedded, ok := bootstrapAsset(rel)
		if !ok {
			continue
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		existing, err := os.ReadFile(target)
		if err != nil {
			continue
		}
		if string(existing) == embedded {
			if dryRun {
				fmt.Printf("  would remove orphaned asset %s (now: erg %s)\n", rel, commandForOrphan(rel))
			} else {
				os.Remove(target)
				fmt.Fprintf(os.Stderr, "init: removed orphaned asset %s (now: erg %s)\n", rel, commandForOrphan(rel))
			}
		}
	}
}

func commandForOrphan(rel string) string {
	if strings.Contains(rel, "spec") {
		return "spec"
	}
	return "integration"
}
