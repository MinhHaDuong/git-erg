package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type blockedByEntry struct {
	kind string
	id   string
	ref  string
}

type readyEntry struct {
	id, title, file string
	tags            []string
	ready           bool
	claimed         bool
	blockedBy       []blockedByEntry
}

var skipReadyTags = map[string]bool{
	"needs-human":     true,
	"deferred":        true,
	"post-talk":       true,
	"post-conference": true,
}

// loadBranchNames returns all local and remote branch names in one git
// invocation. Remote names carry a "remotes/" prefix which is harmless
// for substring matching. Failure (no repo, no remote) returns nil.
func loadBranchNames() []string {
	cmd := exec.Command("git", "branch", "-a")
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		// Strip leading marker characters (* current branch, + worktree-active branch)
		t := strings.TrimSpace(line)
		if len(t) >= 2 && (t[0] == '*' || t[0] == '+') && t[1] == ' ' {
			t = t[2:]
		}
		// Skip symref lines (e.g. "remotes/origin/HEAD -> origin/main")
		if t == "" || strings.Contains(t, " -> ") {
			continue
		}
		names = append(names, t)
	}
	return names
}

// isBranchClaimed reports whether any branch name in the pre-loaded list
// contains the given ticket ID.
func isBranchClaimed(id string, branches []string) bool {
	for _, b := range branches {
		if strings.Contains(b, id) {
			return true
		}
	}
	return false
}

