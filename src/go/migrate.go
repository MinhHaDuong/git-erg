package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// summaryMigrate is the one-liner printed by printUsage via the commands registry.
const summaryMigrate = "Convert legacy Status: headers to Closed: form"

const helpMigrate = `## erg migrate [DIR]

Convert legacy headers to %erg 0.1 format.

Idempotent (safe to run repeatedly: already-migrated files are not modified twice). For every .erg file under DIR (default: tickets/) the migration
rules are:

  - 'Status: closed' (case-insensitive) → drop the line; append
    'Closed: migrated from Status: closed' to the preamble.
  - 'Status: open', 'Status: doing', or 'Status: pending' → drop the line;
    the ticket becomes not-closed (the correct new state).
  - 'Tags:' preamble line → rewrite the key to 'Tag:' (singular; the header is
    repeatable and singular names are the v1 convention). The value is preserved.
  - No legacy line → no-op.

After migration, erg validate will reject any remaining Status: or Tags: lines.

When DIR is named "tickets" (the canonical layout), also performs a one-time
project layout upgrade: removes tickets/tools/ and tickets/FORMAT.md if present,
renames archive/ to closed/ if archive/ exists and closed/ does not, then
refreshes init assets via cmdInit.

Does NOT commit. Exits 1 on archive/→closed/ filename collision (both directories are left untouched; the user must resolve manually). Exits 0 otherwise.
Review the diff with 'git diff tickets/' and commit manually.
`

// cmdMigrate implements `erg migrate [dir]`. See helpMigrate for the user-facing summary.
func cmdMigrate(args []string) int {
	var explicit string
	if len(args) > 0 {
		explicit = args[0]
	}
	dir, err := resolveDir(explicit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		return 1
	}

	migratedClosed := 0
	migratedOther := 0
	migratedTags := 0
	migratedMagic := 0
	migratedBlanks := 0
	alreadyClean := 0

	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".erg") {
			return nil
		}
		res, mErr := migrateFile(path)
		if mErr != nil {
			fmt.Fprintf(os.Stderr, "migrate: %s: %v\n", path, mErr)
			return nil
		}
		if !res.changed {
			alreadyClean++
			return nil
		}
		switch {
		case res.wasClosed:
			migratedClosed++
		case res.statusStripped:
			migratedOther++
		}
		if res.tagsRenamed {
			migratedTags++
		}
		if res.magicRewritten {
			migratedMagic++
		}
		if res.blanksSwept {
			migratedBlanks++
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate: walk error: %v\n", err)
		return 1
	}

	total := migratedClosed + migratedOther
	fmt.Printf("migrated: %d tickets (%d closed, %d open/doing/pending stripped)\n",
		total, migratedClosed, migratedOther)
	fmt.Printf("Tags: → Tag: rewrite: %d tickets\n", migratedTags)
	fmt.Printf("%%erg v1 → %%erg 0.1 rewrite: %d tickets\n", migratedMagic)
	fmt.Printf("interior header blank sweep: %d tickets\n", migratedBlanks)
	fmt.Printf("already clean: %d tickets\n", alreadyClean)

	// Layout migration: only run when dir is named "tickets" (canonical layout).
	if filepath.Base(dir) == "tickets" {
		if code := migrateLayout(dir); code != 0 {
			return code
		}
	}

	return 0
}

