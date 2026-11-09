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
// 40 characters, never yielding a slug that would make the generated filename
// read as a closed ticket (ticket 0256).
func slugify(title string) string {
	s := strings.ToLower(title)
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = s[:40]
		s = strings.TrimRight(s, "-")
	}
	s = stripClosedSuffix(s)
	if s == "" {
		s = "untitled"
	}
	return s
}

// stripClosedSuffix repeatedly drops the trailing hyphen-segment of s as long
// as a generated "NNNN-"+s basename would trip pathIsClosed's path component
// test (spec-erg-v1.md "Closed / not-closed criterion"). Truncating the slug
// to 40 characters can land the cut squarely on a "-closed" segment, which
// makes the whole toolchain treat a brand-new ticket as already closed --
// observed live on ticket 0255, which vanished from erg list and erg ready
// the moment it was created.
//
// Only the trailing "-closed" branch is reachable from a generated slug: the
// numeric ID prefix forecloses the equality and "closed-"/"closed." prefix
// branches. The loop calls the real predicate rather than hardcoding that
// suffix, so it stays correct if pathIsClosed's rule set ever changes.
func stripClosedSuffix(s string) string {
	for s != "" && pathIsClosed("0000-"+s+".erg") {
		idx := strings.LastIndex(s, "-")
		if idx < 0 {
			return ""
		}
		s = strings.TrimRight(s[:idx], "-")
	}
	return s
}

// summaryNew is the one-liner printed by printUsage via the commands registry.
const summaryNew = "Create a new ticket file atomically"

const helpNew = `## erg new TITLE [DIR] [--author NAME]

Create a new %erg 0.1 ticket file atomically.

Allocates the next available ID by scanning DIR (default: auto-discovered tickets/)
for the highest numeric .erg filename prefix, then creates a file named
NNNN-{slug}.erg where the slug is the title lowercased and kebab-cased (truncated
to 40 characters).

ID claims are required (tickets 0306, 0307): new first creates
DIR/.claims/NNNN, one line holding the slug, a space and a
random 8-hex-digit nonce, then the ticket. The claim is published with an
exclusive create (a hard link from a temp file, which fails like O_EXCL when
the name exists), so two invocations racing for one number in the same
checkout cannot both take it: the loser recomputes the next ID and retries
(up to 20 attempts). The ticket file itself is created with O_EXCL, so an
existing file is never clobbered. Two branches that take the same number add
the same claim path with different content, so git reports an add/add
conflict when the second one is merged or rebased. Stage both files: the
'git add' line is printed on stderr. new refuses when git ignores the claim
path, since the claim would then never be committed; the refusal names the
ignoring rule (file:line:pattern) and the exact lines to append to that file. new refuses, writing
nothing, in a store without .claims/; the message names the repair:
'erg migrate --claims DIR'. new never creates the store: 'erg init' does.

The new file contains the required preamble headers (Title, Created, Author),
an empty log section with a "created" entry, and an empty body section.

  --author NAME, -a NAME
      Override the Author header with NAME. If not given, author is resolved
      from the ERG_AUTHOR environment variable, or the git user.name, or the
      system username -- whichever is available first. NAME may not be empty
      or whitespace-only. Newlines and carriage returns are stripped.

Prints 'CREATED NNNN-slug.erg' on success. Exits non-zero on exhaustion or I/O errors.
`

