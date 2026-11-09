package main

import (
	"os"
	"strings"
	"testing"
)

func TestHeaderBlankWarnings(t *testing.T) {
	t.Run("clean corpus yields no warnings", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0001-clean.erg",
			"%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n")
		if w := headerBlankWarnings(dir); w != nil {
			t.Errorf("clean corpus -> %v, want nil", w)
		}
	})

	t.Run("interior header blank produces one warning naming the file", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0001-clean.erg",
			"%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n")
		// Blank line between Created and Author is interior (Author still parses
		// as a header line), so the header block is not yet terminated.
		writeErg(t, dir, "0002-blank.erg",
			"%erg 0.1\nTitle: T\nCreated: 2024-01-01\n\nAuthor: test\n\n--- log ---\n--- body ---\n")

		w := headerBlankWarnings(dir)
		if len(w) != 1 {
			t.Fatalf("got %d warnings, want 1: %v", len(w), w)
		}
		// Class guard: no user-facing warning should leak an internal absolute
		// path. The contract is basename-only; filepath.Base(path)->path is the
		// distinguishing mutation.
		if strings.Contains(w[0], string(os.PathSeparator)) {
			t.Errorf("warning leaks a path separator (absolute path in user-facing output): %q", w[0])
		}
		want := "WARN 0002-blank.erg: blank line inside header block"
		if !strings.HasPrefix(w[0], want) {
			t.Errorf("warning %q should have prefix %q", w[0], want)
		}
	})

	t.Run("warning shows basename even when file is in a subdirectory", func(t *testing.T) {
		// Place the blank .erg in a real nested subdir under dir and walk from
		// dir, so the parent->child traversal path is actually exercised.
		dir := t.TempDir()
		sub, err := os.MkdirTemp(dir, "sub")
		if err != nil {
			t.Fatalf("creating subdir: %v", err)
		}
		writeErg(t, sub, "0003-nested.erg",
			"%erg 0.1\nTitle: T\nCreated: 2024-01-01\n\nAuthor: test\n\n--- log ---\n--- body ---\n")
		w := headerBlankWarnings(dir)
		if len(w) != 1 {
			t.Fatalf("got %d warnings in subdir, want 1: %v", len(w), w)
		}
		if strings.Contains(w[0], string(os.PathSeparator)) {
			t.Errorf("warning from nested dir leaks path separator: %q", w[0])
		}
	})

	t.Run("non-.erg files are ignored even with interior header blanks", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0001-real.erg",
			"%erg 0.1\nTitle: T\nCreated: 2024-01-01\n\nAuthor: test\n\n--- log ---\n--- body ---\n")
		// A .txt file with erg-shaped content and an interior header blank.
		os.WriteFile(
			dir+"/notes.txt",
			[]byte("%erg 0.1\nTitle: T\nCreated: 2024-01-01\n\nAuthor: test\n\n--- log ---\n--- body ---\n"),
			0644,
		)
		w := headerBlankWarnings(dir)
		if len(w) != 1 {
			t.Fatalf("got %d warnings, want exactly 1 (the .erg file only): %v", len(w), w)
		}
		if !strings.Contains(w[0], "0001-real.erg") {
			t.Errorf("warning should name the .erg file, got: %q", w[0])
		}
	})
}

