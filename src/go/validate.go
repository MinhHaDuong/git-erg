package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validateErg returns rule violations for a single ticket. allIDs lists every
// ticket ID known to the run for Blocked-by resolution. diag carries
// parser observations (unknown/repeated headers, separator sightings,
// misplaced `Closed:` lines, empty `Closed:` values).
func validateErg(t *Erg, diag ParseDiagnostics, allIDs map[string]bool) []string {
	var errors []string
	name := t.Filename()

	// Rule 1: magic first line
	if !t.HasMagic {
		errors = append(errors, fmt.Sprintf("%s: missing magic first line '%%erg v1'", name))
	}

	// Rule 2: required headers — must be present AND non-empty.
	if strings.TrimSpace(t.Title) == "" {
		errors = append(errors, fmt.Sprintf("%s: missing or empty required header 'Title' — add 'Title: <text>' to the preamble", name))
	}
	if strings.TrimSpace(t.Created) == "" {
		errors = append(errors, fmt.Sprintf("%s: missing or empty required header 'Created' — add 'Created: YYYY-MM-DD' to the preamble", name))
	}
	if strings.TrimSpace(t.Author) == "" {
		errors = append(errors, fmt.Sprintf("%s: missing or empty required header 'Author' — add 'Author: <name>' to the preamble", name))
	}

	// Rule 3: no unknown headers (Status: and Tags: are relics; run `erg
	// migrate` to convert them).
	// Rule 4: non-repeatable headers appear at most once.
	// Unknown keys come from the parser in first-occurrence order.
	for _, key := range diag.Unknown {
		switch key {
		case "Status":
			errors = append(errors, fmt.Sprintf(
				"%s: 'Status:' header is no longer part of %%erg v1 — run `erg migrate` to convert", name))
		case "Tags":
			errors = append(errors, fmt.Sprintf(
				"%s: 'Tags:' has been renamed to 'Tag:' — run `erg migrate` to convert", name))
		default:
			errors = append(errors, fmt.Sprintf("%s: unknown header '%s' (not in v1 closed set) — remove it or run `erg migrate`", name, key))
		}
	}
	// (Rule 4 cont.) Singleton check: non-repeatable headers must appear at most once.
	for _, key := range diag.RepeatedSingletons {
		errors = append(errors, fmt.Sprintf(
			"%s: header '%s' is non-repeatable (appears more than once)", name, key))
	}

	// Rule 5: Tag: values must be from the closed value set.
	for _, v := range t.Tags {
		if !validTagValues[v] {
			errors = append(errors, fmt.Sprintf(
				"%s: unknown Tag value '%s' (not in v1 closed set: needs-human, deferred, post-talk, post-conference)", name, v))
		}
	}

	// Rule 6: Closed: header — value required, non-empty; not in log/body.
	if diag.ClosedEmpty {
		errors = append(errors, fmt.Sprintf(
			"%s: 'Closed:' header requires a non-empty value (closure reason)", name))
	}
	if diag.ClosedInLog {
		errors = append(errors, fmt.Sprintf(
			"%s: 'Closed:' header found in log section — only allowed in header section", name))
	}
	if diag.ClosedInBody {
		errors = append(errors, fmt.Sprintf(
			"%s: 'Closed:' header found in body section — only allowed in header section", name))
	}

	// Rule 7: Created is ISO date
	if c := t.Created; c != "" && !isoDateRE.MatchString(c) {
		errors = append(errors, fmt.Sprintf(
			"%s: Created '%s' is not a valid ISO date (YYYY-MM-DD)", name, c))
	}

	// Rule 8: filename matches NNNN-slug.erg
	if !filenameRE.MatchString(name) {
		errors = append(errors, fmt.Sprintf(
			"%s: filename does not match NNNN-slug.erg pattern", name))
	}

	// Rule 9: Blocked-by values parse to one of the two ref forms.
	// Rule 10: local refs point to existing ticket IDs.
	refs, refErrs := t.BlockedByRefs()
	for i, ref := range refs {
		if refErrs[i] != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", name, refErrs[i]))
			continue
		}
		if ref.Kind == RefLocal && !allIDs[ref.ID] {
			errors = append(errors, fmt.Sprintf(
				"%s: Blocked-by '%s' references unknown ticket ID", name, ref.ID))
		}
	}

	// Rule 11: log lines match format
	for _, line := range t.LogLines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !logLineRE.MatchString(trimmed) {
			errors = append(errors, fmt.Sprintf(
				"%s: malformed log line: %s", name, trimmed))
		}
	}

	// Rule 12: the first `--- log ---` and the first `--- body ---` in
	// order are the section separators; subsequent occurrences are body
	// text. Only the missing case is an error — a body that quotes the
	// separator literals is legitimate (rule 12 relaxation, ticket 0116).
	if !diag.HasLogSep {
		errors = append(errors, fmt.Sprintf("%s: missing '--- log ---' separator", name))
	}
	if !diag.HasBodySep {
		errors = append(errors, fmt.Sprintf("%s: missing '--- body ---' separator", name))
	}

	return errors
}

