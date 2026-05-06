package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// cmdMigrate implements `erg migrate [dir]`.
//
// Idempotent. For every .erg file under dir (default: tickets/):
//   - `Status: closed` (any case) → drop the line and append
//     `Closed: migrated from Status: closed` to the preamble.
//   - `Status: open|doing|pending` (any case) → drop the line; the
//     ticket becomes not-closed (which is the correct new state).
//   - No `Status:` line → no-op.
//
// Does NOT commit. Always exits 0; running twice is safe.
func cmdMigrate(args []string) int {
	var dir string
	if len(args) > 0 {
		dir = args[0]
	} else {
		var err error
		dir, err = findTicketsDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "migrate: directory not found: %s\n", dir)
		return 1
	}

	migratedClosed := 0
	migratedOther := 0
	alreadyClean := 0

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".erg") {
			return nil
		}
		changed, wasClosed, mErr := migrateFile(path)
		if mErr != nil {
			fmt.Fprintf(os.Stderr, "migrate: %s: %v\n", path, mErr)
			return nil
		}
		switch {
		case !changed:
			alreadyClean++
		case wasClosed:
			migratedClosed++
		default:
			migratedOther++
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate: walk error: %v\n", err)
		return 1
	}

	total := migratedClosed + migratedOther
	fmt.Printf("migrated: %d tickets (%d closed, %d open/doing/pending stripped)\n",
		total, migratedClosed, migratedOther)
	fmt.Printf("already clean: %d tickets\n", alreadyClean)
	return 0
}

// migrateFile rewrites a single .erg file in place. Returns:
//   - changed: whether the file was modified.
//   - wasClosed: whether at least one removed Status: line carried the
//     value "closed" (in which case a Closed: header is appended).
func migrateFile(path string) (changed bool, wasClosed bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, false, err
	}
	original := string(data)
	// Preserve trailing-newline state when splitting/rejoining.
	hadTrailingNewline := strings.HasSuffix(original, "\n")
	lines := strings.Split(original, "\n")
	if hadTrailingNewline {
		lines = lines[:len(lines)-1]
	}

	// Bound preamble at the first `--- log ---` separator.
	logIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "--- log ---" {
			logIdx = i
			break
		}
	}
	preambleEnd := len(lines)
	if logIdx >= 0 {
		preambleEnd = logIdx
	}

	out := make([]string, 0, len(lines))
	foundStatus := false
	foundClosed := false

	for i, line := range lines {
		if i < preambleEnd && isStatusHeaderLine(line) {
			foundStatus = true
			val := strings.TrimSpace(line[len("Status:"):])
			if strings.EqualFold(val, "closed") {
				foundClosed = true
			}
			continue
		}
		out = append(out, line)
	}

	if !foundStatus {
		return false, false, nil
	}

	if foundClosed {
		// After we've removed Status: lines, the preamble end shifts by
		// however many we dropped — recompute relative to the rewritten
		// slice. Insert the Closed header immediately after the last
		// non-blank preamble line.
		newLogIdx := -1
		for i, line := range out {
			if strings.TrimSpace(line) == "--- log ---" {
				newLogIdx = i
				break
			}
		}
		insertAt := len(out)
		if newLogIdx >= 0 {
			insertAt = newLogIdx
		}
		for insertAt > 0 && strings.TrimSpace(out[insertAt-1]) == "" {
			insertAt--
		}
		header := "Closed: migrated from Status: closed"
		out = append(out[:insertAt], append([]string{header}, out[insertAt:]...)...)
	}

	rejoined := strings.Join(out, "\n")
	if hadTrailingNewline {
		rejoined += "\n"
	}
	if rejoined == original {
		return false, false, nil
	}
	if err := os.WriteFile(path, []byte(rejoined), 0644); err != nil {
		return false, false, err
	}
	return true, foundClosed, nil
}

// isStatusHeaderLine reports whether a line begins with the literal
// `Status:` header key (case-insensitive on the key itself, since some
// tickets in the wild may carry quirky casing).
func isStatusHeaderLine(line string) bool {
	const key = "Status:"
	if len(line) < len(key) {
		return false
	}
	return strings.EqualFold(line[:len(key)], key)
}

// hasStatusHeader scans dir for any .erg file containing a `Status:` line
// in the preamble. Used by `erg update` to decide whether to print
// migration guidance after a binary swap.
func hasStatusHeader(dir string) bool {
	stopWalk := fmt.Errorf("found")
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".erg") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if !strings.Contains(string(data), "Status:") {
			return nil
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == "--- log ---" {
				return nil
			}
			if isStatusHeaderLine(line) {
				return stopWalk
			}
		}
		return nil
	})
	return err == stopWalk
}
