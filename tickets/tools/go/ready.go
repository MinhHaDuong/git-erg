package main

import (
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
		if len(openEntries) == 0 {
			fmt.Println("[]")
		} else {
			fmt.Println("[")
			for i, r := range openEntries {
				comma := ","
				if i == len(openEntries)-1 {
					comma = ""
				}

				readyJSON := "false"
				if r.ready {
					readyJSON = "true"
				}

				claimedJSON := "false"
				if r.claimed {
					claimedJSON = "true"
				}

				tagJSON := "[]"
				if len(r.tags) > 0 {
					tagJSON = "["
					for j, tag := range r.tags {
						if j > 0 {
							tagJSON += ", "
						}
						tagJSON += fmt.Sprintf("\"%s\"", jsonEscape(tag))
					}
					tagJSON += "]"
				}

				blockedByJSON := "[]"
				if len(r.blockedBy) > 0 {
					blockedByJSON = "["
					for j, b := range r.blockedBy {
						if j > 0 {
							blockedByJSON += ", "
						}
						if b.kind == "forge" {
							blockedByJSON += fmt.Sprintf("{\"kind\": \"forge\", \"ref\": \"%s\"}", jsonEscape(b.ref))
						} else {
							blockedByJSON += fmt.Sprintf("{\"kind\": \"local\", \"id\": \"%s\"}", jsonEscape(b.id))
						}
					}
					blockedByJSON += "]"
				}

				fmt.Printf("  {\n    \"id\": \"%s\",\n    \"title\": \"%s\",\n    \"file\": \"%s\",\n    \"ready\": %s,\n    \"claimed\": %s,\n    \"tags\": %s,\n    \"blocked_by\": %s\n  }%s\n",
					jsonEscape(r.id), jsonEscape(r.title), jsonEscape(r.file), readyJSON, claimedJSON, tagJSON, blockedByJSON, comma)
			}
			fmt.Println("]")
		}
	} else {
		if len(ready) == 0 {
			if len(tickets) == 0 {
				fmt.Println("No tickets found.")
			} else if openCount == 0 {
				fmt.Printf("All %d tickets are closed.\n", len(tickets))
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
	return 0
}
