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

// migrateAssetPaths is the subset of assets that `erg migrate` refreshes during
// its layout sweep. It deliberately excludes tickets/.ergrc: configuration
// delivery is `erg init`'s job (the dpkg 3-state compare that preserves local
// edits). AGENTS.md keeps the charter's force-overwrite behaviour here because
// agent operating instructions must track the binary (ticket 0224).
var migrateAssetPaths = []string{
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

It also writes tickets/.erg-assets, a provenance manifest recording this
binary's rev/date and the SHA-256 of each embedded asset. The manifest is
committable durable state (not gitignored) and is invisible to erg check, so
it never trips the pre-commit hook. It is deterministic: the same binary and
assets always produce byte-identical content.

The format specification and setup guide are available on demand via
erg spec and erg integration respectively.

Requires tickets/erg (the binary) to already exist in the project; the command
refuses if it is absent. This requirement ensures that agents do not accidentally
initialize an empty directory that was never meant to be a ticket store.

Each asset is compared against the embedded version with a dpkg-style 3-state
rule. Byte-identical files are left unchanged. A differing file that still
matches the .erg-assets stamp (or, with no stamp, a known shipped hash) is a
clean upgrade -- erg never touched it, so it is overwritten and a
"git restore -- <path>" hint is printed. A differing file that matches neither
is a local edit: it is preserved and the command exits 2 (local edits are never
overwritten without --force).

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

Canonical keep-current sequence: 'erg update && erg init'. erg update replaces the
binary; erg init delivers embedded-asset changes and refreshes the default label
vocabulary. The default vocabulary is frozen-by-copy into .ergrc at init time -- a
new default added later to the binary is shadowed by the existing file and never takes
effect until erg init overwrites the file (clean upgrade) or the user opts in with
--force (local edit). erg update alone cannot un-shadow a frozen vocabulary.

Exit codes: 0 success; 1 a hard error (bad flag, missing binary, write
failure); 2 local edits were preserved and skipped (run with --force to
overwrite). See "Exit codes" in erg --help --all.
`

// installAssets unpacks the embedded bootstrap assets under root, returning
// counts of how many files were newly created, refreshed (overwritten with
// different content), skipped (differed from embedded but refuseDiverged was
// set), or left unchanged (byte-identical to the embedded copy). Shared by
// `erg init` and by `erg migrate`'s layout sweep. The caller supplies the exact
// asset list via paths: `erg init` passes initAssetPaths (both assets); `erg
// migrate` passes migrateAssetPaths (AGENTS.md only -- .ergrc is configuration,
// delivered by init, ticket 0224). Error messages are unwrapped -- no "init:"
// prefix -- so each caller can label them with its own command name.
//
// When refuseDiverged is true, files that differ from the embedded asset go
// through the dpkg 3-state compare: a clean upgrade (on-disk matches the
// .erg-assets stamp, or a known shipped hash when no stamp) is overwritten
// (refresh); a local edit (matches neither) is preserved with a message on
// stderr and counted as skipped. When false (erg init --force, erg migrate),
// differing files are overwritten unconditionally (refresh).
//
// When dryRun is true, no directory is created, no file is written, and the
// orphan sweep is not performed; instead a preview line is printed for each
// asset describing the action that would be taken. The returned counts are the
// same as a real run would produce.
func installAssets(root string, paths []string, refuseDiverged, dryRun bool) (created, refreshed, skipped, unchanged int, err error) {
	// The .erg-assets stamp from a previous init (nil if absent or malformed):
	// name -> recorded SHA-256. Read once; the dpkg compare consults it per asset.
	stamps := readManifest(root)
	for _, rel := range paths {
		content, ok := bootstrapAsset(rel)
		if !ok {
			return created, refreshed, skipped, unchanged, fmt.Errorf("missing embedded asset: %s", rel)
		}
		name := strings.TrimPrefix(rel, "tickets/")
		target := filepath.Join(root, filepath.FromSlash(rel))
		existing, readErr := os.ReadFile(target)
		exists := readErr == nil

		// Row 1: on-disk byte-identical to embedded -> nothing to do.
		// Loud per-file output names this skip outcome too (criterion 5:
		// each action prints its file + action), matching refresh/preserve.
		if exists && string(existing) == content {
			unchanged++
			if dryRun {
				fmt.Printf("  unchanged  %s\n", rel)
			} else {
				fmt.Fprintf(os.Stderr, "init: %s unchanged\n", rel)
			}
			continue
		}

		// Divergent (or absent). Decide overwrite vs preserve.
		// - !refuseDiverged (erg init --force, erg migrate): overwrite
		//   unconditionally -- exempt from the dpkg prompt.
		// - refuseDiverged (erg init default): dpkg 3-state compare. A clean
		//   upgrade (on-disk == stamp, or a known shipped hash when no stamp)
		//   is overwritten silently; a local edit is preserved (exit 2).
		preserve := false
		if exists && refuseDiverged {
			diskHash := sha256hex(existing)
			if !isCleanUpgrade(diskHash, stamps[name], knownAssetHashes(rel)) {
				preserve = true
			}
		}

		if preserve {
			skipped++
			if dryRun {
				fmt.Printf("  would preserve (local edits)  %s\n", rel)
			} else {
				fmt.Fprintf(os.Stderr, "init: %s has local edits -- preserving (run with --force to overwrite)\n", rel)
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
		// Loud output: name each overwrite and give a reversibility hint, so a
		// refresh (even a safe clean upgrade) is never silent.
		if exists {
			fmt.Fprintf(os.Stderr, "init: refreshed %s (git restore -- %s to undo)\n", rel, rel)
		}
	}
	// Record provenance (ticket 0210): a deterministic manifest of the embedded
	// asset hashes for this binary. Written by both erg init and erg migrate
	// (the two callers of installAssets). Skipped in dry-run.
	if err := writeManifest(root, dryRun); err != nil {
		return created, refreshed, skipped, unchanged, fmt.Errorf("cannot write provenance manifest: %w", err)
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
	created, refreshed, skipped, unchanged, err := installAssets(root, initAssetPaths, refuseDiverged, dryRun)
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
