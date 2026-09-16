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
		// Historical live case (ticket 0256): the 40-char truncation of this
		// title lands exactly on "-closed", which pathIsClosed reads as a
		// closed ticket. The offending trailing segment must be dropped.
		{"erg-pr-merge regex misses tickets/closed/NNNN paths",
			"erg-pr-merge-regex-misses-tickets"},
	}
	for _, c := range cases {
		got := slugify(c.in)
		if got != c.want {
			t.Errorf("slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestSlugifyNeverProducesClosedBasename pins the semantic property behind
// ticket 0256: no title may make erg new emit a filename that pathIsClosed
// reads as closed. Asserting through pathIsClosed itself -- rather than a
// literal strings.HasSuffix check -- keeps the test honest if the predicate's
// rule set ever changes.
func TestSlugifyNeverProducesClosedBasename(t *testing.T) {
	cases := []struct{ name, title string }{
		{
			"historical 0255 case",
			"erg-pr-merge regex misses tickets/closed/NNNN paths",
		},
		{
			// Anti-cheat control: this slug is 39 characters long after
			// truncation, so an implementation that merely truncates to 39
			// instead of guarding still keeps the whole "-closed" suffix and
			// still fails here.
			"exact 39-char boundary after truncation",
			strings.Repeat("x", 32) + "-closed-extra-words-here-padding-more",
		},
		{
			// Distinct path from the two cases above: this slug is 36
			// characters, so no truncation happens at all. The guard must
			// fire on a title that simply ends in "closed", not only on one
			// the 40-char cut mangled into it.
			"short title ending in closed, untruncated",
			"archive the ticket once it is closed",
		},
		{
			"title that is only the word closed",
			"closed",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			slug := slugify(c.title)
			basename := "0000-" + slug + ".erg"
			if pathIsClosed(basename) {
				t.Errorf("slugify(%q) = %q -> %q, which pathIsClosed reports as closed",
					c.title, slug, basename)
			}
			if slug == "" {
				t.Errorf("slugify(%q) returned an empty slug", c.title)
			}
		})
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
