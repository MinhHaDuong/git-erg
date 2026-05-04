package main

import (
	"fmt"
	"os"
	"path/filepath"
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

	err := filepath.Walk(ticketDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(path, ".erg") {
			return nil
		}
		name := filepath.Base(path)
		stem := strings.TrimSuffix(name, ".erg")
		if idx := strings.Index(stem, "-"); idx > 0 {
			stem = stem[:idx]
		}
		if n, err := strconv.Atoi(stem); err == nil && n > maxID {
			maxID = n
		}
		return nil
	})
	if err != nil {
		fmt.Printf("%04d\n", 1)
		return 0
	}

	fmt.Printf("%04d\n", maxID+1)
	return 0
}
