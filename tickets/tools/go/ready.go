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

// isBranchClaimed reports whether any local or remote git branch name
// contains the given 4-digit ticket ID. Remote branch check is best-effort:
// if it fails (no remote, no network), the ticket is treated as unclaimed.
func isBranchClaimed(id string) bool {
	pattern := "*" + id + "*"
	// Local branches
	cmd := exec.Command("git", "branch", "--list", pattern)
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return true
	}
	// Remote branches (best-effort — skip on error)
	cmd = exec.Command("git", "branch", "-r", "--list", pattern)
	cmd.Stderr = io.Discard
	out, err = cmd.Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return true
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

func printReadyText(totalCount, openCount int, openEntries, ready []readyEntry) {
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

// cmdReady implements `erg ready [dir] [--json]`.
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

	ticketDir := "tickets"
	if len(rest) > 0 {
		ticketDir = rest[0]
	}

	info, err := os.Stat(ticketDir)
	if err != nil || !info.IsDir() {
		fmt.Printf("Directory not found: %s\n", ticketDir)
		return 1
	}

	tickets := loadErgs(ticketDir)
	closedByID := make(map[string]bool)
	knownID := make(map[string]bool)
	for i := range tickets {
		id := tickets[i].FilenameID()
		if id != "" {
			closedByID[id] = tickets[i].Closed()
			knownID[id] = true
		}
	}

	var warnings []string
	var openEntries []readyEntry
	var ready []readyEntry
	openCount := 0

	for i := range tickets {
		t := &tickets[i]
		if t.Closed() {
			continue
		}
		openCount++

		tid := t.FilenameID()
		tags := t.Tags()
		blocked := false
		var blockedBy []blockedByEntry
		for _, tag := range tags {
			if skipReadyTags[tag] {
				blocked = true
				break
			}
		}

		if !blocked {
			refs, errs := t.BlockedByRefs()
			for i, ref := range refs {
				if errs[i] != nil {
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
			claimed = isBranchClaimed(tid)
			if claimed {
				blocked = true
			}
		}

		entry := readyEntry{tid, t.Title(), t.Filename(), tags, !blocked, claimed, blockedBy}
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
		printReadyText(len(tickets), openCount, openEntries, ready)
	}
	return 0
}
