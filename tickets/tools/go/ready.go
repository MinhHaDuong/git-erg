package main

// This file implements `erg ready`: list open tickets whose Blocked-by refs
// are all closed.

import (
	"fmt"
	"os"
	"strings"
)

type readyEntry struct {
	id, title, file string
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
	statusByID := make(map[string]string)
	for i := range tickets {
		id := tickets[i].FilenameID()
		if id != "" {
			statusByID[id] = tickets[i].Status()
		}
	}

	var warnings []string
	var ready []readyEntry
	openCount := 0

	for i := range tickets {
		t := &tickets[i]
		if t.Status() != "open" {
			continue
		}
		openCount++

		tid := t.FilenameID()
		blocked := false
		for _, refID := range t.BlockedBy() {
			if strings.HasPrefix(refID, "gh#") {
				continue // GitHub refs treated as satisfied offline
			}
			refStatus, found := statusByID[refID]
			if !found {
				warnings = append(warnings, fmt.Sprintf(
					"%s: Blocked-by '%s' not found (treating as satisfied)", t.Filename(), refID))
			} else if refStatus != "closed" {
				blocked = true
				break
			}
		}
		if !blocked {
			ready = append(ready, readyEntry{tid, t.Title(), t.Filename()})
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
				fmt.Printf("  {\n    \"id\": \"%s\",\n    \"title\": \"%s\",\n    \"file\": \"%s\"\n  }%s\n",
					jsonEscape(r.id), jsonEscape(r.title), jsonEscape(r.file), comma)
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
