package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// nextID scans dir for the highest numeric .erg filename prefix and returns
// the next ID as a zero-padded 4-digit string.  Returns "0001" when dir does
// not exist or contains no numbered tickets.
func nextID(dir string) string {
	maxID := 0

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "0001"
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".erg") {
			continue
		}
		stem := strings.TrimSuffix(name, ".erg")
		if idx := strings.Index(stem, "-"); idx > 0 {
			stem = stem[:idx]
		}
		if n, err := strconv.Atoi(stem); err == nil && n > maxID {
			maxID = n
		}
	}

	return fmt.Sprintf("%04d", maxID+1)
}

// cmdNextID implements `erg next-id [dir]`.
func cmdNextID(args []string) int {
	ticketDir := "tickets"
	if len(args) > 0 {
		ticketDir = args[0]
	}
	fmt.Println(nextID(ticketDir))
	return 0
}
