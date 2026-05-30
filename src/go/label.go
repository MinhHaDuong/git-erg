package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// summaryLabel is the one-liner printed by printUsage via the commands registry.
const summaryLabel = "Add a label to a ticket"

const helpLabel = `## erg label ID LABELNAME [DIR]

Add a Label: header to the ticket's preamble and append a log line.

The label value must be in the project vocabulary (tickets/.ergrc [labels]
section; default: needs-human, deferred). If the ticket already has the
label, prints "LABELED (already)" and exits 0 without modifying the file.

Exits non-zero if the label is not in the vocabulary or the ticket is not found.
`

// summaryUnlabel is the one-liner printed by printUsage via the commands registry.
const summaryUnlabel = "Remove a label from a ticket"

const helpUnlabel = `## erg unlabel ID LABELNAME [DIR]

Remove a Label: header from the ticket's preamble and append a log line.

The label value must be in the project vocabulary. If the ticket does not
have the label, prints "NOT LABELED" and exits 0 without modifying the file.

Exits non-zero if the label is not in the vocabulary or the ticket is not found.
`

// cmdLabel implements `erg label ID LABELNAME [DIR]`. See helpLabel for the user-facing summary.
func cmdLabel(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: erg label ID LABELNAME [DIR]")
		return 1
	}

	id := args[0]
	labelname := args[1]
	var explicit string
	if len(args) >= 3 {
		explicit = args[2]
	}

	ticketDir, err := resolveDir(explicit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "label: %v\n", err)
		return 1
	}

	cfg, err := loadConfig(ticketDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "label: %v\n", err)
		return 1
	}
	labelSet := effectiveLabelSet(cfg)
	if !labelSet[labelname] {
		fmt.Fprintf(os.Stderr, "label: unknown label %q (not in vocabulary)\n", labelname)
		return 1
	}

	ticketPath, err := resolveTicketByID(ticketDir, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "label: %v\n", err)
		return 1
	}

	data, err := os.ReadFile(ticketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "label: cannot read %s: %v\n", ticketPath, err)
		return 1
	}

	ticket, _ := parseErgBytes(data, ticketPath)

	// Idempotent: already labeled — exit 0 without modifying the file.
	for _, l := range ticket.Labels {
		if l == labelname {
			fmt.Println("LABELED (already)")
			return 0
		}
	}

	content, err := insertClosedHeader(string(data), "Label: "+labelname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "label: %v\n", err)
		return 1
	}

	now := time.Now().UTC().Format("2006-01-02T15:04Z")
	author := resolveAuthor()
	logLine := fmt.Sprintf("%s %s label %s", now, author, labelname)
	content = appendLogLine(content, logLine)
	content = string(collapseHeaderBlanks([]byte(content)))

	if err := writeTicketAtomic(ticketDir, ticketPath, []byte(content)); err != nil {
		fmt.Fprintf(os.Stderr, "label: cannot write %s: %v\n", ticketPath, err)
		return 1
	}

	fmt.Println("LABELED")
	return 0
}

// cmdUnlabel implements `erg unlabel ID LABELNAME [DIR]`. See helpUnlabel for the user-facing summary.
func cmdUnlabel(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: erg unlabel ID LABELNAME [DIR]")
		return 1
	}

	id := args[0]
	labelname := args[1]
	var explicit string
	if len(args) >= 3 {
		explicit = args[2]
	}

	ticketDir, err := resolveDir(explicit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unlabel: %v\n", err)
		return 1
	}

	cfg, err := loadConfig(ticketDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unlabel: %v\n", err)
		return 1
	}
	labelSet := effectiveLabelSet(cfg)
	if !labelSet[labelname] {
		fmt.Fprintf(os.Stderr, "unlabel: unknown label %q (not in vocabulary)\n", labelname)
		return 1
	}

	ticketPath, err := resolveTicketByID(ticketDir, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unlabel: %v\n", err)
		return 1
	}

	data, err := os.ReadFile(ticketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unlabel: cannot read %s: %v\n", ticketPath, err)
		return 1
	}

	ticket, _ := parseErgBytes(data, ticketPath)

	// Idempotent: label not present — exit 0 without modifying the file.
	found := false
	for _, l := range ticket.Labels {
		if l == labelname {
			found = true
			break
		}
	}
	if !found {
		fmt.Println("NOT LABELED")
		return 0
	}

	content := removeLabelLine(string(data), labelname)

	now := time.Now().UTC().Format("2006-01-02T15:04Z")
	author := resolveAuthor()
	logLine := fmt.Sprintf("%s %s unlabel %s", now, author, labelname)
	content = appendLogLine(content, logLine)
	content = string(collapseHeaderBlanks([]byte(content)))

	if err := writeTicketAtomic(ticketDir, ticketPath, []byte(content)); err != nil {
		fmt.Fprintf(os.Stderr, "unlabel: cannot write %s: %v\n", ticketPath, err)
		return 1
	}

	fmt.Println("UNLABELED")
	return 0
}

// removeLabelLine removes Label header lines whose value equals label. It parses
// each line with parseHeaderLine — the same parser that populated Erg.Labels for
// cmdUnlabel's idempotency check — so detection ("ticket has this label") and
// removal ("strip this Label line") agree on every tolerated spelling
// (e.g. "Label : deferred" with whitespace before the colon). Matching the full
// trimmed value also means unlabeling "deferred" never strikes a different
// "Label: deferred-soon" line.
func removeLabelLine(content, label string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if key, val, ok := parseHeaderLine(line); ok && key == "Label" && val == label {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