// cmdNew implements `erg new TITLE [DIR] [--author NAME]`. See helpNew for the
// user-facing summary.
func cmdNew(args []string) int {
	var authorFlag string
	var authorSet bool
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--author" || a == "-a":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "new: --author requires a value")
				return 1
			}
			i++
			authorFlag = args[i]
			authorSet = true
		case strings.HasPrefix(a, "--author="):
			authorFlag = strings.TrimPrefix(a, "--author=")
			authorSet = true
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "new: unknown flag %q\nUsage: erg new TITLE [DIR] [--author NAME]\n", a)
			return 1
		default:
			positional = append(positional, a)
		}
	}

	if len(positional) < 1 || strings.TrimSpace(positional[0]) == "" {
		fmt.Fprintln(os.Stderr, "Usage: erg new TITLE [DIR] [--author NAME]")
		fmt.Fprintln(os.Stderr, "  title: non-empty string (required)")
		return 1
	}

	title := positional[0]

	// Rule 14 applies to open + new tickets: refuse to create a ticket whose
	// Title begins or ends with a status word, otherwise `erg new` would emit
	// a file that the very next `erg validate`/`erg check` rejects. Same
	// message function the validator uses -- no duplicated wording.
	if msg, bad := titleStatusWordMessage(title); bad {
		fmt.Fprintf(os.Stderr, "new: %s\n", msg)
		return 1
	}

	var ticketDir string
	if len(positional) >= 2 {
		// Explicit DIR is an intentional escape hatch, NOT confined.
		//
		// The other mutating commands (close/log/label/rm) treat an explicit DIR
		// as the trusted store: resolveDir accepts any directory the caller
		// names. For `new` too the named DIR *is* the store (it must hold its
		// .claims/, checked below), so confining it against a "discovered" or
		// cwd store is ill-defined. 0149's withinStore is a fat-finger guard, not a security
		// boundary; here the directory is precisely what the user fat-fingered or
		// chose, so there is nothing to guard against. Confining against cwd would
		// also break the legitimate absolute-DIR form every caller already relies
		// on (e.g. `erg new TITLE /path/to/tickets`, and the test suites). So an
		// explicit DIR -- relative subdir or absolute -- is honoured verbatim. The
		// unconfined surface needs attacker-controlled CLI args, not a committed
		// .erg, so the parser-input attack surface is unaffected.
		ticketDir = filepath.Clean(positional[1])
	} else {
		var err error
		ticketDir, err = findTicketsDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	// Before anything is created: a store without .claims/ is noncompliant,
	// and its repair is migrate, which claims the tickets already there.
	if err := requireClaims(ticketDir); err != nil {
		fmt.Fprintf(os.Stderr, "new: %v\n", err)
		return 1
	}

	// author is constant across retries; resolve it once outside the loop.
	var author string
	if authorSet {
		author = sanitizeAuthor(authorFlag)
		if strings.TrimSpace(author) == "" {
			fmt.Fprintln(os.Stderr, "new: --author value cannot be empty or whitespace-only")
			return 1
		}
	} else {
		author = resolveAuthor()
	}
	slug := slugify(title)

	const maxAttempts = 20
	for attempt := 0; attempt < maxAttempts; attempt++ {
		id, err := allocateID(ticketDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}

		filename := fmt.Sprintf("%s-%s.erg", id, slug)
		path := filepath.Join(ticketDir, filename)

		// Probe the real claim path: a claim git ignores would never
		// be committed, while erg check passes on the file on disk.
		if rules := ignoredClaimRules(ticketDir, id); len(rules) > 0 {
			for _, r := range rules {
				fmt.Fprintf(os.Stderr, "new: %s\n", r.message())
			}
			return 1
		}
		// The claim is the exclusive create: its path depends on the
		// number alone, so EEXIST is exactly "this number is taken".
		nonce, err := newNonce()
		if err != nil {
			fmt.Fprintf(os.Stderr, "new: %v\n", err)
			return 1
		}
		if err := writeClaim(ticketDir, id, slug, nonce); err != nil {
			if os.IsExist(err) {
				continue
			}
			fmt.Fprintf(os.Stderr, "new: cannot create claim %s: %v\n", filepath.Join(claimsDir(ticketDir), id), err)
			return 1
		}

		now := time.Now().UTC()
		today := now.Format("2006-01-02")
		timestamp := now.Format("2006-01-02T15:04Z")
		content := fmt.Sprintf("%%erg 0.1\nTitle: %s\nCreated: %s\nAuthor: %s\n\n%s\n%s %s created\n\n%s\n", title, today, author, separatorLog, timestamp, author, separatorBody)

		if err := createExclusive(path, content); err != nil {
			// The claim made the number ours, so an existing file at this
			// path is not a race to retry; like any I/O error it is reported.
			// A claim left without its ticket only burns the number.
			fmt.Fprintf(os.Stderr, "new: cannot create %s: %v\n", path, err)
			return 1
		}

		fmt.Printf("CREATED %s\n", filename)
		cwd, _ := os.Getwd()
		fmt.Fprintf(os.Stderr, "new: stage both files: git add %s %s\n",
			displayPath(cwd, path), displayPath(cwd, filepath.Join(claimsDir(ticketDir), id)))
		return 0
	}

	fmt.Fprintf(os.Stderr, "new: could not allocate a unique ID after %d attempts (concurrent contention)\n", maxAttempts)
	return 1
}

// allocateID is nextID behind a seam, so a test can make two creations
// compute the same number and race for it.
var allocateID = nextID

// createExclusive writes content to a brand-new file at path, refusing to
// touch an existing one. O_EXCL is the no-clobber guard: it never overwrites
// an existing file at the exact same path (same ID + same slug). Two
// invocations that computed the same number under different slugs are kept
// apart by the claim (writeClaim), whose path depends on the number alone.
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