// migrateLayout performs a one-time project layout upgrade when the ticket
// directory is the canonical "tickets/". It removes legacy artifacts
// (tickets/tools/, tickets/FORMAT.md), renames archive/ to closed/ if
// applicable, copies the erg binary into tickets/, and refreshes init
// assets. Returns 0 on success or 1 on collision (archive/ vs closed/).
func migrateLayout(dir string) int {
	root := filepath.Dir(dir)
	toolsDir := filepath.Join(dir, "tools")
	formatMD := filepath.Join(dir, "FORMAT.md")
	archiveDir := filepath.Join(root, "archive")
	closedDir := filepath.Join(root, "closed")

	if _, err := os.Stat(toolsDir); err == nil {
		if err := os.RemoveAll(toolsDir); err != nil {
			fmt.Fprintf(os.Stderr, "migrate: remove tools/: %v\n", err)
		} else {
			fmt.Println("migrate: removed tickets/tools/")
		}
	}
	if _, err := os.Stat(formatMD); err == nil {
		if err := os.Remove(formatMD); err != nil {
			fmt.Fprintf(os.Stderr, "migrate: remove FORMAT.md: %v\n", err)
		} else {
			fmt.Println("migrate: removed tickets/FORMAT.md")
		}
	}
	_, archiveErr := os.Stat(archiveDir)
	_, closedErr := os.Stat(closedDir)
	if archiveErr == nil && closedErr != nil {
		if err := os.Rename(archiveDir, closedDir); err != nil {
			fmt.Fprintf(os.Stderr, "migrate: rename archive/→closed/: %v\n", err)
		} else {
			fmt.Println("migrate: renamed archive/ → closed/")
		}
	} else if archiveErr == nil && closedErr == nil {
		entries, err := os.ReadDir(archiveDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "migrate: read archive/: %v\n", err)
		} else {
			var conflicts []string
			for _, e := range entries {
				if _, err := os.Stat(filepath.Join(closedDir, e.Name())); err == nil {
					conflicts = append(conflicts, e.Name())
				}
			}
			if len(conflicts) > 0 {
				fmt.Fprintf(os.Stderr, "migrate: archive/→closed/ collision: %v — resolve manually\n", conflicts)
				return 1
			}
			for _, e := range entries {
				if err := os.Rename(filepath.Join(archiveDir, e.Name()), filepath.Join(closedDir, e.Name())); err != nil {
					fmt.Fprintf(os.Stderr, "migrate: move %s: %v\n", e.Name(), err)
				}
			}
			if err := os.Remove(archiveDir); err != nil {
				fmt.Fprintf(os.Stderr, "migrate: remove archive/: %v\n", err)
			} else {
				fmt.Printf("migrate: merged %d files archive/ → closed/, removed archive/\n", len(entries))
			}
		}
	}
	ergBin := filepath.Join(dir, "erg")
	_, statErr := os.Stat(ergBin)
	if os.IsNotExist(statErr) {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "migrate: cannot locate self: %v\n", err)
			return 1
		}
		data, err := os.ReadFile(exe)
		if err != nil {
			fmt.Fprintf(os.Stderr, "migrate: cannot read self: %v\n", err)
			return 1
		}
		if err := os.WriteFile(ergBin, data, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "migrate: cannot write tickets/erg: %v\n", err)
			return 1
		}
		if err := os.Chmod(ergBin, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "migrate: cannot chmod tickets/erg: %v\n", err)
			return 1
		}
		fmt.Println("migrate: copied binary → tickets/erg")
	}
	if code := cmdInit([]string{root}); code != 0 {
		fmt.Fprintln(os.Stderr, "migrate: init assets refresh failed")
	}
	return 0
}

// migrateResult summarizes what migrateFile rewrote in a single .erg file.
type migrateResult struct {
	changed        bool // file was rewritten on disk
	wasClosed      bool // at least one removed Status: line carried value "closed"
	statusStripped bool // at least one Status: line was removed (closed or open/doing/pending)
	tagsRenamed    bool // at least one preamble `Tags:` line was renamed to `Tag:`
	magicRewritten bool // legacy `%erg v1` magic line was rewritten to `%erg 0.1`
	blanksSwept    bool // at least one interior header blank line was removed
}