func printReadyJSON(entries []readyEntry) {
	type blockedByJSON struct {
		Kind string `json:"kind"`
		ID   string `json:"id,omitempty"`
		Ref  string `json:"ref,omitempty"`
	}
	type entryJSON struct {
		ID        string          `json:"id"`
		Title     string          `json:"title"`
		File      string          `json:"file"`
		Ready     bool            `json:"ready"`
		Claimed   bool            `json:"claimed"`
		Tags      []string        `json:"tags"`
		BlockedBy []blockedByJSON `json:"blocked_by"`
	}

	result := make([]entryJSON, len(entries))
	for i, r := range entries {
		bb := make([]blockedByJSON, len(r.blockedBy))
		for j, b := range r.blockedBy {
			bb[j] = blockedByJSON{Kind: b.kind, ID: b.id, Ref: b.ref}
		}
		tags := r.tags
		if tags == nil {
			tags = []string{}
		}
		result[i] = entryJSON{
			ID: r.id, Title: r.title, File: r.file,
			Ready: r.ready, Claimed: r.claimed,
			Tags: tags, BlockedBy: bb,
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ready: JSON marshal error: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func printReadyText(totalCount int, openEntries, ready []readyEntry) {
	openCount := len(openEntries)
	if len(ready) == 0 {
		if totalCount == 0 {
			fmt.Println("No tickets found.")
		} else if openCount == 0 {
			fmt.Printf("All %d tickets are closed.\n", totalCount)
		} else {
			fmt.Printf("%d open tickets, all blocked.\n", openCount)
		}
	} else {
		fmt.Printf("Ready tickets (%d):\n", len(ready))
		for _, r := range ready {
			tagsSuffix := ""
			if len(r.tags) > 0 {
				tagsSuffix = fmt.Sprintf(" (tags: %s)", strings.Join(r.tags, ", "))
			}
			fmt.Printf("  %-8s %-40s %s%s\n", r.id, r.file, r.title, tagsSuffix)
		}
	}

	var claimedEntries []readyEntry
	for _, e := range openEntries {
		if e.claimed {
			claimedEntries = append(claimedEntries, e)
		}
	}
	if len(claimedEntries) > 0 {
		fmt.Printf("\nClaimed tickets (%d):\n", len(claimedEntries))
		for _, r := range claimedEntries {
			tagsSuffix := ""
			if len(r.tags) > 0 {
				tagsSuffix = fmt.Sprintf(" (tags: %s)", strings.Join(r.tags, ", "))
			}
			fmt.Printf("  %-8s %-40s %s%s\n", r.id, r.file, r.title, tagsSuffix)
		}
	}
}

// summaryReady is the one-liner printed by printUsage via the commands registry.
const summaryReady = "Show tickets ready for work"

const helpReady = `## erg ready [DIR] [--json]

List tickets ready for work.

A ticket is ready when all of the following hold:

  - Not closed (no Closed: header and not in a closed/ directory).
  - No Blocked-by headers pointing to open local tickets.
  - No forge-ref Blocked-by lines (forge refs are offline-unknown, treated as blocking).
  - No tags from the skip set: needs-human, deferred, post-talk, post-conference.

Tickets that pass the readiness test but have a git branch containing the
ticket ID are reported as "claimed" (shown separately, not in the ready list).

Without --json, prints a human-readable summary. With --json, prints a JSON
array where each element has the fields: id, title, file, ready, claimed, tags, blocked_by.

The JSON output covers all open tickets (not just ready ones), so callers can
filter and sort by any field. ready=true implies blocked_by is empty.
`

// cmdReady implements `erg ready [dir] [--json]`. See helpReady for the user-facing summary.
func cmdReady(args []string) int {
	useJSON := false
	var rest []string
	for _, a := range args {
		if a == "--json" {
			useJSON = true
		} else {
			rest = append(rest, a)
		}
	}

	ticketDir, err := findTicketsDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(rest) > 0 {
		ticketDir = rest[0]
	}

	info, err := os.Stat(ticketDir)
	if err != nil || !info.IsDir() {
		fmt.Printf("Directory not found: %s\n", ticketDir)
		return 1
	}

	tickets, _ := loadErgs(ticketDir)
	closedByID := make(map[string]bool)
	knownID := make(map[string]bool)
	for i := range tickets {
		id := tickets[i].FilenameID()
		if id != "" {
			closedByID[id] = tickets[i].IsClosed()
			knownID[id] = true
		}
	}

	var branches []string
	branchesLoaded := false

	var warnings []string
	var openEntries []readyEntry
	var ready []readyEntry
	openCount := 0

	for i := range tickets {
		t := &tickets[i]
		if t.IsClosed() {
			continue
		}
		openCount++

		tid := t.FilenameID()
		tags := t.Tags
		blocked := false
		var blockedBy []blockedByEntry
		for _, tag := range tags {
			if skipReadyTags[tag] {
				blocked = true
				break
			}
		}

		if !blocked {
			for _, ref := range t.BlockedBys {
				if ref.Kind == RefInvalid {
					continue // malformed refs are validator territory
				}
				if ref.IsForge() {
					// Forge refs are offline-unknown → blocking.
					blocked = true
					blockedBy = append(blockedBy, blockedByEntry{kind: "forge", ref: ref.Raw})
					break
				}
				if !knownID[ref.ID] {
					warnings = append(warnings, fmt.Sprintf(
						"%s: Blocked-by '%s' not found (treating as satisfied)", t.Filename(), ref.ID))
				} else if !closedByID[ref.ID] {
					blocked = true
					blockedBy = append(blockedBy, blockedByEntry{kind: "local", id: ref.ID})
					break
				}
			}
		}

		claimed := false
		if !blocked {
			if !branchesLoaded {
				branches = loadBranchNames()
				branchesLoaded = true
			}
			claimed = isBranchClaimed(tid, branches)
			if claimed {
				blocked = true
			}
		}

		entry := readyEntry{tid, t.Title, t.Filename(), tags, !blocked, claimed, blockedBy}
		openEntries = append(openEntries, entry)
		if !blocked {
			ready = append(ready, entry)
		}
	}

	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "WARNING: %s\n", w)
	}

	if useJSON {
		printReadyJSON(openEntries)
	} else {
		printReadyText(len(tickets), openEntries, ready)
	}
	return 0
}
