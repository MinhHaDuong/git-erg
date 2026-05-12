package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDir(t *testing.T) {
	t.Run("explicit directory", func(t *testing.T) {
		tmp := t.TempDir()
		got, err := resolveDir(tmp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != filepath.Clean(tmp) {
			t.Errorf("got %q, want %q", got, filepath.Clean(tmp))
		}
	})

	t.Run("explicit not a directory", func(t *testing.T) {
		tmp := t.TempDir()
		f := filepath.Join(tmp, "file.txt")
		os.WriteFile(f, []byte("x"), 0644)
		_, err := resolveDir(f)
		if err == nil || !strings.Contains(err.Error(), "not a directory") {
			t.Fatalf("expected 'not a directory' error, got: %v", err)
		}
	})

	t.Run("explicit missing", func(t *testing.T) {
		_, err := resolveDir("/no/such/path/xyzzy")
		if err == nil {
			t.Fatal("expected error for missing directory")
		}
	})
}

func TestResolveTicketByID(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "0042-fix-bug.erg"), []byte("%erg 0.1\n"), 0644)

	t.Run("happy path", func(t *testing.T) {
		got, err := resolveTicketByID(tmp, "0042")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(tmp, "0042-fix-bug.erg")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("no match", func(t *testing.T) {
		_, err := resolveTicketByID(tmp, "9999")
		if err == nil || !strings.Contains(err.Error(), "no ticket found") {
			t.Fatalf("expected 'no ticket found' error, got: %v", err)
		}
	})

	t.Run("ambiguous", func(t *testing.T) {
		ambiguous := t.TempDir()
		os.WriteFile(filepath.Join(ambiguous, "0042-fix-bug.erg"), []byte("%erg 0.1\n"), 0644)
		os.WriteFile(filepath.Join(ambiguous, "0042-other.erg"), []byte("%erg 0.1\n"), 0644)
		_, err := resolveTicketByID(ambiguous, "0042")
		if err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("expected 'ambiguous' error, got: %v", err)
		}
	})
}
