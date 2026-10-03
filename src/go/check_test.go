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
