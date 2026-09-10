package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// logTicket is the fixture every test below appends to.
const logTicket = `%erg 0.1
Title: Loggable ticket
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
`

// lastLogLine returns the final line of the ticket's log section -- the one
// cmdLog just appended.
func lastLogLine(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	logPart, _, found := strings.Cut(string(data), separatorBody)
	if !found {
		t.Fatalf("no %s separator in %s", separatorBody, name)
	}
	var last string
	for _, l := range strings.Split(logPart, "\n") {
		if strings.TrimSpace(l) != "" && logLineRE.MatchString(strings.TrimSpace(l)) {
			last = strings.TrimSpace(l)
		}
	}
	if last == "" {
		t.Fatalf("no log line found in %s", name)
	}
	return last
}

func pinAuthor(t *testing.T, name string) {
	t.Helper()
	old := gitConfigUserName
	gitConfigUserName = func() string { return "" }
	t.Cleanup(func() { gitConfigUserName = old })
	t.Setenv("ERG_AUTHOR", name)
}

// TestCmdLog_SuppliesActor is the red step for ticket 0276. Before the fix,
// cmdLog wrote LINE verbatim after the timestamp, so `erg log ID "note ..."`
// produced `<ts> note ...` -- the actor slot held a verb and every downstream
// reader silently mis-parsed the entry. logLineRE cannot catch this: it asks
// only for two tokens after the timestamp, and `note corrected` supplies two.
//
// The assertion is therefore positional, not shape-based: token 1 after the
// timestamp must be the resolved author.
func TestCmdLog_SuppliesActor(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7100-loggable.erg", logTicket)
	pinAuthor(t, "testuser")

	if rc := cmdLog([]string{"7100", "note corrected the sweep", dir}); rc != 0 {
		t.Fatalf("cmdLog returned %d, want 0", rc)
	}

	line := lastLogLine(t, dir, "7100-loggable.erg")
	fields := strings.Fields(line)
	if len(fields) < 3 {
		t.Fatalf("log line %q has %d fields, want at least 3", line, len(fields))
	}
	if fields[1] != "testuser" {
		t.Errorf("actor slot is %q, want %q -- full line: %q", fields[1], "testuser", line)
	}
	if fields[2] != "note" {
		t.Errorf("verb slot is %q, want %q -- full line: %q", fields[2], "note", line)
	}
}

// TestCmdLog_AuthorFlagOverrides covers the override the ticket calls for,
// spelled as `erg new` already spells it.
func TestCmdLog_AuthorFlagOverrides(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7101-loggable.erg", logTicket)
	pinAuthor(t, "testuser")

	if rc := cmdLog([]string{"7101", "note from a bot", dir, "--author", "bump"}); rc != 0 {
		t.Fatalf("cmdLog returned %d, want 0", rc)
	}

	fields := strings.Fields(lastLogLine(t, dir, "7101-loggable.erg"))
	if fields[1] != "bump" {
		t.Errorf("actor slot is %q, want %q", fields[1], "bump")
	}
}

// TestCmdLog_SingleTokenLineIsEnough: with the actor supplied, LINE carries
// only VERB [detail], so a bare verb is now a complete entry. Before the fix
// this was refused for having fewer than two tokens.
func TestCmdLog_SingleTokenLineIsEnough(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7102-loggable.erg", logTicket)
	pinAuthor(t, "testuser")

	if rc := cmdLog([]string{"7102", "reopened", dir}); rc != 0 {
		t.Fatalf("cmdLog returned %d, want 0", rc)
	}

	fields := strings.Fields(lastLogLine(t, dir, "7102-loggable.erg"))
	if fields[1] != "testuser" || fields[2] != "reopened" {
		t.Errorf("got actor %q verb %q, want %q %q", fields[1], fields[2], "testuser", "reopened")
	}
}

// TestCmdLog_EmptyLineRefused: the actor is supplied, but a verb is not, so an
// empty LINE must still be refused rather than writing a line with an actor and
// nothing else.
func TestCmdLog_EmptyLineRefused(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7103-loggable.erg", logTicket)
	pinAuthor(t, "testuser")

	if rc := cmdLog([]string{"7103", "   ", dir}); rc == 0 {
		t.Error("cmdLog accepted a blank LINE, want refusal")
	}
}

// TestCmdLog_WrittenLineValidates guards the invariant the code comments claim:
// a state-altering command must never write a line the validator rejects.
func TestCmdLog_WrittenLineValidates(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7104-loggable.erg", logTicket)
	pinAuthor(t, "testuser")

	if rc := cmdLog([]string{"7104", "note something", dir}); rc != 0 {
		t.Fatalf("cmdLog returned %d, want 0", rc)
	}
	line := lastLogLine(t, dir, "7104-loggable.erg")
	if !logLineRE.MatchString(line) {
		t.Errorf("cmdLog wrote a line its own validator rejects: %q", line)
	}
}
