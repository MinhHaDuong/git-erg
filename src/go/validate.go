package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
		for _, ref := range tickets[i].BlockedBys {
			corpusOpCount++ // per-ref adjacency build
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
			corpusOpCount++ // per-edge DFS visit
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

// validateLabelVocabulary checks that all labels on a ticket belong to the
// effective vocabulary, returning error strings for any that don't.
func validateLabelVocabulary(t *Erg, labelSet map[string]bool, validList []string) []string {
	var errs []string
	name := t.Filename()
	for j, v := range t.Labels {
		if !labelSet[v] {
			lineInfo := ""
			if j < len(t.LabelLines) {
				lineInfo = fmt.Sprintf(":%d", t.LabelLines[j])
			}
			errs = append(errs, fmt.Sprintf(
				"%s%s: unknown Label value '%s' (valid labels: %s)",
				name, lineInfo, v, strings.Join(validList, ", ")))
		}
	}
	return errs
}

// validateCorpus runs the corpus-level rules across all tickets and
// folds in the per-file parse errors. parseErrs is the parallel-by-index
// slice of parse errors emitted by parseErg / loadErgs (already covers
// rules 1-9, 11, 12). validateCorpus adds:
//   - duplicate ID detection (no rule number; corpus-level invariant)
//   - rule 10: local Blocked-by refs resolve to existing ticket IDs
//   - rule 13: no dependency cycles among local Blocked-by edges
func validateCorpus(tickets []Erg, parseErrs [][]string, cfg *Config) []string {
	var errors []string

	// Fold in per-file parse errors first (rules 1-9, 11, 12).
	for _, e := range parseErrs {
		errors = append(errors, e...)
	}

	// Rule 5: Label values must be from the effective vocabulary.
	labelSet := effectiveLabelSet(cfg)
	validList := sortedKeys(labelSet)
	for i := range tickets {
		corpusOpCount++ // per-ticket label-vocabulary check
		errors = append(errors, validateLabelVocabulary(&tickets[i], labelSet, validList)...)
	}

	// Corpus check: no duplicate IDs (not a per-file rule).
	idToFiles := make(map[string][]string)
	for i := range tickets {
		corpusOpCount++ // per-ticket ID extraction
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

	// Build allIDs for reference checking.
	allIDs := make(map[string]bool)
	for id := range idToFiles {
		allIDs[id] = true
	}

	// Rule 10: local Blocked-by refs point to existing ticket IDs in the
	// corpus.
	for i := range tickets {
		t := &tickets[i]
		name := t.Filename()
		for _, ref := range t.BlockedBys {
			corpusOpCount++ // per-ref lookup
			if ref.Kind == RefLocal && !idExists(allIDs, ref.ID) {
				errors = append(errors, fmt.Sprintf(
					"%s: Blocked-by '%s' references unknown ticket ID", name, ref.ID))
			}
		}
	}

	// Rule 15: local Superseded-by refs point to existing ticket IDs in the
	// corpus. Self-reference is caught at parse time (folded in above).
	for i := range tickets {
		t := &tickets[i]
		name := t.Filename()
		for _, ref := range t.SupersededBys {
			corpusOpCount++ // per-ref lookup
			if ref.Kind == RefLocal && !idExists(allIDs, ref.ID) {
				errors = append(errors, fmt.Sprintf(
					"%s: Superseded-by '%s' references unknown ticket ID", name, ref.ID))
			}
		}
	}

	// Rule 13: dependency cycles.
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

  1. Magic first line is '%erg 0.1' (rejects unknown versions).
  2. All required headers present AND non-empty: Title, Created, Author.
  3. No unknown headers (Status: is unknown; run 'erg migrate' to convert it).
  4. Non-repeatable headers (Title, Created, Author, Closed) appear at most once.
  5. Label: values are from the vocabulary (default: needs-human, deferred; see tickets/.ergrc [labels]).
  6. Closed: header has a non-empty value and does not appear in the log or body sections.
  7. Created is a valid ISO date (YYYY-MM-DD).
  8. Filename matches NNNN-slug.erg (4-digit ID, lowercase ASCII kebab slug).
  9. Blocked-by values parse as local-ref (NNNN, exactly 4 digits) or
     forge-ref (host/owner/repo#N, e.g. github.com/acme/myrepo#42).
  10. Local Blocked-by refs point to existing ticket IDs in the same directory.
  11. Log lines match structural format: timestamp (YYYY-MM-DDThh:mmZ)
      followed by at least two whitespace-separated tokens. By convention
      these are 'actor verb [detail]', but the validator checks structure,
      not the semantic meaning of those tokens.
  12. Both separators (` + "`--- log ---`" + `, ` + "`--- body ---`" + `) appear at least once;
      the first occurrence of each is the section separator, subsequent
      occurrences are body text (legitimate bodies may quote the literals).
  13. No dependency cycles among local Blocked-by refs.
  14. Title does not begin or end with a status word (ready, done, closed,
      open) -- these read as a status assertion about the ticket rather than
      the thing being changed. Enforced on open tickets; closed tickets are
      grandfathered (existing closed history is never flagged).
  15. Superseded-by values parse as local-ref (NNNN), path-ref (module/NNNN),
      or forge-ref (host/owner/repo#N) -- same grammar as Blocked-by. Local
      refs must point to existing ticket IDs. Self-reference is an error.
      Repeatable (one-to-many supersession). Carried by the CLOSED ticket,
      pointing at the ticket(s) that replace it; it is durable lineage and is
      never stripped on close.

Error format: 'filename:LINE: message' when a specific line applies
(rules 1-7, 9, 11, 14, 15 self-ref); 'filename: message' when no line applies (rules 8, 12, 10, 15 unknown-ref).
Line numbers are 1-indexed.

For corpus-level checks (duplicate IDs, cycles), use: erg check [dir]

Exit codes: 0 on pass, 1 on any violation. Directories are rejected -- use erg check.
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

	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			fmt.Fprintf(os.Stderr, "validate: unknown flag %q\nUsage: erg validate FILE...\n", a)
			return 1
		}
	}

	var allErrors []string
	count := 0
	// Cache globLocalIDs and Config per directory.
	idCache := make(map[string]map[string]bool)
	cfgCache := make(map[string]*Config)
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
		t, parseErrs := parseErg(arg)
		// Shout (non-fatal) on an interior header blank: validate runs in the
		// pre-commit hook, so this is where an author sees the nudge at commit
		// time. The file is still accepted -- exit code is unaffected (ticket 0141).
		if data, rerr := os.ReadFile(arg); rerr == nil && hasInteriorHeaderBlank(data) {
			fmt.Fprintf(os.Stderr,
				"WARNING: %s: blank line inside header block -- run `erg migrate` to normalise (tolerated; not a validation error)\n",
				t.Filename())
		}
		dir := filepath.Dir(arg)
		localIDs, ok := idCache[dir]
		if !ok {
			localIDs = globLocalIDs(dir)
			idCache[dir] = localIDs
		}
		cfg, cfgOk := cfgCache[dir]
		if !cfgOk {
			var cfgErr error
			cfg, cfgErr = loadConfig(dir)
			if cfgErr != nil {
				fmt.Fprintf(os.Stderr, "validate: cannot read .ergrc in %s: %v\n", dir, cfgErr)
				return 1
			}
			cfgCache[dir] = cfg
		}
		allErrors = append(allErrors, parseErrs...)
		// Rule 10: local Blocked-by refs resolve to a known ticket ID.
		name := t.Filename()
		for _, ref := range t.BlockedBys {
			if ref.Kind == RefLocal && !localIDs[ref.ID] {
				allErrors = append(allErrors, fmt.Sprintf(
					"%s: Blocked-by '%s' references unknown ticket ID", name, ref.ID))
			}
		}
		// Rule 15: local Superseded-by refs resolve to a known ticket ID.
		for _, ref := range t.SupersededBys {
			if ref.Kind == RefLocal && !localIDs[ref.ID] {
				allErrors = append(allErrors, fmt.Sprintf(
					"%s: Superseded-by '%s' references unknown ticket ID", name, ref.ID))
			}
		}
		// Rule 5: Label values from effective vocabulary.
		labelSet := effectiveLabelSet(cfg)
		validList := sortedKeys(labelSet)
		allErrors = append(allErrors, validateLabelVocabulary(&t, labelSet, validList)...)
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
		fmt.Fprintf(os.Stderr, "ERG VALIDATION FAILED (%d %s):\n", len(allErrors), errWord)
		for _, e := range allErrors {
			fmt.Fprintf(os.Stderr, "  %s\n", e)
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
