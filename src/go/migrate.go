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

  - 'Status: closed' (case-insensitive) -> drop the line; append
    'Closed: migrated from Status: closed' to the preamble.
  - 'Status: open', 'Status: doing', or 'Status: pending' -> drop the line;
    the ticket becomes not-closed (the correct new state).
  - 'Tag:' (or legacy 'Tags:') preamble line -> rewrite the key to 'Label:'. The
    value is preserved; legacy 'Tags:' converges to 'Label:' in a single run.
  - '.ergrc' '[tags]' section header -> rewritten to '[labels]'.
  - Legacy '%erg v1' magic line -> rewritten to '%erg 0.1'.
  - Interior blank lines inside the header block -> swept (ticket 0141:
    accept on read, autofix on write). The first blank line still terminates
    the header block; only blanks between header lines are removed.
  - Log continuation lines: any non-blank line in the log section that does
    not start with a YYYY-MM-DD timestamp is joined (single space, stripped)
    onto the preceding log entry. Blank lines between an entry and its
    continuation content are dropped. Content before the first timestamped
    entry is untouched.
  - Date-only log stamps: a leading 'YYYY-MM-DD ' (date, no T separator) is
    rewritten to 'YYYY-MM-DDT00:00Z '.
  - No legacy line and no interior blanks -> no-op.

After migration, erg validate will reject any remaining Status:, Tags:, or Tag: lines,
and folds legacy wrapped log details plus date-only log stamps so a migrated store
passes validation (orphan content before the first log entry is left for validate to flag).

When DIR is named "tickets" (the canonical layout), also performs a one-time
project layout upgrade: removes tickets/tools/ and tickets/FORMAT.md if present,
renames archive/ to closed/ if archive/ exists and closed/ does not, refreshes
init assets (overwrites diverged files without prompting), and rewrites .git/hooks/pre-commit if it references
the legacy tickets/tools/go/erg path or the legacy 'validate tickets/' CLI
form. The hook rewrite is content-based and idempotent; hooks without legacy
patterns are left untouched.

