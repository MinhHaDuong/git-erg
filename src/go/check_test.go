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
		// dir, so the parent→child traversal path is actually exercised.
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
}
