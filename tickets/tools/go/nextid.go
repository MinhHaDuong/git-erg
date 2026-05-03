package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// cmdNextID implements `erg next-id [dir]`.
func cmdNextID(args []string) int {
	ticketDir := "tickets"
	if len(args) > 0 {
		ticketDir = args[0]
	}

	maxID := 0

	entries, err := os.ReadDir(ticketDir)
	if err != nil {
		fmt.Printf("%04d\n", 1)
		return 0
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".erg") {
			continue
		}
		stem := strings.TrimSuffix(e.Name(), ".erg")
		if idx := strings.Index(stem, "-"); idx > 0 {
			stem = stem[:idx]
		}
		if n, err := strconv.Atoi(stem); err == nil && n > maxID {
			maxID = n
		}
	}

	fmt.Printf("%04d\n", maxID+1)
	return 0
}
