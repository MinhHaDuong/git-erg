package main

import (
	"io"
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

// captureOutput runs fn with os.Stdout and os.Stderr redirected to temporary
// files, and returns the contents of each. This is the test-local equivalent of
// withDiscardedStdout in resource_test.go, but for capture rather than discard.
func captureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	outFile, err := os.CreateTemp("", "capture-stdout-*")
	if err != nil {
		t.Fatalf("captureOutput: create stdout temp: %v", err)
	}
	defer os.Remove(outFile.Name())
	defer outFile.Close()

	errFile, err := os.CreateTemp("", "capture-stderr-*")
	if err != nil {
		t.Fatalf("captureOutput: create stderr temp: %v", err)
	}
	defer os.Remove(errFile.Name())
	defer errFile.Close()

	savedOut, savedErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outFile, errFile
	defer func() { os.Stdout, os.Stderr = savedOut, savedErr }()

	fn()

	os.Stdout, os.Stderr = savedOut, savedErr

	if _, err := outFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("captureOutput: seek stdout: %v", err)
	}
	outBytes, err := io.ReadAll(outFile)
	if err != nil {
		t.Fatalf("captureOutput: read stdout: %v", err)
	}

	if _, err := errFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("captureOutput: seek stderr: %v", err)
	}
	errBytes, err := io.ReadAll(errFile)
	if err != nil {
		t.Fatalf("captureOutput: read stderr: %v", err)
	}

	return string(outBytes), string(errBytes)
}

func TestResolveDirBanner(t *testing.T) {
	t.Run("emits branch banner on git repo", func(t *testing.T) {
		gitOrSkip(t)
		repo := t.TempDir()
		initRepoWithCommit(t, repo)

		_, stderr := captureOutput(t, func() {
			if _, err := resolveDir(repo); err != nil {
				t.Fatalf("resolveDir: %v", err)
			}
		})

		if !strings.Contains(stderr, "erg: branch ") {
			t.Errorf("expected banner on stderr, got: %q", stderr)
		}
		if !strings.Contains(stderr, "(") {
			t.Errorf("expected display path in banner, got: %q", stderr)
		}
	})

	t.Run("no banner on non-repo dir", func(t *testing.T) {
		dir := t.TempDir()

		_, stderr := captureOutput(t, func() {
			if _, err := resolveDir(dir); err != nil {
				t.Fatalf("resolveDir: %v", err)
			}
		})

		if stderr != "" {
			t.Errorf("expected empty stderr for non-repo dir, got: %q", stderr)
		}
	})

	t.Run("cmdList --json stdout clean", func(t *testing.T) {
		gitOrSkip(t)
		repo := t.TempDir()
		initRepoWithCommit(t, repo)
		// Write one ticket so the store is valid (looksLikeTicketStore requires .erg file
		// or "tickets" basename; repo dir has neither - use explicit dir with a ticket).
		ticketDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(ticketDir, "0001-test.erg"), []byte("%erg 0.1\nTitle: Test\nCreated: 2026-01-01\nAuthor: a\n\n--- log ---\n--- body ---\n"), 0644); err != nil {
			t.Fatal(err)
		}
		// Run git init in ticketDir so gitBranch returns a branch name,
		// ensuring the banner is actually generated (non-vacuous).
		gitRun(t, ticketDir, "init", "-q", "-b", "main")
		gitRun(t, ticketDir, "config", "user.email", "test@example.com")
		gitRun(t, ticketDir, "config", "user.name", "Test")
		gitRun(t, ticketDir, "config", "commit.gpgsign", "false")
		gitRun(t, ticketDir, "add", "0001-test.erg")
		gitRun(t, ticketDir, "commit", "-q", "-m", "init")

		stdout, stderr := captureOutput(t, func() {
			cmdList([]string{"--json", ticketDir})
		})

		// Banner must appear on stderr (confirms it was generated).
		if !strings.Contains(stderr, "erg: branch ") {
			t.Errorf("expected banner on stderr, got stderr: %q", stderr)
		}
		// Banner must NOT appear on stdout.
		if strings.Contains(stdout, "erg: branch") {
			t.Errorf("banner leaked to stdout: %q", stdout)
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
