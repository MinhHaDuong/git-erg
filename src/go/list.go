package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type listEntry struct {
	id, title, file string
	closed          bool
	tags            []string
	blockedBy       []blockedByEntry
}

// summaryList is the one-liner printed by printUsage via the commands registry.
const summaryList = "List all open tickets (alias: ls)"

const helpList = `## erg list [DIR] [--all] [--json]

List tickets, one per line: ID, title, tags, and blocked-by refs.

By default only open tickets are shown. With --all, closed tickets are included
too (and marked [closed] in the human-readable output). Tickets are sorted by ID
ascending.

Without --json, prints a human-readable line per ticket. With --json, prints a
JSON array where each element has the fields: id, title, file, closed, tags,
blocked_by.

Alias: erg ls.

Unlike 'erg ready', which shows only unblocked tickets you can pick up now,
'erg list' shows the full picture — blocked and tagged tickets included — so it
answers "what is still open?".
`

// cmdList implements `erg list [dir] [--all] [--json]`. See helpList for the
// user-facing summary.
func cmdList(args []string) int {
	useJSON := false
	includeAll := false
	var rest []string
	for _, a := range args {
		switch a {
		case "--json":
			useJSON = true
		case "--all":
			includeAll = true
		default:
			rest = append(rest, a)
		}
	}

	var explicit string
	if len(rest) > 0 {
		explicit = rest[0]
	}
	ticketDir, err := resolveDir(explicit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list: %v\n", err)
		return 1
	}

	tickets, _ := loadErgs(ticketDir)

	var entries []listEntry
	for i := range tickets {
		t := &tickets[i]
		closed := t.IsClosed()
		if closed && !includeAll {
			continue
		}
		var blockedBy []blockedByEntry
		for _, ref := range t.BlockedBys {
			if ref.Kind == RefInvalid {
				continue // malformed refs are validator territory
			}
			if ref.IsForge() {
				blockedBy = append(blockedBy, blockedByEntry{kind: "forge", ref: ref.Raw})
			} else {
				blockedBy = append(blockedBy, blockedByEntry{kind: "local", id: ref.ID})
			}
		}
		entries = append(entries, listEntry{
			id:        t.FilenameID(),
			title:     t.Title,
			file:      t.Filename(),
			closed:    closed,
			tags:      t.Tags,
			blockedBy: blockedBy,
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].id < entries[j].id
	})

	if useJSON {
		printListJSON(entries)
	} else {
		printListText(entries, includeAll)
	}
	return 0
}

// blockedByLabel renders a blocked-by entry for human-readable output: the
// ticket ID for local refs, the raw ref string for forge refs.
func blockedByLabel(b blockedByEntry) string {
	if b.kind == "forge" {
		return b.ref
	}
	return b.id
}

func printListText(entries []listEntry, includeAll bool) {
	noun := "Open tickets"
	if includeAll {
		noun = "Tickets"
	}
	if len(entries) == 0 {
		fmt.Printf("No %s found.\n", strings.ToLower(noun))
		return
	}
	fmt.Printf("%s (%d):\n", noun, len(entries))
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
