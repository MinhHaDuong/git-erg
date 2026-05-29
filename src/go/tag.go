package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// summaryTag is the one-liner printed by printUsage via the commands registry.
const summaryTag = "Add a tag to a ticket"

const helpTag = `## erg tag ID TAGNAME [DIR]

Add a Tag: header to the ticket's preamble and append a log line.

The tag value must be in the project vocabulary (tickets/.ergrc [tags]
section; default: needs-human, deferred). If the ticket already has the
tag, prints "TAGGED (already)" and exits 0 without modifying the file.

Exits non-zero if the tag is not in the vocabulary or the ticket is not found.
`

// summaryUntag is the one-liner printed by printUsage via the commands registry.
const summaryUntag = "Remove a tag from a ticket"

const helpUntag = `## erg untag ID TAGNAME [DIR]

Remove a Tag: header from the ticket's preamble and append a log line.

The tag value must be in the project vocabulary. If the ticket does not
have the tag, prints "NOT TAGGED" and exits 0 without modifying the file.

Exits non-zero if the tag is not in the vocabulary or the ticket is not found.
`

// cmdTag implements `erg tag ID TAGNAME [DIR]`. See helpTag for the user-facing summary.
func cmdTag(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: erg tag ID TAGNAME [DIR]")
		return 1
	}

	id := args[0]
	tagname := args[1]
	var explicit string
	if len(args) >= 3 {
		explicit = args[2]
	}

	ticketDir, err := resolveDir(explicit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tag: %v\n", err)
		return 1
	}

	cfg, err := loadConfig(ticketDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tag: %v\n", err)
		return 1
	}
	tagSet := effectiveTagSet(cfg)
	if !tagSet[tagname] {
		fmt.Fprintf(os.Stderr, "tag: unknown tag %q (not in vocabulary)\n", tagname)
		return 1
	}

	ticketPath, err := resolveTicketByID(ticketDir, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tag: %v\n", err)
		return 1
	}

	data, err := os.ReadFile(ticketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tag: cannot read %s: %v\n", ticketPath, err)
		return 1
	}

	ticket, _ := parseErgBytes(data, ticketPath)

	// Idempotent: already tagged — exit 0 without modifying the file.
	for _, t := range ticket.Tags {
		if t == tagname {
			fmt.Println("TAGGED (already)")
			return 0
		}
	}

	content, err := insertClosedHeader(string(data), "Tag: "+tagname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tag: %v\n", err)
		return 1
	}

	now := time.Now().UTC().Format("2006-01-02T15:04Z")
	author := resolveAuthor()
	logLine := fmt.Sprintf("%s %s tag %s", now, author, tagname)
	content = appendLogLine(content, logLine)
	content = string(collapseHeaderBlanks([]byte(content)))

	if err := os.WriteFile(ticketPath, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "tag: cannot write %s: %v\n", ticketPath, err)
		return 1
	}

	fmt.Println("TAGGED")
	return 0
}

// cmdUntag implements `erg untag ID TAGNAME [DIR]`. See helpUntag for the user-facing summary.
func cmdUntag(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: erg untag ID TAGNAME [DIR]")
		return 1
	}

	id := args[0]
	tagname := args[1]
	var explicit string
	if len(args) >= 3 {
		explicit = args[2]
	}

	ticketDir, err := resolveDir(explicit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "untag: %v\n", err)
		return 1
	}

	cfg, err := loadConfig(ticketDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "untag: %v\n", err)
		return 1
	}
	tagSet := effectiveTagSet(cfg)
	if !tagSet[tagname] {
		fmt.Fprintf(os.Stderr, "untag: unknown tag %q (not in vocabulary)\n", tagname)
		return 1
	}

	ticketPath, err := resolveTicketByID(ticketDir, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "untag: %v\n", err)
		return 1
	}

	data, err := os.ReadFile(ticketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "untag: cannot read %s: %v\n", ticketPath, err)
		return 1
	}

	ticket, _ := parseErgBytes(data, ticketPath)

	// Idempotent: tag not present — exit 0 without modifying the file.
	found := false
	for _, t := range ticket.Tags {
		if t == tagname {
			found = true
			break
		}
	}
	if !found {
		fmt.Println("NOT TAGGED")
		return 0
	}

	content := removeTagLine(string(data), tagname)

	now := time.Now().UTC().Format("2006-01-02T15:04Z")
	author := resolveAuthor()
	logLine := fmt.Sprintf("%s %s untag %s", now, author, tagname)
	content = appendLogLine(content, logLine)
	content = string(collapseHeaderBlanks([]byte(content)))

	if err := os.WriteFile(ticketPath, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "untag: cannot write %s: %v\n", ticketPath, err)
		return 1
	}

	fmt.Println("UNTAGGED")
	return 0
}

// removeTagLine removes Tag header lines whose value equals tag. It parses
// each line with parseHeaderLine — the same parser that populated Erg.Tags for
// cmdUntag's idempotency check — so detection ("ticket has this tag") and
// removal ("strip this Tag line") agree on every tolerated spelling
// (e.g. "Tag : deferred" with whitespace before the colon). Matching the full
// trimmed value also means untagging "deferred" never strikes a different
// "Tag: deferred-soon" line.
func removeTagLine(content, tag string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if key, val, ok := parseHeaderLine(line); ok && key == "Tag" && val == tag {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
