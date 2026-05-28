package main

import (
	"fmt"
	"os"
	"sort"
)

// summaryReady is the one-liner printed by printUsage via the commands registry.
const summaryReady = "Show tickets ready for work"

const helpReady = `## erg ready [DIR] [--json]

List tickets ready for work — a saved filter over 'erg list'.

A ticket is ready when all of the following hold:

  - Open (not closed).
  - Not blocked: no Blocked-by pointing at an open local ticket, and no
    forge-ref Blocked-by (forge refs are offline-unknown, treated as
    blocking).
  - Carries none of the skip tags (default: needs-human, deferred;
    configurable via tickets/.ergrc [tags]).

Equivalent to 'erg list open not blocked not needs-human not deferred', and
shares its output: a human-readable line per ticket, or --json for a JSON
array with the fields id, title, file, closed, refs, tags, blocked_by.

Each line is annotated with the comma-separated [refs] — git branch short
names, remote-tracking branch short names (with their <remote>/ prefix),
and worktree paths — that reference the ticket per spec-erg-v1.md. The scan
is local-only; PRs and forge state are out of scope (pep-erg-v1.md §7).
`

// cmdReady implements `erg ready [dir] [--json]` as a thin alias over the
// shared list engine: open, not blocked, and free of the configured skip
// tags. See helpReady.
func cmdReady(args []string) int {
	useJSON := false
	var explicit string
	for _, a := range args {
		if a == "--json" {
			useJSON = true
		} else if explicit == "" {
			explicit = a
		}
	}

	ticketDir, err := resolveDir(explicit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ready: %v\n", err)
		return 1
	}

	cfg, cfgErr := loadConfig(ticketDir)
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "ready: cannot read .ergrc: %v\n", cfgErr)
		return 1
	}

	f := filter{positive: []string{"open"}, negative: []string{"blocked"}}
	for tag := range effectiveTagSet(cfg) {
		f.negative = append(f.negative, tag)
	}
	sort.Strings(f.negative)

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
	} else {
		printListText("Ready tickets", matched)
	}
	return 0
}
