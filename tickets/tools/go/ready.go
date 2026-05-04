package main

import (
	"fmt"
	"os"
)

type readyEntry struct {
	id, title, file string
	tags            []string
}

var skipReadyTags = map[string]bool{
	"needs-human":     true,
	"deferred":        true,
	"post-talk":       true,
	"post-conference": true,
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
					break
				}
				if !knownID[ref.ID] {
					warnings = append(warnings, fmt.Sprintf(
						"%s: Blocked-by '%s' not found (treating as satisfied)", t.Filename(), ref.ID))
				} else if !closedByID[ref.ID] {
					blocked = true
					break
				}
			}
		}
		if !blocked {
			ready = append(ready, readyEntry{tid, t.Title(), t.Filename(), tags})
		}
	}

	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "WARNING: %s\n", w)
	}

	if useJSON {
		if len(ready) == 0 {
			fmt.Println("[]")
		} else {
			fmt.Println("[")
			for i, r := range ready {
				comma := ","
				if i == len(ready)-1 {
					comma = ""
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
				fmt.Printf("  {\n    \"id\": \"%s\",\n    \"title\": \"%s\",\n    \"file\": \"%s\",\n    \"tags\": %s\n  }%s\n",
					jsonEscape(r.id), jsonEscape(r.title), jsonEscape(r.file), tagJSON, comma)
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
				fmt.Printf("  %-8s %-40s %s\n", r.id, r.file, r.title)
			}
		}
	}
	return 0
}