// migrateFile rewrites a single .erg file in place. The rewrite is preamble-bounded
// (everything before the first `--- log ---` separator); body code blocks that
// happen to contain `Status:` or `Tags:` are preserved verbatim.
func migrateFile(path string) (migrateResult, error) {
	var res migrateResult
	data, err := os.ReadFile(path)
	if err != nil {
		return res, err
	}
	original := string(data)
	// Preserve trailing-newline state when splitting/rejoining.
	hadTrailingNewline := strings.HasSuffix(original, "\n")
	lines := strings.Split(original, "\n")
	if hadTrailingNewline {
		lines = lines[:len(lines)-1]
	}

	// Rewrite legacy `%erg v1` magic line to `%erg 0.1`.
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == "%erg v1" {
			lines[i] = "%erg 0.1"
			res.magicRewritten = true
		}
		break // only check first non-empty line
	}

	// Bound preamble at the first `--- log ---` separator.
	logIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == separatorLog {
			logIdx = i
			break
		}
	}
	preambleEnd := len(lines)
	if logIdx >= 0 {
		preambleEnd = logIdx
	}

	out := make([]string, 0, len(lines))
	for i, line := range lines {
		if i < preambleEnd && isStatusHeaderLine(line) {
			res.statusStripped = true
			val := strings.TrimSpace(line[len("Status:"):])
			if strings.EqualFold(val, "closed") {
				res.wasClosed = true
			}
			continue
		}
		if i < preambleEnd && isTagsHeaderLine(line) {
			// Rewrite `Tags:` → `Tag:` preserving the value (and any
			// inline comment). Original casing of the value is kept.
			rewritten := "Tag:" + line[len("Tags:"):]
			out = append(out, rewritten)
			res.tagsRenamed = true
			continue
		}
		out = append(out, line)
	}

	if res.wasClosed {
		// After removing Status: lines, the preamble end shifts by however
		// many we dropped — recompute relative to the rewritten slice. Insert
		// the Closed header immediately after the last non-blank preamble line.
		newLogIdx := -1
		for i, line := range out {
			if strings.TrimSpace(line) == separatorLog {
				newLogIdx = i
				break
			}
		}
		insertAt := len(out)
		if newLogIdx >= 0 {
			insertAt = newLogIdx
		}
		for insertAt > 0 && strings.TrimSpace(out[insertAt-1]) == "" {
			insertAt--
		}
		header := "Closed: migrated from Status: closed"
		out = append(out[:insertAt], append([]string{header}, out[insertAt:]...)...)
	}

	rejoined := strings.Join(out, "\n")
	if hadTrailingNewline {
		rejoined += "\n"
	}

	// Sweep interior header blanks across the corpus (ticket 0141). Autofix
	// only fires on the next mutation; migrate is the one command that
	// proactively cleans every file, so hand-authored blanks do not linger.
	if hasInteriorHeaderBlank([]byte(rejoined)) {
		rejoined = string(collapseHeaderBlanks([]byte(rejoined)))
		res.blanksSwept = true
	}

	if rejoined == original {
		return res, nil
	}
	if err := os.WriteFile(path, []byte(rejoined), 0644); err != nil {
		return res, err
	}
	res.changed = true
	return res, nil
}

// isStatusHeaderLine reports whether a line begins with the literal
// `Status:` header key (case-insensitive on the key itself, since some
// tickets in the wild may carry quirky casing).
func isStatusHeaderLine(line string) bool {
	return hasHeaderKey(line, "Status:", true)
}

// isTagsHeaderLine reports whether a line begins with the literal
// `Tags:` header key (case-insensitive on the key itself, parallel to
// isStatusHeaderLine). Used to rewrite legacy `Tags:` preamble lines
// to the singular `Tag:` form.
func isTagsHeaderLine(line string) bool {
	return hasHeaderKey(line, "Tags:", true)
}

// hasStatusHeader scans dir for any .erg file containing a `Status:` line
// in the preamble. Used by `erg update` to decide whether to print
// migration guidance after a binary swap.
func hasStatusHeader(dir string) bool {
	stopWalk := fmt.Errorf("found")
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".erg") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if !strings.Contains(string(data), "Status:") {
			return nil
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == separatorLog {
				return nil
			}
			if isStatusHeaderLine(line) {
				return stopWalk
			}
		}
		return nil
	})
	return err == stopWalk
}
