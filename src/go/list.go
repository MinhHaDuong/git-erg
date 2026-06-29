package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// blockedByEntry describes one unsatisfied blocker of a ticket: an open local
// ticket (kind "local") or a relative path-ref that resolves to an open ticket
// in a sibling store (kind "path"). Unresolved references never become a
// blocker -- the policy is optimistic; they surface as warnings instead.
type blockedByEntry struct {
	kind string
	id   string
	ref  string
}

type listEntry struct {
	id, title, file string
	closed          bool
	blocked         bool
	labels          []string
	blockedBy       []blockedByEntry
	refs            []string // git refs + worktree paths referencing this ticket
}

// has reports whether the entry carries the given filter term. closed, open,
// and blocked are computed pseudo-labels (open == not closed); any other name is
// matched against the ticket's literal Label: headers.
func (e listEntry) has(term string) bool {
	switch term {
	case "closed":
		return e.closed
	case "open":
		return !e.closed
	case "blocked":
		return e.blocked
	default:
		for _, l := range e.labels {
			if l == term {
				return true
			}
		}
		return false
	}
}

// filter is a conjunction: an entry matches when it has every positive term
// and none of the negative terms.
type filter struct {
	positive []string
	negative []string
}

func (f filter) matches(e listEntry) bool {
	for _, p := range f.positive {
		if !e.has(p) {
			return false
		}
	}
	for _, n := range f.negative {
		if e.has(n) {
			return false
		}
	}
	return true
}

// loadListEntries parses every ticket under dir and computes its closed/blocked
// state and unsatisfied blockers, sorted by ID ascending. Warnings (returned
// separately, printed by callers) flag Blocked-by refs to unknown local IDs,
// which are treated as satisfied. Shared by `erg list` and `erg ready`.
func loadListEntries(dir string) ([]listEntry, []string) {
	tickets, _ := loadErgs(dir)

	closedByID := make(map[string]bool)
	knownID := make(map[string]bool)
	for i := range tickets {
		id := tickets[i].FilenameID()
		if id != "" {
			closedByID[id] = tickets[i].IsClosed()
			knownID[id] = true
		}
	}

	var warnings []string
	var entries []listEntry
	for i := range tickets {
		t := &tickets[i]
		var blockedBy []blockedByEntry
		for _, ref := range t.BlockedBys {
			// Optimistic resolution: a dependent is blocked only by a reference
			// that resolves to an OPEN ticket. An unresolved reference is a
			// non-fatal warning, never a blocker (ticket 0253).
			switch ref.Kind {
			case RefInvalid:
				continue // malformed refs are validator territory
			case RefLocal:
				if !knownID[ref.ID] {
					warnings = append(warnings, fmt.Sprintf(
						"%s: Blocked-by '%s' unresolved (treating as satisfied)", t.Filename(), ref.Raw))
					continue
				}
				if !closedByID[ref.ID] {
					blockedBy = append(blockedBy, blockedByEntry{kind: "local", id: ref.ID})
				}
			case RefPath:
				switch resolvePathRef(dir, ref) {
				case refOpen:
					blockedBy = append(blockedBy, blockedByEntry{kind: "path", ref: ref.Raw})
				case refClosed:
					// satisfied
				default:
					warnings = append(warnings, fmt.Sprintf(
						"%s: Blocked-by '%s' unresolved (treating as satisfied)", t.Filename(), ref.Raw))
				}
			default: // RefURI -- absolute URI or other unresolvable handle
				warnings = append(warnings, fmt.Sprintf(
					"%s: Blocked-by '%s' unresolved (treating as satisfied)", t.Filename(), ref.Raw))
			}
		}
		entries = append(entries, listEntry{
			id:        t.FilenameID(),
			title:     t.Title,
			file:      t.Filename(),
			closed:    t.IsClosed(),
			blocked:   len(blockedBy) > 0,
			labels:    t.Labels,
			blockedBy: blockedBy,
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].id < entries[j].id
	})

	ids := make([]string, len(entries))
	for i, e := range entries {
		ids[i] = e.id
	}
	if refs := loadRefMatches(dir, ids); refs != nil {
		for i := range entries {
			entries[i].refs = refs[entries[i].id]
		}
	}

	return entries, warnings
}

// summaryList is the one-liner printed by printUsage via the commands registry.
const summaryList = "List tickets, filtered by label (alias: ls)"

