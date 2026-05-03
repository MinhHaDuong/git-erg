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
		"Closed": true, "Blocked-by": true,
	}
	isoDateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	// Filename: 4-digit ID, dash, lowercase kebab slug
	filenameRE = regexp.MustCompile(`^\d{4}-[a-z0-9]+(-[a-z0-9]+)*\.erg$`)
	// Log line: ISO timestamp, space, actor, space, verb [detail]
	logLineRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}Z\s+\S+\s+\S+`)
)

// validateErg returns rule violations for a single ticket. allIDs lists every
// ticket ID known to the run (live + archived) for Blocked-by resolution.
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
			"%s: 'Closed:' header found in log section — only allowed in preamble", name))
	}
	if t.ClosedInBody {
		errors = append(errors, fmt.Sprintf(
			"%s: 'Closed:' header found in body section — only allowed in preamble", name))
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

	// Rule 8: Blocked-by refs exist
	for _, refID := range t.BlockedBy() {
		if strings.HasPrefix(refID, "gh#") {
			continue // GitHub issue reference — not validated locally
		}
		if !allIDs[refID] {
			errors = append(errors, fmt.Sprintf(
				"%s: Blocked-by '%s' references unknown ticket ID", name, refID))
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
// edges (gh# refs ignored).
func detectCycles(tickets []Erg) []string {
	var errors []string

	adj := make(map[string][]string)
	for i := range tickets {
		id := tickets[i].FilenameID()
		if id != "" {
			var localRefs []string
			for _, ref := range tickets[i].BlockedBy() {
				if !strings.HasPrefix(ref, "gh#") {
					localRefs = append(localRefs, ref)
				}
			}
			adj[id] = localRefs
		}
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

// validateAll runs every rule across the supplied tickets, treating extraIDs
// (typically archived ticket IDs) as valid Blocked-by targets.
func validateAll(tickets []Erg, extraIDs map[string]bool) []string {
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

	// Check collisions with archived ticket IDs
	if extraIDs != nil {
		for tid := range idToFiles {
			if extraIDs[tid] {
				errors = append(errors, fmt.Sprintf(
					"ID '%s' in %s collides with an archived ticket",
					tid, strings.Join(idToFiles[tid], ", ")))
			}
		}
	}

	// Build allIDs for reference checking
	allIDs := make(map[string]bool)
	for id := range idToFiles {
		allIDs[id] = true
	}
	for id := range extraIDs {
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

// cmdValidate implements `erg validate [dir|file ...]`.
func cmdValidate(args []string) int {
	if len(args) == 0 {
		args = []string{"tickets/"}
	}

	var tickets []Erg
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			fmt.Printf("WARNING: skipping %s (%v)\n", arg, err)
			continue
		}
		if info.IsDir() {
			tickets = append(tickets, loadErgs(arg)...)
		} else if strings.HasSuffix(arg, ".erg") {
			tickets = append(tickets, parseErg(arg))
		} else {
			fmt.Printf("WARNING: skipping %s (not a .erg file or directory)\n", arg)
		}
	}

	if len(tickets) == 0 {
		fmt.Println("No .erg files found.")
		return 0
	}

	// Load archived ticket IDs as valid Blocked-by targets
	extraIDs := make(map[string]bool)
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil || !info.IsDir() {
			continue
		}
		archiveDir := filepath.Join(arg, "archive")
		if info, err := os.Stat(archiveDir); err == nil && info.IsDir() {
			for _, at := range loadErgs(archiveDir) {
				id := at.FilenameID()
				if id != "" {
					extraIDs[id] = true
				}
			}
		}
	}

	errors := validateAll(tickets, extraIDs)
	if len(errors) > 0 {
		fmt.Printf("ERG VALIDATION FAILED (%d error(s)):\n", len(errors))
		for _, e := range errors {
			fmt.Printf("  %s\n", e)
		}
		return 1
	}

	fmt.Printf("ERG VALIDATION: PASS (%d tickets)\n", len(tickets))
	return 0
}
