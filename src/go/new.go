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

// summaryNew is the one-liner printed by printUsage via the commands registry.
const summaryNew = "Create a new ticket file atomically"

const helpNew = `## erg new TITLE [DIR]

Create a new %erg 0.1 ticket file atomically.

Allocates the next available ID by scanning DIR (default: auto-discovered tickets/)
for the highest numeric .erg filename prefix, then creates a file named
NNNN-{slug}.erg where the slug is the title lowercased and kebab-cased (truncated
to 40 characters). Uses O_EXCL to prevent races with concurrent invocations.

The new file contains the required preamble headers (Title, Created, Author),
an empty log section with a "created" entry, and an empty body section.
Author is resolved from the ERG_AUTHOR environment variable, or the git user.name,
or the system username — whichever is available first.

Prints 'CREATED NNNN-slug.erg' on success. Exits non-zero on I/O errors.
`

// cmdNew implements `erg new TITLE [DIR]`. See helpNew for the user-facing summary.
func cmdNew(args []string) int {
	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(os.Stderr, "Usage: erg new TITLE [DIR]")
		fmt.Fprintln(os.Stderr, "  title: non-empty string (required)")
		return 1
	}

	title := args[0]

	// Rule 14 applies to open + new tickets: refuse to create a ticket whose
	// Title begins or ends with a status word, otherwise `erg new` would emit
	// a file that the very next `erg validate`/`erg check` rejects. Same
	// message function the validator uses — no duplicated wording.
	if msg, bad := titleStatusWordMessage(title); bad {
		fmt.Fprintf(os.Stderr, "new: %s\n", msg)
		return 1
	}

	var ticketDir string
	if len(args) >= 2 {
		ticketDir = filepath.Clean(args[1])
	} else {
		var err error
		ticketDir, err = findTicketsDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
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
	content := fmt.Sprintf("%%erg 0.1\nTitle: %s\nCreated: %s\nAuthor: %s\n\n%s\n%s %s created\n\n%s\n", title, today, author, separatorLog, timestamp, author, separatorBody)

	if err := createExclusive(path, content); err != nil {
		fmt.Fprintf(os.Stderr, "new: cannot create %s: %v\n", path, err)
		return 1
	}

	fmt.Printf("CREATED %s\n", filename)
	return 0
}

// createExclusive writes content to a brand-new file at path, refusing to
// touch an existing one. O_EXCL is the no-clobber guard: it never overwrites an
// existing ID (a collision returns an error rather than truncating the file
// already there), and it closes the race window between two concurrent `erg
// new`/`next-id` invocations that computed the same ID.
func createExclusive(path, content string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(f, content); err != nil {
		f.Close()
		// O_EXCL just created this file; a failed write would otherwise leave a
		// truncated/empty ticket on disk. Remove it so a failed `erg new` leaves
		// no partial artifact behind.
		os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}
