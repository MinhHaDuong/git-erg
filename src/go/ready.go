package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// summaryReady is the one-liner printed by printUsage via the commands registry.
const summaryReady = "Show tickets ready for work"

const helpReady = `## erg ready [DIR] [--json]

List tickets ready for work -- a saved filter over 'erg list'.

A ticket is ready when all of the following hold:

  - Open (not closed).
  - Not blocked: no Blocked-by pointing at an open local ticket, and no
    forge-ref Blocked-by (forge refs are offline-unknown, treated as
    blocking).
  - Carries none of the skip labels (default: needs-human, deferred;
    configurable via tickets/.ergrc [labels]).

Equivalent to 'erg list open not blocked' with every configured label
(.ergrc [labels]; default: needs-human, deferred) also negated. Shares
its output: a human-readable line per ticket, or --json for a JSON array
with the fields id, title, file, closed, refs, labels, blocked_by.

Each line is annotated with the comma-separated [refs] -- git branch short
names, remote-tracking branch short names (with their <remote>/ prefix),
and worktree paths -- that reference the ticket per spec-erg-v1.md. The scan
is local-only; PRs and forge state are out of scope (pep-erg-v1.md sec.7).
`

// cmdReady implements `erg ready [dir] [--json]` as a thin alias over the
// shared list engine: open, not blocked, and free of the configured skip
// labels. See helpReady.
func cmdReady(args []string) int {
	useJSON := false
	var explicit string
	for _, a := range args {
		switch {
		case a == "--json":
			useJSON = true
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "ready: unknown flag %q\nUsage: erg ready [DIR] [--json]\n", a)
			return 1
		case explicit == "":
			explicit = a
		default:
			fmt.Fprintf(os.Stderr, "ready: unexpected argument %q (usage: erg ready [DIR] [--json])\n", a)
			return 1
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
	for label := range effectiveLabelSet(cfg) {
		f.negative = append(f.negative, label)
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