const helpList = `## erg list [DIR] [LABEL...] [not LABEL...] [--all] [--json]

List tickets, one per line, sorted by ID. Each line carries any [refs] --
git branches, remote-tracking branches, and worktree paths that reference the
ticket per the spec-erg-v1.md matching rule -- plus (labels: ...) and (blocked-by:
...) when present. The refs scan is local-only (git for-each-ref, git worktree
list); no network calls.

Label arguments filter the list as a conjunction: a bare LABEL keeps only tickets
carrying it, and "not LABEL" drops tickets carrying it. Beyond the literal Label:
vocabulary, three computed pseudo-labels are accepted:

  - closed   -- the ticket is closed (Closed: header or closed/ path).
  - open     -- the ticket is not closed.
  - blocked  -- the ticket has a Blocked-by that resolves to an open ticket
               (a local NNNN, or an open sibling path-ref); an unresolved
               reference only warns, it does not block.

Open is the default: with no open/closed term and without --all, only open
tickets are shown. --all drops that default so closed tickets appear too
(marked [closed]). Tickets are sorted by ID ascending.

DIR selects the ticket store: an argument naming an existing directory (or one
containing '/'), e.g. 'erg ls tickets/'. The pseudo-labels closed/open/blocked are
always filter terms, so 'erg ls closed' lists closed tickets even from inside a
store that contains a closed/ directory.

Without --json, prints a human-readable line per ticket. With --json, prints a
JSON array where each element has the fields: id, title, file, closed, refs,
labels, blocked_by.

Alias: erg ls.

Examples:
  erg ls                      open tickets
  erg ls needs-human          open tickets labeled needs-human
  erg ls not deferred         open tickets not labeled deferred
  erg ls closed               closed tickets
  erg ls --all blocked        all blocked tickets, open or closed
`

// pseudoLabelSet holds the computed filter terms. They are always filters, never
// directory arguments -- so `erg ls closed` filters even from inside a store
// that happens to contain a closed/ subdirectory.
var pseudoLabelSet = map[string]bool{"closed": true, "open": true, "blocked": true}

// isDirArg reports whether a bare argument denotes the ticket store directory
// rather than a filter term. An argument is a directory when it names an
// existing directory, contains a path separator, or is the current/parent
// directory. The pseudo-labels closed/open/blocked are reserved as filter terms.
func isDirArg(arg string) bool {
	if pseudoLabelSet[arg] {
		return false
	}
	if strings.Contains(arg, "/") || arg == "." || arg == ".." {
		return true
	}
	info, err := os.Stat(arg)
	return err == nil && info.IsDir()
}

// cmdList implements `erg list [dir] [label...] [not label...] [--all] [--json]`.
func cmdList(args []string) int {
	useJSON := false
	includeAll := false
	var explicitDir string
	var positive, negative []string
	negateNext := false

	for _, a := range args {
		switch {
		case a == "--json":
			useJSON = true
		case a == "--all":
			includeAll = true
		case a == "not":
			negateNext = true
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "list: unknown flag %q\nUsage: erg list [DIR] [LABEL...] [not LABEL...] [--all] [--json]\n", a)
			return 1
		case isDirArg(a):
			if negateNext {
				fmt.Fprintln(os.Stderr, "list: 'not' must be followed by a label name")
				return 1
			}
			if explicitDir != "" {
				fmt.Fprintf(os.Stderr, "list: more than one directory given (%s, %s)\n", explicitDir, a)
				return 1
			}
			explicitDir = a
		default:
			if negateNext {
				negative = append(negative, a)
				negateNext = false
			} else {
				positive = append(positive, a)
			}
		}
	}
	if negateNext {
		fmt.Fprintln(os.Stderr, "list: 'not' must be followed by a label name")
		return 1
	}

	ticketDir, err := resolveDir(explicitDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list: %v\n", err)
		return 1
	}

	f := filter{positive: positive, negative: negative}
	if !includeAll && !referencesOpenClosed(positive, negative) {
		f.positive = append(f.positive, "open")
	}

	entries, warnings := loadListEntries(ticketDir)
	var matched []listEntry
	for _, e := range entries {
		if f.matches(e) {
			matched = append(matched, e)
		}
	}

	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "WARNING: %s\n", w)
	}

	if useJSON {
		printListJSON(matched)
		return 0
	}

	heading := "Tickets"
	if !includeAll && len(positive) == 0 && len(negative) == 0 {
		heading = "Open tickets"
	}
	printListText(heading, matched)
	return 0
}

