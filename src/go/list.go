package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// blockedByEntry describes one unsatisfied blocker of a ticket: a forge ref
// (offline-unknown, always blocking) or an open local ticket.
type blockedByEntry struct {
	kind string
	id   string
	ref  string
}

type listEntry struct {
	id, title, file string
	closed          bool
	blocked         bool
	tags            []string
	blockedBy       []blockedByEntry
}

// has reports whether the entry carries the given filter term. closed, open,
// and blocked are computed pseudo-tags (open == not closed); any other name is
// matched against the ticket's literal Tag: headers.
func (e listEntry) has(term string) bool {
	switch term {
	case "closed":
		return e.closed
	case "open":
		return !e.closed
	case "blocked":
		return e.blocked
	default:
		for _, t := range e.tags {
			if t == term {
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
			if ref.Kind == RefInvalid {
				continue // malformed refs are validator territory
			}
			if ref.IsForge() {
				// Forge refs are offline-unknown → always blocking.
				blockedBy = append(blockedBy, blockedByEntry{kind: "forge", ref: ref.Raw})
				continue
			}
			if !knownID[ref.ID] {
				warnings = append(warnings, fmt.Sprintf(
					"%s: Blocked-by '%s' not found (treating as satisfied)", t.Filename(), ref.ID))
				continue
			}
			if !closedByID[ref.ID] {
				blockedBy = append(blockedBy, blockedByEntry{kind: "local", id: ref.ID})
			}
		}
		entries = append(entries, listEntry{
			id:        t.FilenameID(),
			title:     t.Title,
			file:      t.Filename(),
			closed:    t.IsClosed(),
			blocked:   len(blockedBy) > 0,
			tags:      t.Tags,
			blockedBy: blockedBy,
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].id < entries[j].id
	})
	return entries, warnings
}

// summaryList is the one-liner printed by printUsage via the commands registry.
const summaryList = "List tickets, filtered by tag (alias: ls)"

const helpList = `## erg list [DIR] [TAG...] [not TAG...] [--all] [--json]

List tickets, one per line: ID, title, tags, and blocked-by refs.

Tag arguments filter the list as a conjunction: a bare TAG keeps only tickets
carrying it, and "not TAG" drops tickets carrying it. Beyond the literal Tag:
vocabulary, three computed pseudo-tags are accepted:

  - closed   — the ticket is closed (Closed: header or closed/ path).
  - open     — the ticket is not closed.
  - blocked  — the ticket has an unsatisfied blocker (a forge ref, or a
               Blocked-by pointing at an open local ticket).

Open is the default: with no open/closed term and without --all, only open
tickets are shown. --all drops that default so closed tickets appear too
(marked [closed]). Tickets are sorted by ID ascending.

DIR (a path argument containing '/', or '.') selects the ticket store; every
other bare word is a filter term, so 'erg ls closed' lists closed tickets while
'erg ls tickets/' lists the store at tickets/.

Without --json, prints a human-readable line per ticket. With --json, prints a
JSON array where each element has the fields: id, title, file, closed, tags,
blocked_by.

Alias: erg ls.

Examples:
  erg ls                      open tickets
  erg ls needs-human          open tickets tagged needs-human
  erg ls not deferred         open tickets not tagged deferred
  erg ls closed               closed tickets
  erg ls --all blocked        all blocked tickets, open or closed
`

// isDirArg reports whether a bare argument denotes the ticket store directory
// rather than a filter term. A term is a directory when it contains a path
// separator or is the current/parent directory, which keeps slash-less words
// (including the pseudo-tags closed/open/blocked) as filter terms.
func isDirArg(arg string) bool {
	return strings.Contains(arg, "/") || arg == "." || arg == ".."
}

// cmdList implements `erg list [dir] [tag...] [not tag...] [--all] [--json]`.
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
		case isDirArg(a):
			if negateNext {
				fmt.Fprintln(os.Stderr, "list: 'not' must be followed by a tag name")
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
		fmt.Fprintln(os.Stderr, "list: 'not' must be followed by a tag name")
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
// local refs, the raw ref string for forge refs.
func blockedByLabel(b blockedByEntry) string {
	if b.kind == "forge" {
		return b.ref
	}
	return b.id
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
		if len(e.tags) > 0 {
			suffix += fmt.Sprintf(" (tags: %s)", strings.Join(e.tags, ", "))
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
		Tags      []string        `json:"tags"`
		BlockedBy []blockedByJSON `json:"blocked_by"`
	}

	result := make([]entryJSON, len(entries))
	for i, e := range entries {
		bb := make([]blockedByJSON, len(e.blockedBy))
		for j, b := range e.blockedBy {
			bb[j] = blockedByJSON{Kind: b.kind, ID: b.id, Ref: b.ref}
		}
		tags := e.tags
		if tags == nil {
			tags = []string{}
		}
		result[i] = entryJSON{
			ID: e.id, Title: e.title, File: e.file,
			Closed: e.closed, Tags: tags, BlockedBy: bb,
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "list: JSON marshal error: %v\n", err)
		return
	}
	fmt.Println(string(data))
}