Does NOT commit. Exits 1 on archive/->closed/ filename collision (both directories are left untouched; the user must resolve manually). Exits 0 otherwise.
Review the diff with 'git diff tickets/' and commit manually.
`

// cmdMigrate implements `erg migrate [dir]`. See helpMigrate for the user-facing summary.
func cmdMigrate(args []string) int {
	var positional []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			fmt.Fprintf(os.Stderr, "migrate: unknown flag %q\nUsage: erg migrate [DIR]\n", a)
			return 1
		}
		positional = append(positional, a)
	}
	var explicit string
	if len(positional) > 0 {
		explicit = positional[0]
	}
	dir, err := resolveDir(explicit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		return 1
	}

	migratedClosed := 0
	migratedOther := 0
	migratedLabels := 0
	migratedMagic := 0
	migratedBlanks := 0
	migratedFolded := 0
	migratedStamped := 0
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
		if res.labelsRewritten {
			migratedLabels++
		}
		if res.magicRewritten {
			migratedMagic++
		}
		if res.blanksSwept {
			migratedBlanks++
		}
		if res.folded {
			migratedFolded++
		}
		if res.stamped {
			migratedStamped++
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
	fmt.Printf("Tag: \u2192 Label: rewrite: %d tickets\n", migratedLabels)
	fmt.Printf("%%erg v1 \u2192 %%erg 0.1 rewrite: %d tickets\n", migratedMagic)
	fmt.Printf("interior header blank sweep: %d tickets\n", migratedBlanks)
	fmt.Printf("log continuation fold: %d tickets\n", migratedFolded)
	fmt.Printf("date-only stamp normalise: %d tickets\n", migratedStamped)
	fmt.Printf("already clean: %d tickets\n", alreadyClean)

	// Rewrite the .ergrc [tags] section header to [labels] (idempotent).
	if migrateErgrc(dir) {
		fmt.Println(".ergrc [tags] \u2192 [labels] rewrite: 1 file")
	}

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
			fmt.Fprintf(os.Stderr, "migrate: rename archive/->closed/: %v\n", err)
		} else {
			fmt.Println("migrate: renamed archive/ \u2192 closed/")
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
				fmt.Fprintf(os.Stderr, "migrate: archive/->closed/ collision: %v -- resolve manually\n", conflicts)
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
				fmt.Printf("migrate: merged %d files archive/ \u2192 closed/, removed archive/\n", len(entries))
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
		fmt.Println("migrate: copied binary \u2192 tickets/erg")
	}
	if c, r, _, u, err := installAssets(root, false, false); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: init assets refresh failed: %v\n", err)
	} else {
		fmt.Printf("migrate: init assets refreshed (%d created, %d refreshed, %d unchanged)\n", c, r, u)
	}
	migrateHook(root)
	return 0
}

// migrateHook rewrites a legacy pre-commit hook in place so the post-upgrade
// hook keeps working. Two patterns are replaced: the pre-bootstrap binary
// path (tickets/tools/go/erg -> tickets/erg) and the old corpus-check CLI
// form (validate tickets/ -> check tickets/, which the current binary
// rejects in favor of `erg check`). Detection is content-based -- no marker
// comments are required -- and the rewrite is idempotent: a hook with no
// legacy pattern is left untouched and silent. A hook that does not exist,
// or a .git directory that is unreadable (e.g. a worktree gitfile), is
// likewise skipped silently.
func migrateHook(root string) {
	hookPath := filepath.Join(root, ".git", "hooks", "pre-commit")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		return
	}
	original := string(data)
	updated := original
	updated = strings.ReplaceAll(updated, `erg_bin="tickets/tools/go/erg"`, `erg_bin="tickets/erg"`)
	updated = strings.ReplaceAll(updated, `"$erg_bin" validate tickets/`, `"$erg_bin" check tickets/`)
	updated = strings.ReplaceAll(updated, `$erg_bin validate tickets/`, `$erg_bin check tickets/`)
	if updated == original {
		return
	}
	if err := os.WriteFile(hookPath, []byte(updated), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: rewrite hook: %v\n", err)
		return
	}
	fmt.Println("migrate: rewrote .git/hooks/pre-commit (tickets/tools/go/erg -> tickets/erg, validate -> check)")
}

// migrateResult summarizes what migrateFile rewrote in a single .erg file.
type migrateResult struct {
	changed         bool // file was rewritten on disk
	wasClosed       bool // at least one removed Status: line carried value "closed"
	statusStripped  bool // at least one Status: line was removed (closed or open/doing/pending)
	labelsRewritten bool // at least one preamble `Tag:`/`Tags:` line was rewritten to `Label:`
	magicRewritten  bool // legacy `%erg v1` magic line was rewritten to `%erg 0.1`
	blanksSwept     bool // at least one interior header blank line was removed
	folded          bool // at least one log continuation line was folded onto its entry
	stamped         bool // at least one date-only log stamp was normalised to T00:00Z
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
			// Rewrite legacy `Tags:` -> `Label:` preserving the value (and any
			// inline comment). Original casing of the value is kept. Converges
			// in one run -- no intermediate `Tag:` stop.
			rewritten := "Label:" + line[len("Tags:"):]
			out = append(out, rewritten)
			res.labelsRewritten = true
			continue
		}
		if i < preambleEnd && isTagHeaderLine(line) {
			// Rewrite `Tag:` -> `Label:` preserving the value (and any inline
			// comment). isTagsHeaderLine is checked first above, so a `Tags:`
			// line never reaches here (the 4-char `Tag:` prefix excludes it).
			rewritten := "Label:" + line[len("Tag:"):]
			out = append(out, rewritten)
			res.labelsRewritten = true
			continue
		}
		out = append(out, line)
	}

	if res.wasClosed {
		// After removing Status: lines, the preamble end shifts by however
		// many we dropped -- recompute relative to the rewritten slice. Insert
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

	// Fold legacy free-form log content (wrapped detail, prose paragraphs,
	// numbered lists) onto its parent entry, and normalise date-only stamps.
	// Operates on the log section only -- preamble and body untouched.
	logStartIdx := -1
	bodyStartIdx := len(out)
	for i, line := range out {
		if strings.TrimSpace(line) == separatorLog && logStartIdx < 0 {
			logStartIdx = i
		}
		if strings.TrimSpace(line) == separatorBody {
			bodyStartIdx = i
			break
		}
	}
	if logStartIdx >= 0 {
		logSlice := out[logStartIdx+1 : bodyStartIdx]
		foldedSlice, didFold, didStamp := foldLogLines(logSlice)
		if didFold || didStamp {
			// Build a fresh slice to avoid append aliasing into out's backing array.
			newOut := make([]string, 0, len(out))
			newOut = append(newOut, out[:logStartIdx+1]...)
			newOut = append(newOut, foldedSlice...)
			newOut = append(newOut, out[bodyStartIdx:]...)
			out = newOut
			res.folded = didFold
			res.stamped = didStamp
		}
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
	// Rewrite through the shared audited path so migrate gets the same atomic
	// replace + validate-before-replace as the other mutators. The store root
	// is the file's own directory (migrate has no separately-resolved store in
	// hand), so confinement is a no-op here; the value is the atomic temp-then-
	// rename plus the guard that migrate never replaces a clean ticket with
	// unparseable output. The validate gate does NOT block legitimate
	// migrations: a legacy file (e.g. carrying a Status: header) is already
	// invalid by current rules, so writeTicketAtomic permits rewriting it.
	if err := writeTicketAtomic(filepath.Dir(path), path, []byte(rejoined)); err != nil {
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
// to the `Label:` form.
func isTagsHeaderLine(line string) bool {
	return hasHeaderKey(line, "Tags:", true)
}

// isTagHeaderLine reports whether a line begins with the literal `Tag:`
// header key (case-insensitive on the key itself). Used to rewrite legacy
// `Tag:` preamble lines to the `Label:` form. The 4-char `Tag:` prefix does
// not match `Tags:` (5 chars), so the two rewrite passes are disjoint.
func isTagHeaderLine(line string) bool {
	return hasHeaderKey(line, "Tag:", true)
}

// migrateErgrc rewrites the `[tags]` section header to `[labels]` in dir/.ergrc,
// returning true when the file existed and was changed. Operates on the raw
// section-header line only (text-level, no loadConfig dependency); section
// comments and entries are preserved verbatim. Idempotent: a .ergrc already
// using `[labels]`, or absent entirely, is a no-op (returns false).
func migrateErgrc(dir string) bool {
	path := filepath.Join(dir, ".ergrc")
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	original := string(data)
	lines := strings.Split(original, "\n")
	changed := false
	for i, line := range lines {
		if strings.TrimSpace(line) == "[tags]" {
			lines[i] = strings.Replace(line, "[tags]", "[labels]", 1)
			changed = true
		}
	}
	if !changed {
		return false
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: rewrite .ergrc: %v\n", err)
		return false
	}
	return true
}

// foldLogLines folds legacy free-form log content into well-formed %erg 0.1
// log lines. It operates on the log-section slice ONLY (the lines between
// `--- log ---` and `--- body ---`); preamble and body are never passed in.
//
// Two rewrites, both idempotent:
//
//  1. Fold: any non-blank line that does not open a new entry (its first 10
//     characters are not a YYYY-MM-DD date) is joined, single-space and
//     stripped, onto the preceding entry. Blank lines between an entry and its
//     continuation are dropped. Content before the first timestamped entry is
//     an orphan -- it has no parent entry, so it is emitted verbatim and left
//     for the validator to flag.
//  2. Stamp: a leading `YYYY-MM-DD ` (date, no T separator) on an entry-opener
//     is rewritten to `YYYY-MM-DDT00:00Z `.
//
// Blank-line preservation is precise: a blank is emitted iff it is followed by
// a new entry-opener, the body separator, or EOF -- never before continuation
// text. This keeps inter-entry blanks and the canonical trailing blank before
// `--- body ---`, while discarding blanks that merely separated an entry from
// its wrapped detail. The result is a byte-level no-op on an already-clean log.
func foldLogLines(log []string) (out []string, folded bool, stamped bool) {
	var cur string
	var pendingBlanks []string
	hasCur := false
	seenFirstEntry := false
	for _, line := range log {
		if strings.TrimSpace(line) == "" {
			pendingBlanks = append(pendingBlanks, line)
			continue
		}
		if logEntryPrefixRE.MatchString(line) {
			// New entry opener: flush the accumulated entry and any pending
			// blanks (inter-entry blanks are preserved).
			if hasCur {
				out = append(out, cur)
			}
			out = append(out, pendingBlanks...)
			pendingBlanks = nil
			seenFirstEntry = true
			if logDateOnlyRE.MatchString(line) {
				// line[10] is the space after the date; splice in T00:00Z.
				line = line[:10] + "T00:00Z" + line[10:]
				stamped = true
			}
			cur = line
			hasCur = true
			continue
		}
		// Non-blank, non-opener: continuation or orphan.
		if seenFirstEntry {
			// Continuation: drop the pending blanks that separated this detail
			// from its entry, then fold onto the current entry.
			pendingBlanks = nil
			cur += " " + strings.TrimSpace(line)
			folded = true
			continue
		}
		// Orphan before the first entry: emit verbatim, leave for the validator.
		out = append(out, pendingBlanks...)
		pendingBlanks = nil
		out = append(out, line)
	}
	if hasCur {
		out = append(out, cur)
	}
	out = append(out, pendingBlanks...)
	return out, folded, stamped
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