// detectCycles reports any dependency cycles among the tickets' Blocked-by
// edges. Only RefLocal edges participate; forge refs are terminal from
// this repo's view and cannot form local cycles.
func detectCycles(tickets []Erg) []string {
	var errors []string

	adj := make(map[string][]string)
	for i := range tickets {
		id := tickets[i].FilenameID()
		if id == "" {
			continue
		}
		var localRefs []string
		refs, errs := tickets[i].BlockedByRefs()
		for j, ref := range refs {
			if errs[j] != nil {
				continue // malformed — already reported by validateErg
			}
			if ref.Kind == RefLocal {
				localRefs = append(localRefs, ref.ID)
			}
		}
		adj[id] = localRefs
	}

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)
	for id := range adj {
		color[id] = white
	}

	// Use a shared stack with explicit push/pop to avoid Go slice aliasing bugs.
	var stack []string

	var dfs func(node string)
	dfs = func(node string) {
		color[node] = gray
		stack = append(stack, node) // push
		for _, neighbor := range adj[node] {
			c, exists := color[neighbor]
			if !exists {
				continue
			}
			if c == gray {
				start := 0
				for i, n := range stack {
					if n == neighbor {
						start = i
						break
					}
				}
				cycle := append([]string{}, stack[start:]...)
				cycle = append(cycle, neighbor)
				errors = append(errors, "dependency cycle: "+strings.Join(cycle, " -> "))
			} else if c == white {
				dfs(neighbor)
			}
		}
		stack = stack[:len(stack)-1] // pop
		color[node] = black
	}

	ids := sortedKeys(adj)
	for _, id := range ids {
		if color[id] == white {
			dfs(id)
		}
	}
	return errors
}

// validateAll runs every rule across the supplied tickets. diags must be
// the parallel-by-index slice of ParseDiagnostics emitted by parseErg.
func validateAll(tickets []Erg, diags []ParseDiagnostics) []string {
	var errors []string

	// Corpus check: no duplicate IDs (not a per-file rule)
	idToFiles := make(map[string][]string)
	for i := range tickets {
		id := tickets[i].FilenameID()
		if id != "" {
			idToFiles[id] = append(idToFiles[id], tickets[i].Filename())
		}
	}

	dupIDs := sortedKeys(idToFiles)
	for _, tid := range dupIDs {
		files := idToFiles[tid]
		if len(files) > 1 {
			errors = append(errors, fmt.Sprintf(
				"duplicate ID '%s' in: %s", tid, strings.Join(files, ", ")))
		}
	}

	// Build allIDs for reference checking
	allIDs := make(map[string]bool)
	for id := range idToFiles {
		allIDs[id] = true
	}

	// Per-ticket validation
	for i := range tickets {
		var diag ParseDiagnostics
		if i < len(diags) {
			diag = diags[i]
		}
		errors = append(errors, validateErg(&tickets[i], diag, allIDs)...)
	}

	// Rule 13: dependency cycles
	errors = append(errors, detectCycles(tickets)...)
	return errors
}

