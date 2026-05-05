package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)
)

// slugify converts a title to a lowercase ASCII kebab-case slug, truncated to
// 40 characters.
func slugify(title string) string {
	s := strings.ToLower(title)
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = s[:40]
		s = strings.TrimRight(s, "-")
	}
	if s == "" {
		s = "untitled"
	}
	return s
}

// cmdNew implements `erg new <title> [dir]`.
func cmdNew(args []string) int {
	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(os.Stderr, "Usage: erg new TITLE [DIR]")
		fmt.Fprintln(os.Stderr, "  title: non-empty string (required)")
		return 1
	}

	title := args[0]
	ticketDir := "tickets"
	if len(args) >= 2 {
		ticketDir = args[1]
	}

	if err := os.MkdirAll(ticketDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "new: cannot create directory %s: %v\n", ticketDir, err)
		return 1
	}

	id := nextID(ticketDir)
	slug := slugify(title)
	filename := fmt.Sprintf("%s-%s.erg", id, slug)
	path := filepath.Join(ticketDir, filename)

	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	timestamp := now.Format("2006-01-02T15:04Z")

	author := resolveAuthor()
	content := fmt.Sprintf("%%erg v1\nTitle: %s\nCreated: %s\nAuthor: %s\n\n--- log ---\n%s %s created\n\n--- body ---\n", title, today, author, timestamp, author)

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new: cannot create %s: %v\n", path, err)
		return 1
	}
	if _, err := fmt.Fprint(f, content); err != nil {
		f.Close()
		fmt.Fprintf(os.Stderr, "new: cannot write %s: %v\n", path, err)
		return 1
	}
	if err := f.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "new: cannot close %s: %v\n", path, err)
		return 1
	}

	fmt.Printf("CREATED %s\n", filename)
	return 0
}
