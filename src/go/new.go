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
to 40 characters).

Uses an optimistic post-check retry loop to handle concurrent invocations:
O_EXCL writes the file, then a glob for NNNN-*.erg verifies uniqueness of the
NNNN prefix. If a collision is detected (two concurrent invocations computed the
same ID for different slugs), the losing invocation removes its file and retries
with the next free ID. Up to 20 attempts are made before giving up.

The new file contains the required preamble headers (Title, Created, Author),
an empty log section with a "created" entry, and an empty body section.
Author is resolved from the ERG_AUTHOR environment variable, or the git user.name,
or the system username -- whichever is available first.

Prints 'CREATED NNNN-slug.erg' on success. Exits non-zero on exhaustion or I/O errors.
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
	// message function the validator uses -- no duplicated wording.
	if msg, bad := titleStatusWordMessage(title); bad {
		fmt.Fprintf(os.Stderr, "new: %s\n", msg)
		return 1
	}

	var ticketDir string
	if len(args) >= 2 {
		// Explicit DIR is an intentional escape hatch, NOT confined.
		//
		// The other mutating commands (close/log/label/rm) treat an explicit DIR
		// as the trusted store: resolveDir accepts any directory the caller
		// names. `new` creates that store (MkdirAll), so confining it against a
		// "discovered" or cwd store is ill-defined -- for `new` the named DIR *is*
		// the store. 0149's withinStore is a fat-finger guard, not a security
		// boundary; here the directory is precisely what the user fat-fingered or
		// chose, so there is nothing to guard against. Confining against cwd would
		// also break the legitimate absolute-DIR form every caller already relies
		// on (e.g. `erg new TITLE /path/to/tickets`, and the test suites). So an
		// explicit DIR -- relative subdir or absolute -- is honoured verbatim. The
		// unconfined surface needs attacker-controlled CLI args, not a committed
		// .erg, so the parser-input attack surface is unaffected.
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

	// author is constant across retries; resolve it once outside the loop.
	author := resolveAuthor()
	slug := slugify(title)

	const maxAttempts = 20
	for attempt := 0; attempt < maxAttempts; attempt++ {
		id, err := nextID(ticketDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}

		filename := fmt.Sprintf("%s-%s.erg", id, slug)
		path := filepath.Join(ticketDir, filename)

		now := time.Now().UTC()
		today := now.Format("2006-01-02")
		timestamp := now.Format("2006-01-02T15:04Z")
		content := fmt.Sprintf("%%erg 0.1\nTitle: %s\nCreated: %s\nAuthor: %s\n\n%s\n%s %s created\n\n%s\n", title, today, author, separatorLog, timestamp, author, separatorBody)

		if err := createExclusive(path, content); err != nil {
			if !os.IsExist(err) {
				// Real I/O error (permission denied, disk full, etc.) -- surface immediately.
				fmt.Fprintf(os.Stderr, "new: cannot create %s: %v\n", path, err)
				return 1
			}
			// EEXIST: same slug + same ID claimed by a concurrent invocation; retry.
			continue
		}

		// Optimistic post-check: glob for any NNNN-*.erg siblings. If more
		// than one exists, a concurrent invocation with a different slug won
		// the same ID. Delete our file and retry with the next free ID.
		pattern := filepath.Join(ticketDir, id+"-*.erg")
		siblings, err := filepath.Glob(pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "new: cannot scan store for ID conflicts: %v\n", err)
			os.Remove(path)
			return 1
		}
		if len(siblings) > 1 {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "new: cannot remove conflicting file %s: %v\n", path, err)
				return 1
			}
			continue
		}

		// Unique win: we are the sole holder of this NNNN prefix.
		fmt.Printf("CREATED %s\n", filename)
		return 0
	}

	fmt.Fprintf(os.Stderr, "new: could not allocate a unique ID after %d attempts (concurrent contention)\n", maxAttempts)
	return 1
}

// createExclusive writes content to a brand-new file at path, refusing to
// touch an existing one. O_EXCL is the no-clobber guard: it never overwrites
// an existing file at the exact same path (same ID + same slug). It does not
// guard against two concurrent `erg new` invocations that computed the same
// numeric ID but used different slugs -- that case is handled by cmdNew's
// optimistic post-check glob loop, which detects NNNN-prefix collisions after
// the O_EXCL write and retries with the next free ID.
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
