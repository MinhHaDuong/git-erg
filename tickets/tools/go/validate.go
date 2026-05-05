package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	requiredHeaders = []string{"Title", "Created", "Author"}
	// Non-repeatable headers: appearing more than once is a validation error.
	singletonHeaders = map[string]bool{
		"Title": true, "Created": true, "Author": true, "Closed": true,
	}
	validHeaders = map[string]bool{
		"Title": true, "Created": true, "Author": true,
		"Closed": true, "Blocked-by": true, "Tags": true,
	}
	// validTagValues is the closed value set for the Tags: header (v1.x).
	validTagValues = map[string]bool{
		"needs-human":      true,
		"deferred":         true,
		"post-talk":        true,
		"post-conference":  true,
	}
	isoDateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	// Filename: 4-digit ID, dash, lowercase kebab slug
	filenameRE = regexp.MustCompile(`^\d{4}-[a-z0-9]+(-[a-z0-9]+)*\.erg$`)
	// Log line: ISO timestamp, space, actor, space, verb [detail]
	logLineRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}Z\s+\S+\s+\S+`)
)

// validateErg returns rule violations for a single ticket. allIDs lists every
// ticket ID known to the run for Blocked-by resolution.
func validateErg(t *Erg, allIDs map[string]bool) []string {
	var errors []string
	name := t.Filename()

	// Rule 1: magic first line
	if !t.HasMagic {
		errors = append(errors, fmt.Sprintf("%s: missing magic first line '%%erg v1'", name))
	}

	// Rule 2: required headers
	for _, hdr := range requiredHeaders {
		if _, ok := t.Headers[hdr]; !ok {
			errors = append(errors, fmt.Sprintf("%s: missing required header '%s'", name, hdr))
		}
	}

	// Rule 3: no unknown headers (Status: is a relic of the pre-0022 format;
	// run `erg migrate` to convert it).
	for key, vals := range t.Headers {
		if !validHeaders[key] {
			if key == "Status" {
				errors = append(errors, fmt.Sprintf(
					"%s: 'Status:' header is no longer part of %%erg v1 — run `erg migrate` to convert", name))
			} else {
				errors = append(errors, fmt.Sprintf("%s: unknown header '%s' (not in v1 closed set)", name, key))
			}
			continue
		}
		// Singleton check: non-repeatable headers must appear at most once.
		if singletonHeaders[key] && len(vals) > 1 {
			errors = append(errors, fmt.Sprintf(
				"%s: header '%s' is non-repeatable (appears %d times)", name, key, len(vals)))
		}
	}

	// Rule 3a: Tags: values must be from the closed value set.
	if tags, ok := t.Headers["Tags"]; ok {
		for _, v := range tags {
			if !validTagValues[strings.TrimSpace(v)] {
				errors = append(errors, fmt.Sprintf(
					"%s: unknown Tags value '%s' (not in v1 closed set)", name, v))
			}
		}
	}

	// Rule 4: Closed: header — value required, non-empty; not in log/body.
	if vals, ok := t.Headers["Closed"]; ok {
		for _, v := range vals {
			if strings.TrimSpace(v) == "" {
				errors = append(errors, fmt.Sprintf(
					"%s: 'Closed:' header requires a non-empty value (closure reason)", name))
				break
			}
		}
	}
	if t.ClosedInLog {
		errors = append(errors, fmt.Sprintf(
			"%s: 'Closed:' header found in log section — only allowed in header section", name))
	}
	if t.ClosedInBody {
		errors = append(errors, fmt.Sprintf(
			"%s: 'Closed:' header found in body section — only allowed in header section", name))
	}

	// Rule 5: Created is ISO date
	if created, ok := t.Headers["Created"]; ok && len(created) > 0 {
		if created[0] != "" && !isoDateRE.MatchString(created[0]) {
			errors = append(errors, fmt.Sprintf(
				"%s: Created '%s' is not a valid ISO date (YYYY-MM-DD)", name, created[0]))
		}
	}

	// Rule 6: filename matches NNNN-slug.erg
	if !filenameRE.MatchString(name) {
		errors = append(errors, fmt.Sprintf(
			"%s: filename does not match NNNN-slug.erg pattern", name))
	}

	// Rule 7: Blocked-by values parse to one of the two ref forms.
	// Rule 8: local refs point to existing ticket IDs.
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

	// Rule 10: log lines match format
	for _, line := range t.LogLines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !logLineRE.MatchString(trimmed) {
			errors = append(errors, fmt.Sprintf(
				"%s: malformed log line: %s", name, trimmed))
		}
	}

	// Rule 11: each separator appears exactly once
	if !t.HasLog {
		errors = append(errors, fmt.Sprintf("%s: missing '--- log ---' separator", name))
	} else if t.LogSepCount > 1 {
		errors = append(errors, fmt.Sprintf("%s: '--- log ---' separator appears %d times (expected 1)", name, t.LogSepCount))
	}
	if !t.HasBody {
		errors = append(errors, fmt.Sprintf("%s: missing '--- body ---' separator", name))
	} else if t.BodySepCount > 1 {
		errors = append(errors, fmt.Sprintf("%s: '--- body ---' separator appears %d times (expected 1)", name, t.BodySepCount))
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

	ids := sortedKeys2(adj)
	for _, id := range ids {
		if color[id] == white {
			dfs(id)
		}
	}
	return errors
}

// validateAll runs every rule across the supplied tickets.
func validateAll(tickets []Erg) []string {
	var errors []string

	// Rule 7: no duplicate IDs
	idToFiles := make(map[string][]string)
	for i := range tickets {
		id := tickets[i].FilenameID()
		if id != "" {
			idToFiles[id] = append(idToFiles[id], tickets[i].Filename())
		}
	}

	dupIDs := sortedKeys2(idToFiles)
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
		errors = append(errors, validateErg(&tickets[i], allIDs)...)
	}

	// Rule 9: dependency cycles
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

// cmdValidate implements `erg validate <file ...>` — per-file format checks.
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
		t := parseErg(arg)
		localIDs := globLocalIDs(filepath.Dir(arg))
		errs := validateErg(&t, localIDs)
		allErrors = append(allErrors, errs...)
		count++
	}

	if count == 0 {
		fmt.Println("No .erg files found.")
		return 0
	}

	if len(allErrors) > 0 {
		fmt.Printf("ERG VALIDATION FAILED (%d error(s)):\n", len(allErrors))
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
