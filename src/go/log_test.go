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

// TestCmdLog_WhitespaceAuthorRefused is the regression guard for the defect
// review found in the first cut of this fix. `sanitizeAuthor` strips only \n
// and \r, so "   " survived it non-empty, never fell back to resolveAuthor(),
// and was written verbatim -- producing `<ts>     note something`, whose actor
// slot holds a verb. erg validate passed it, because logLineRE's `\s+\S+\s+\S+`
// backtracks across the run of spaces and still finds two tokens. The fix's own
// flag reintroduced the defect the fix exists to close.
func TestCmdLog_WhitespaceAuthorRefused(t *testing.T) {
	for _, v := range []string{"   ", "\t", " \t "} {
		dir := t.TempDir()
		writeTestTicket(t, dir, "7105-loggable.erg", logTicket)
		pinAuthor(t, "fallbackauthor")

		if rc := cmdLog([]string{"7105", "note something happened", dir, "--author", v}); rc == 0 {
			t.Errorf("cmdLog accepted --author %q, want refusal", v)
			line := lastLogLine(t, dir, "7105-loggable.erg")
			if f := strings.Fields(line); len(f) > 1 && f[1] != "fallbackauthor" {
				t.Errorf("  and it wrote %q -- actor slot is %q", line, f[1])
			}
		}
	}
}

// TestCmdLog_EmptyAuthorFlagRefused: `--author=` states an intent (use THIS
// author) that cannot be honoured. Falling back silently hands the caller a
// different author than the one requested, with no diagnostic -- so refuse,
// as new.go does.
func TestCmdLog_EmptyAuthorFlagRefused(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7106-loggable.erg", logTicket)
	pinAuthor(t, "fallbackauthor")

	if rc := cmdLog([]string{"7106", "note x", dir, "--author="}); rc == 0 {
		t.Error("cmdLog accepted --author= (explicit empty), want refusal")
	}
}

// TestCmdLog_MultiWordAuthorCollapsed: the actor slot is positional, so an
// author containing a space pushes the verb over by one and every reader
// mis-parses the entry. `git config user.name` is commonly "First Last", so
// this is the default configuration on many machines, not an edge case.
func TestCmdLog_MultiWordAuthorCollapsed(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7107-loggable.erg", logTicket)
	pinAuthor(t, "Minh Ha Duong")

	if rc := cmdLog([]string{"7107", "note the slot survives", dir}); rc != 0 {
		t.Fatalf("cmdLog returned %d, want 0", rc)
	}
	fields := strings.Fields(lastLogLine(t, dir, "7107-loggable.erg"))
	if fields[1] != "Minh-Ha-Duong" {
		t.Errorf("actor slot is %q, want %q", fields[1], "Minh-Ha-Duong")
	}
	if fields[2] != "note" {
		t.Errorf("verb slot is %q, want %q -- a multi-word author shifted the line", fields[2], "note")
	}
}

// TestCmdLog_MultiWordAuthorFlagCollapsed: same for the explicit flag.
func TestCmdLog_MultiWordAuthorFlagCollapsed(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7108-loggable.erg", logTicket)
	pinAuthor(t, "testuser")

	if rc := cmdLog([]string{"7108", "note x", dir, "--author", "John Doe"}); rc != 0 {
		t.Fatalf("cmdLog returned %d, want 0", rc)
	}
	fields := strings.Fields(lastLogLine(t, dir, "7108-loggable.erg"))
	if fields[1] != "John-Doe" || fields[2] != "note" {
		t.Errorf("got actor %q verb %q, want %q %q", fields[1], fields[2], "John-Doe", "note")
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
