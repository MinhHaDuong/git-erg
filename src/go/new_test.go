package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Hello World", "hello-world"},
		{"My TICKET: with special\u2014chars & more!", "my-ticket-with-special-chars-more"},
		{"em\u2014dash collapsed", "em-dash-collapsed"},
		{"consecutive---hyphens", "consecutive-hyphens"},
		{"-leading and trailing-", "leading-and-trailing"},
		{"this is a very long title that exceeds forty characters definitely",
			// truncated to 40 chars: "this-is-a-very-long-title-that-exceeds-f"
			// trailing char is 'f' (from "forty"), not a hyphen, so TrimRight is a no-op
			"this-is-a-very-long-title-that-exceeds-f"},
		// Boundary pair: pins both sides of the > 40 truncation.
		{strings.Repeat("a", 40), strings.Repeat("a", 40)},
		{strings.Repeat("a", 41), strings.Repeat("a", 40)},
		{"", "untitled"},
		{"!@#$%^&*()", "untitled"},
	}
	for _, c := range cases {
		got := slugify(c.in)
		if got != c.want {
			t.Errorf("slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// readFirstErgFile returns the content of the first .erg file found in dir,
// or "" if none exists.
func readFirstErgFile(t *testing.T, dir string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*.erg"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("readFirstErgFile: %v", err)
	}
	return string(data)
}

func TestCmdNewAuthorFlag(t *testing.T) {
	// Isolate ERG_AUTHOR env so it doesn't interfere with explicit-flag tests.
	origErgAuthor, ergSet := os.LookupEnv("ERG_AUTHOR")
	defer func() {
		if ergSet {
			os.Setenv("ERG_AUTHOR", origErgAuthor)
		} else {
			os.Unsetenv("ERG_AUTHOR")
		}
	}()
	os.Unsetenv("ERG_AUTHOR")

	t.Run("--author sets Author header", func(t *testing.T) {
		dir := t.TempDir()
		code := cmdNew([]string{"test title", dir, "--author", "alice"})
		if code != 0 {
			t.Fatalf("cmdNew returned %d, want 0", code)
		}
		content := readFirstErgFile(t, dir)
		if content == "" {
			t.Fatal("no .erg file created")
		}
		if !strings.Contains(content, "\nAuthor: alice\n") {
			t.Errorf("Author header not found; got:\n%s", content)
		}
		if !strings.Contains(content, " alice created") {
			t.Errorf("log line does not reference alice; got:\n%s", content)
		}
	})

	t.Run("-a sets Author header", func(t *testing.T) {
		dir := t.TempDir()
		code := cmdNew([]string{"test title", dir, "-a", "bob"})
		if code != 0 {
			t.Fatalf("cmdNew returned %d, want 0", code)
		}
		content := readFirstErgFile(t, dir)
		if !strings.Contains(content, "\nAuthor: bob\n") {
			t.Errorf("Author header not found; got:\n%s", content)
		}
	})

	t.Run("--author=value sets Author header", func(t *testing.T) {
		dir := t.TempDir()
		code := cmdNew([]string{"test title", dir, "--author=carol"})
		if code != 0 {
			t.Fatalf("cmdNew returned %d, want 0", code)
		}
		content := readFirstErgFile(t, dir)
		if !strings.Contains(content, "\nAuthor: carol\n") {
			t.Errorf("Author header not found; got:\n%s", content)
		}
	})

	t.Run("--author overrides ERG_AUTHOR env var", func(t *testing.T) {
		os.Setenv("ERG_AUTHOR", "env-author")
		defer os.Unsetenv("ERG_AUTHOR")
		dir := t.TempDir()
		code := cmdNew([]string{"test title", dir, "--author", "flag-author"})
		if code != 0 {
			t.Fatalf("cmdNew returned %d, want 0", code)
		}
		content := readFirstErgFile(t, dir)
		if !strings.Contains(content, "\nAuthor: flag-author\n") {
			t.Errorf("expected flag-author to win; got:\n%s", content)
		}
	})

	t.Run("empty --author value errors", func(t *testing.T) {
		dir := t.TempDir()
		code := cmdNew([]string{"test title", dir, "--author", ""})
		if code == 0 {
			t.Fatal("cmdNew returned 0, want non-zero for empty --author")
		}
		// No .erg file should have been created.
		matches, _ := filepath.Glob(filepath.Join(dir, "*.erg"))
		if len(matches) > 0 {
			t.Errorf("ticket was created despite empty --author: %v", matches)
		}
	})

	t.Run("whitespace-only --author value errors", func(t *testing.T) {
		dir := t.TempDir()
		code := cmdNew([]string{"test title", dir, "--author", "   "})
		if code == 0 {
			t.Fatal("cmdNew returned 0, want non-zero for whitespace-only --author")
		}
	})

	t.Run("unknown flag errors and creates no directory", func(t *testing.T) {
		// Work in a temp parent so we can assert --bogus dir was not created.
		parent := t.TempDir()
		code := cmdNew([]string{"test title", "--bogus"})
		if code == 0 {
			t.Fatal("cmdNew returned 0, want non-zero for unknown flag")
		}
		// --bogus should not have been created as a directory.
		bogusDir := filepath.Join(parent, "--bogus")
		if _, err := os.Stat(bogusDir); err == nil {
			t.Errorf("--bogus directory was created at %s", bogusDir)
		}
		// Also assert no .erg file appeared in the parent.
		matches, _ := filepath.Glob(filepath.Join(parent, "*.erg"))
		if len(matches) > 0 {
			t.Errorf("unexpected .erg files in parent: %v", matches)
		}
	})

	t.Run("positional DIR still works without --author", func(t *testing.T) {
		dir := t.TempDir()
		code := cmdNew([]string{"my ticket", dir})
		if code != 0 {
			t.Fatalf("cmdNew returned %d, want 0", code)
		}
		content := readFirstErgFile(t, dir)
		if content == "" {
			t.Fatal("no .erg file created")
		}
		if !strings.Contains(content, "\nTitle: my ticket\n") {
			t.Errorf("Title header not found; got:\n%s", content)
		}
	})

	t.Run("--author strips newlines from value", func(t *testing.T) {
		dir := t.TempDir()
		code := cmdNew([]string{"test title", dir, "--author", "alice\nbob"})
		if code != 0 {
			t.Fatalf("cmdNew returned %d, want 0", code)
		}
		content := readFirstErgFile(t, dir)
		if !strings.Contains(content, "\nAuthor: alicebob\n") {
			t.Errorf("newline not stripped from author; got:\n%s", content)
		}
	})
}
