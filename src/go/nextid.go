package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// parseIDFromFilename extracts the leading numeric prefix from an .erg
// filename (e.g. "0042-some-title.erg" → 42). Returns 0 if the file does not
// end in .erg or the prefix is not numeric.
func parseIDFromFilename(name string) int {
	if !strings.HasSuffix(name, ".erg") {
		return 0
	}
	stem := strings.TrimSuffix(name, ".erg")
	if idx := strings.Index(stem, "-"); idx > 0 {
		stem = stem[:idx]
	}
	n, err := strconv.Atoi(stem)
	if err != nil {
		return 0
	}
	return n
}

// nextID scans dir and all its subdirectories for the highest numeric .erg
// filename prefix and returns the next ID as a zero-padded 4-digit string.
// Returns "0001" when dir does not exist or contains no numbered tickets.
func nextID(dir string) string {
	maxID := 0

	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if n := parseIDFromFilename(d.Name()); n > maxID {
			maxID = n
		}
		return nil
	})

	return fmt.Sprintf("%04d", maxID+1)
}

// cmdNextID implements `erg next-id [dir]`.
func cmdNextID(args []string) int {
	var ticketDir string
	if len(args) > 0 {
		ticketDir = args[0]
	} else {
		var err error
		ticketDir, err = findTicketsDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	fmt.Println(nextID(ticketDir))
	return 0
}