// referencesOpenClosed reports whether the user already constrained closed/open
// state, in which case the implicit open default is not applied.
func referencesOpenClosed(positive, negative []string) bool {
	for _, terms := range [][]string{positive, negative} {
		for _, t := range terms {
			if t == "open" || t == "closed" {
				return true
			}
		}
	}
	return false
}

// blockedByLabel renders a blocker for human-readable output: the ticket ID for
// a local ref, the raw reference string for a path-ref.
func blockedByLabel(b blockedByEntry) string {
	if b.kind == "local" {
		return b.id
	}
	return b.ref
}

// refStatus is the outcome of resolving a reference: a blocker only when it
// resolves to an open ticket.
type refStatus int

const (
	refUnresolved refStatus = iota
	refOpen
	refClosed
)

// resolvePathRef resolves a relative path-ref (auth/0042) against the repo
// root: <repo-root>/<module>/tickets/<id>-*.erg. Returns refUnresolved when the
// store is not in a git repo, the sibling module is not present in this
// checkout, or the file cannot be read -- the optimistic policy then treats the
// reference as satisfied (a warning, not a block). No network.
func resolvePathRef(dir string, ref Ref) refStatus {
	top := worktreeTopFor(dir)
	if top == "" {
		return refUnresolved
	}
	base := filepath.Join(top, filepath.FromSlash(ref.Module), "tickets")
	// Never resolve outside the repo: a module like "../../etc" is unresolved,
	// not a window to glob the filesystem.
	if ok, err := withinStore(top, base); err != nil || !ok {
		return refUnresolved
	}
	matches, _ := filepath.Glob(filepath.Join(base, ref.ID+"-*.erg"))
	if len(matches) == 0 {
		matches, _ = filepath.Glob(filepath.Join(base, ref.ID+".erg"))
	}
	if len(matches) == 0 {
		return refUnresolved
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return refUnresolved
	}
	t, _ := parseErgBytes(data, matches[0])
	if t.IsClosed() {
		return refClosed
	}
	return refOpen
}

func printListText(heading string, entries []listEntry) {
	if len(entries) == 0 {
		fmt.Printf("No %s found.\n", strings.ToLower(heading))
		return
	}
	fmt.Printf("%s (%d):\n", heading, len(entries))
	for _, e := range entries {
		suffix := ""
		if e.closed {
			suffix += " [closed]"
		}
		if len(e.refs) > 0 {
			suffix += fmt.Sprintf(" [%s]", strings.Join(e.refs, ", "))
		}
		if len(e.labels) > 0 {
			suffix += fmt.Sprintf(" (labels: %s)", strings.Join(e.labels, ", "))
		}
		if len(e.blockedBy) > 0 {
			labels := make([]string, len(e.blockedBy))
			for i, b := range e.blockedBy {
				labels[i] = blockedByLabel(b)
			}
			suffix += fmt.Sprintf(" (blocked-by: %s)", strings.Join(labels, ", "))
		}
		fmt.Printf("  %-8s %s%s\n", e.id, e.title, suffix)
	}
}

func printListJSON(entries []listEntry) {
	type blockedByJSON struct {
		Kind string `json:"kind"`
		ID   string `json:"id,omitempty"`
		Ref  string `json:"ref,omitempty"`
	}
	type entryJSON struct {
		ID        string          `json:"id"`
		Title     string          `json:"title"`
		File      string          `json:"file"`
		Closed    bool            `json:"closed"`
		Refs      []string        `json:"refs"`
		Labels    []string        `json:"labels"`
		BlockedBy []blockedByJSON `json:"blocked_by"`
	}

	result := make([]entryJSON, len(entries))
	for i, e := range entries {
		bb := make([]blockedByJSON, len(e.blockedBy))
		for j, b := range e.blockedBy {
			bb[j] = blockedByJSON{Kind: b.kind, ID: b.id, Ref: b.ref}
		}
		labels := e.labels
		if labels == nil {
			labels = []string{}
		}
		refs := e.refs
		if refs == nil {
			refs = []string{}
		}
		result[i] = entryJSON{
			ID: e.id, Title: e.title, File: e.file,
			Closed: e.closed, Refs: refs, Labels: labels, BlockedBy: bb,
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "list: JSON marshal error: %v\n", err)
		return
	}
	fmt.Println(string(data))
}
