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

	t.Run("cmdReady emits branch banner", func(t *testing.T) {
		gitOrSkip(t)
		dir := t.TempDir()
		initRepoWithCommit(t, dir)
		// Place an open ticket so the store is valid and resolveDir fires the banner.
		if err := os.WriteFile(filepath.Join(dir, "9000-fixture.erg"), []byte("%erg 0.1\nTitle: Fixture\nCreated: 2026-01-01\nAuthor: a\n\n--- log ---\n--- body ---\n"), 0644); err != nil {
			t.Fatal(err)
		}

		_, stderr := captureOutput(t, func() {
			cmdReady([]string{dir})
		})

		if !strings.Contains(stderr, "erg: branch ") {
			t.Errorf("expected branch banner on stderr, got: %q", stderr)
		}
	})

	t.Run("cmdClose emits branch banner", func(t *testing.T) {
		gitOrSkip(t)
		dir := t.TempDir()
		initRepoWithCommit(t, dir)
		// Place a ticket with ID 9001 so the store is valid; we close a
		// non-existent ID 9000 so the lookup fails, but resolveDir fires
		// the banner before the lookup.
		if err := os.WriteFile(filepath.Join(dir, "9001-fixture.erg"), []byte("%erg 0.1\nTitle: Fixture\nCreated: 2026-01-01\nAuthor: a\n\n--- log ---\n--- body ---\n"), 0644); err != nil {
			t.Fatal(err)
		}

		_, stderr := captureOutput(t, func() {
			cmdClose([]string{"9000", "reason", dir})
		})

		if !strings.Contains(stderr, "erg: branch ") {
			t.Errorf("expected branch banner on stderr, got: %q", stderr)
		}
	})
}

func TestStoreWorktreeConflict(t *testing.T) {
	t.Run("different worktrees returns error", func(t *testing.T) {
		gitOrSkip(t)
		root := t.TempDir()
		initRepoWithCommit(t, root)
		// Create a linked worktree on a separate branch.
		wtPath := filepath.Join(t.TempDir(), "wt2")
		gitRun(t, root, "worktree", "add", "-b", "t0194", wtPath)
		// Create the tickets dir in the main worktree root.
		mainTickets := filepath.Join(root, "tickets")
		if err := os.MkdirAll(mainTickets, 0o755); err != nil {
			t.Fatal(err)
		}
		// Change cwd to the linked worktree.
		saved, _ := os.Getwd()
		defer os.Chdir(saved) //nolint:errcheck
		if err := os.Chdir(wtPath); err != nil {
			t.Fatalf("Chdir: %v", err)
		}
		err := storeWorktreeConflict(mainTickets)
		if err == nil || !strings.Contains(err.Error(), "different worktree") {
			t.Errorf("expected 'different worktree' error, got: %v", err)
		}
	})

	t.Run("same worktree returns nil", func(t *testing.T) {
		gitOrSkip(t)
		root := t.TempDir()
		initRepoWithCommit(t, root)
		mainTickets := filepath.Join(root, "tickets")
		if err := os.MkdirAll(mainTickets, 0o755); err != nil {
			t.Fatal(err)
		}
		saved, _ := os.Getwd()
		defer os.Chdir(saved) //nolint:errcheck
		if err := os.Chdir(root); err != nil {
			t.Fatalf("Chdir: %v", err)
		}
		if err := storeWorktreeConflict(mainTickets); err != nil {
			t.Errorf("expected nil, got: %v", err)
		}
	})

	t.Run("non-repo cwd returns nil", func(t *testing.T) {
		plain := t.TempDir()
		store := t.TempDir()
		saved, _ := os.Getwd()
		defer os.Chdir(saved) //nolint:errcheck
		if err := os.Chdir(plain); err != nil {
			t.Fatalf("Chdir: %v", err)
		}
		if err := storeWorktreeConflict(store); err != nil {
			t.Errorf("expected nil for non-repo cwd, got: %v", err)
		}
	})
}

// setupCrossWorktreeEnv creates a main-worktree ticket store (with a bootstrap
// binary placeholder and one .erg file) and a linked worktree on a separate
// branch, then switches cwd to the linked worktree and installs the
// osExecutable seam so findTicketsDir resolves the main-worktree binary-dir
// candidate. Returns mainTickets for callers that need to inspect the store.
// All teardown is registered via t.Cleanup/defer and runs automatically.
func setupCrossWorktreeEnv(t *testing.T, branch string) (mainTickets string) {
	t.Helper()
	gitOrSkip(t)
	root := t.TempDir()
	initRepoWithCommit(t, root)
	mainTickets = filepath.Join(root, "tickets")
	if err := os.MkdirAll(mainTickets, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mainTickets, "0001-test.erg"),
		[]byte("%erg 0.1\nTitle: Test\nCreated: 2026-01-01\nAuthor: a\n\n--- log ---\n--- body ---\n"),
		0644); err != nil {
		t.Fatal(err)
	}
	ergBin := filepath.Join(mainTickets, "erg")
	if err := os.WriteFile(ergBin, []byte(""), 0755); err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(t.TempDir(), "wt")
	gitRun(t, root, "worktree", "add", "-b", branch, wtPath)

	origExe := osExecutable
	osExecutable = func() (string, error) { return ergBin, nil }
	t.Cleanup(func() { osExecutable = origExe })

	saved, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(saved) }) //nolint:errcheck
	if err := os.Chdir(wtPath); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	return mainTickets
}

func TestFindTicketsDir_CrossWorktreeRefused(t *testing.T) {
	setupCrossWorktreeEnv(t, "t0194")
	_, err := findTicketsDir()
	if err == nil || !strings.Contains(err.Error(), "different worktree") {
		t.Errorf("expected 'different worktree' error from findTicketsDir, got: %v", err)
	}
}

// TestCrossWorktreeCommandRejection verifies that cmdList, cmdReady, and cmdNew
// all fail with the "different worktree" diagnostic when the auto-discovered store
// is in a different git worktree than cwd.
func TestCrossWorktreeCommandRejection(t *testing.T) {
	setupCrossWorktreeEnv(t, "t0200-cmd")

	for _, tc := range []struct {
		name string
		run  func() (string, string)
	}{
		{"cmdList", func() (string, string) { return captureOutput(t, func() { cmdList(nil) }) }},
		{"cmdReady", func() (string, string) { return captureOutput(t, func() { cmdReady(nil) }) }},
		{"cmdNew", func() (string, string) { return captureOutput(t, func() { cmdNew([]string{"test ticket"}) }) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr := tc.run()
			if !strings.Contains(stderr, "different worktree") {
				t.Errorf("%s: expected 'different worktree' diagnostic on stderr, got: %q", tc.name, stderr)
			}
		})
	}
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