// globLocalIDs scans dir (non-recursively) for .erg files and returns a set
// of their filename IDs. Used by cmdValidate for per-file ref checking
// without loading a full corpus.
func globLocalIDs(dir string) map[string]bool {
	ids := make(map[string]bool)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ids
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".erg") {
			continue
		}
		stem := strings.TrimSuffix(e.Name(), ".erg")
		if idx := strings.Index(stem, "-"); idx > 0 {
			ids[stem[:idx]] = true
		}
	}
	return ids
}

// summaryValidate is the one-liner printed by printUsage via the commands registry.
const summaryValidate = "Validate individual .erg files (format, headers, refs)"

const helpValidate = `## erg validate FILE...

Validate individual .erg ticket files (format, headers, refs).

Each FILE must be a .erg ticket. For every file the validator enforces:

  1. Magic first line is '%erg v1' (rejects unknown versions).
  2. All required headers present AND non-empty: Title, Created, Author.
  3. No unknown headers (Status: is unknown; run 'erg migrate' to convert it).
  4. Non-repeatable headers (Title, Created, Author, Closed) appear at most once.
  5. Tag: values are from the closed set (needs-human, deferred, post-talk, post-conference).
  6. Closed: header has a non-empty value and does not appear in the log or body sections.
  7. Created is a valid ISO date (YYYY-MM-DD).
  8. Filename matches NNNN-slug.erg (4-digit ID, lowercase ASCII kebab slug).
  9. Blocked-by values parse as local-ref (NNNN, exactly 4 digits) or
     forge-ref (host/owner/repo#N, e.g. github.com/acme/myrepo#42).
  10. Local Blocked-by refs point to existing ticket IDs in the same directory.
  11. Log lines match 'YYYY-MM-DDThh:mmZ actor verb [detail]' format.
  12. Both separators (` + "`--- log ---`" + `, ` + "`--- body ---`" + `) appear at least once;
      the first occurrence of each is the section separator, subsequent
      occurrences are body text (legitimate bodies may quote the literals).
  13. No dependency cycles among local Blocked-by refs.

For corpus-level checks (duplicate IDs, cycles), use: erg check [dir]

Exit codes: 0 on pass, 1 on any violation. Directories are rejected — use erg check.
`

// cmdValidate implements `erg validate FILE...`. See helpValidate for the user-facing summary.
func cmdValidate(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: erg validate FILE...")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Validate individual .erg ticket files (format, headers, refs).")
		fmt.Fprintln(os.Stderr, "For corpus-level checks (duplicate IDs, cycles), use: erg check [dir]")
		return 1
	}

	var allErrors []string
	count := 0
	// Cache globLocalIDs per directory — avoids re-reading the same dir
	// when multiple files from the same directory are validated together.
	idCache := make(map[string]map[string]bool)
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: skipping %s (%v)\n", arg, err)
			continue
		}
		if info.IsDir() {
			fmt.Fprintf(os.Stderr, "ERROR: %s is a directory; use 'erg check %s' for directory validation\n", arg, arg)
			return 1
		}
		if !strings.HasSuffix(arg, ".erg") {
			fmt.Fprintf(os.Stderr, "WARNING: skipping %s (not a .erg file)\n", arg)
			continue
		}
		t, diag := parseErg(arg)
		dir := filepath.Dir(arg)
		localIDs, ok := idCache[dir]
		if !ok {
			localIDs = globLocalIDs(dir)
			idCache[dir] = localIDs
		}
		errs := validateErg(&t, diag, localIDs)
		allErrors = append(allErrors, errs...)
		count++
	}

	if count == 0 {
		fmt.Println("No .erg files found.")
		return 0
	}

	if len(allErrors) > 0 {
		errWord := "errors"
		if len(allErrors) == 1 {
			errWord = "error"
		}
		fmt.Printf("ERG VALIDATION FAILED (%d %s):\n", len(allErrors), errWord)
		for _, e := range allErrors {
			fmt.Printf("  %s\n", e)
		}
		return 1
	}

	fileWord := "files"
	if count == 1 {
		fileWord = "file"
	}
	fmt.Printf("ERG VALIDATION: PASS (%d %s)\n", count, fileWord)
	return 0
}