func TestLogPlacementWarnings(t *testing.T) {
	t.Run("well-formed log yields no warnings", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0001-clean.erg",
			"%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n"+
				"2024-01-01T10:00Z test created\n2024-01-01T11:00Z test note ok\n\n--- body ---\n")
		if w := logPlacementWarnings(dir); w != nil {
			t.Errorf("clean log -> %v, want nil", w)
		}
	})

	t.Run("displaced entry after the terminal blank warns once", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0002-displaced.erg",
			"%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n"+
				"2024-01-01T10:00Z test created\n\n2024-01-01T12:00Z test note displaced\n--- body ---\n")
		w := logPlacementWarnings(dir)
		if len(w) != 1 {
			t.Fatalf("got %d warnings, want 1: %v", len(w), w)
		}
		want := "WARN 0002-displaced.erg: log entry after the entry run's terminal blank"
		if !strings.HasPrefix(w[0], want) {
			t.Errorf("warning %q should have prefix %q", w[0], want)
		}
	})

	t.Run("regressive timestamps warn and stay advisory", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0003-regressive.erg",
			"%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n"+
				"2024-01-02T10:00Z test created\n2024-01-01T09:00Z test note replayed\n\n--- body ---\n")
		w := logPlacementWarnings(dir)
		if len(w) != 1 {
			t.Fatalf("got %d warnings, want 1: %v", len(w), w)
		}
		want := "WARN 0003-regressive.erg: regressive log timestamps"
		if !strings.HasPrefix(w[0], want) {
			t.Errorf("warning %q should have prefix %q", w[0], want)
		}
	})

	t.Run("displaced and regressive on one file warn in that order", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0004-both.erg",
			"%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n"+
				"2024-01-02T10:00Z test created\n\n2024-01-01T09:00Z test note displaced\n--- body ---\n")
		w := logPlacementWarnings(dir)
		if len(w) != 2 {
			t.Fatalf("got %d warnings, want 2: %v", len(w), w)
		}
		if !strings.HasPrefix(w[0], "WARN 0004-both.erg: log entry after") ||
			!strings.HasPrefix(w[1], "WARN 0004-both.erg: regressive log timestamps") {
			t.Errorf("unexpected warnings: %v", w)
		}
	})

	t.Run("continuation lines do not count as entries", func(t *testing.T) {
		dir := t.TempDir()
		// A folded detail line after the terminal blank, with no entry-
		// format line below the blank, draws no placement warning.
		writeErg(t, dir, "0005-cont.erg",
			"%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n"+
				"2024-01-01T10:00Z test created\n\nstill the created entry\n\n--- body ---\n")
		if w := logPlacementWarnings(dir); w != nil {
			t.Errorf("continuation-only after blank -> %v, want nil", w)
		}
	})

	t.Run("equal consecutive timestamps are not regressive", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0006-equal.erg",
			"%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n"+
				"2024-01-01T10:00Z test created\n2024-01-01T10:00Z test note same minute\n\n--- body ---\n")
		if w := logPlacementWarnings(dir); w != nil {
			t.Errorf("equal timestamps -> %v, want nil", w)
		}
	})
}

// TestCheckFailsWithoutClaimsDir: claims are required (ticket 0307). A store
// without .claims/ fails with the one message that names the repair; the
// same store passes once it is claimed (control).
func TestCheckFailsWithoutClaimsDir(t *testing.T) {
	dir := t.TempDir()
	writeClaimTestTicket(t, dir, "0001-a.erg")
	var code int
	stdout, stderr := captureOutput(t, func() { code = cmdCheck([]string{dir}) })
	out := stdout + stderr
	if code != 1 {
		t.Errorf("erg check on a store without .claims/ = %d, want 1", code)
	}
	if !strings.Contains(out, "run: erg migrate --claims "+dir) {
		t.Errorf("message does not name the repair: %s", out)
	}
	claimStore(t, dir)
	stdout, stderr = captureOutput(t, func() { code = cmdCheck([]string{dir}) })
	out = stdout + stderr
	if code != 0 {
		t.Errorf("control: erg check on the claimed store = %d, want 0: %s", code, out)
	}
}

// TestCheckDuplicateHintSaysRenumber: two tickets sharing a number get one
// line, the duplicate, telling the user to renumber -- not the plain-file
// claim fix, which written for both tickets of a pair would create a fresh
// conflict (ticket 0307). Both with no claim for the number and with a claim
// naming one of the two.
func TestCheckDuplicateHintSaysRenumber(t *testing.T) {
	for _, claimed := range []bool{false, true} {
		dir := t.TempDir()
		writeClaimTestTicket(t, dir, "0001-a.erg")
		writeClaimTestTicket(t, dir, "0001-b.erg")
		writeTestClaim(t, dir, claimsKeepName, "")
		if claimed {
			writeTestClaim(t, dir, "0001", "a 0123abcd\n")
		}
		var code int
		stdout, stderr := captureOutput(t, func() { code = cmdCheck([]string{dir}) })
		var renumber, echo int
		for _, line := range strings.Split(stdout+stderr, "\n") {
			if strings.Contains(line, "renumber") {
				renumber++
			}
			if strings.Contains(line, `echo "`) {
				echo++
			}
		}
		if code != 1 || renumber != 1 || echo != 0 {
			t.Errorf("claimed=%v: rc=%d, %d renumber lines (want 1), %d echo lines (want 0):\n%s%s",
				claimed, code, renumber, echo, stdout, stderr)
		}
	}
}
