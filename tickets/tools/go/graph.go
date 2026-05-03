// `erg graph` — visualize the ticket dependency DAG.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// cmdGraph implements `erg graph [dir] [--json]`.
func cmdGraph(args []string) int {
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
	if len(tickets) == 0 {
		fmt.Println("No tickets found.")
		return 0
	}

	// Build lookup maps
	byID := make(map[string]*Erg)
	statusByID := make(map[string]string)
	for i := range tickets {
		id := tickets[i].FilenameID()
		if id != "" {
			byID[id] = &tickets[i]
			statusByID[id] = tickets[i].Status()
		}
	}

	// Build children map (reverse of Blocked-by): if B is blocked by A, then A -> B
	children := make(map[string][]string)
	hasParent := make(map[string]bool)
	for i := range tickets {
		id := tickets[i].FilenameID()
		for _, ref := range tickets[i].BlockedBy() {
			if strings.HasPrefix(ref, "gh#") {
				continue
			}
			if _, exists := byID[ref]; exists {
				children[ref] = append(children[ref], id)
				hasParent[id] = true
			}
		}
	}

	// Sort children for deterministic output
	for k := range children {
		sort.Strings(children[k])
	}

	// Determine annotation for a ticket
	annotate := func(id string) string {
		status := statusByID[id]
		if status == "open" {
			// Check if blocked
			t := byID[id]
			for _, ref := range t.BlockedBy() {
				if strings.HasPrefix(ref, "gh#") {
					continue
				}
				if s, ok := statusByID[ref]; ok && s != "closed" {
					return "open, blocked"
				}
			}
			return "open, READY"
		}
		return status
	}

	// Find root nodes (no parent in the DAG)
	var roots []string
	for i := range tickets {
		id := tickets[i].FilenameID()
		if id != "" && !hasParent[id] {
			roots = append(roots, id)
		}
	}
	sort.Strings(roots)

	if useJSON {
		type jsonNode struct {
			id, title, status, annotation string
			blockedBy                     []string
			deps                          []string
		}
		var nodes []jsonNode
		for i := range tickets {
			id := tickets[i].FilenameID()
			n := jsonNode{
				id:         id,
				title:      tickets[i].Title(),
				status:     tickets[i].Status(),
				annotation: annotate(id),
				blockedBy:  tickets[i].BlockedBy(),
				deps:       children[id],
			}
			nodes = append(nodes, n)
		}
		fmt.Println("[")
		for i, n := range nodes {
			comma := ","
			if i == len(nodes)-1 {
				comma = ""
			}
			blockedByJSON := "[]"
			if len(n.blockedBy) > 0 {
				var parts []string
				for _, b := range n.blockedBy {
					parts = append(parts, fmt.Sprintf("\"%s\"", jsonEscape(b)))
				}
				blockedByJSON = "[" + strings.Join(parts, ", ") + "]"
			}
			depsJSON := "[]"
			if len(n.deps) > 0 {
				var parts []string
				for _, d := range n.deps {
					parts = append(parts, fmt.Sprintf("\"%s\"", jsonEscape(d)))
				}
				depsJSON = "[" + strings.Join(parts, ", ") + "]"
			}
			fmt.Printf("  {\n    \"id\": \"%s\",\n    \"title\": \"%s\",\n    \"status\": \"%s\",\n    \"annotation\": \"%s\",\n    \"blocked_by\": %s,\n    \"unblocks\": %s\n  }%s\n",
				jsonEscape(n.id), jsonEscape(n.title), jsonEscape(n.status), jsonEscape(n.annotation), blockedByJSON, depsJSON, comma)
		}
		fmt.Println("]")
		return 0
	}

	// ASCII tree output
	var printTree func(id string, prefix string, isLast bool)
	printTree = func(id string, prefix string, isLast bool) {
		t := byID[id]
		if t == nil {
			return
		}
		ann := annotate(id)
		connector := "-> "
		if prefix == "" {
			connector = ""
		}
		fmt.Printf("%s%s%s %s [%s]\n", prefix, connector, id, t.Title(), ann)

		kids := children[id]
		childPrefix := prefix
		if prefix != "" {
			if isLast {
				childPrefix = prefix[:len(prefix)-3] + "   "
			} else {
				childPrefix = prefix[:len(prefix)-3] + "|  "
			}
		}
		for i, kid := range kids {
			last := i == len(kids)-1
			if childPrefix == "" {
				printTree(kid, "   ", last)
			} else {
				printTree(kid, childPrefix+"   ", last)
			}
		}
	}

	for _, root := range roots {
		printTree(root, "", true)
	}

	return 0
}
