package main

import (
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
		if !strings.Contains(w[0], "0002-blank.erg") {
			t.Errorf("warning %q should name the offending file", w[0])
		}
		if !strings.Contains(w[0], "blank line inside header block") {
			t.Errorf("warning %q should describe the interior blank", w[0])
		}
	})
}
