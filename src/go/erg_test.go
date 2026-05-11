package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseHeaderLine(t *testing.T) {
	cases := []struct {
		input   string
		wantKey string
		wantVal string
		wantOK  bool
	}{
		{"Title: foo", "Title", "foo", true},
		{"Blocked-by: 0042", "Blocked-by", "0042", true},
		{"Key_with_under: v", "Key_with_under", "v", true},
		{"Key  : v", "Key", "v", true}, // spaces before colon are allowed
		{"no-colon", "", "", false},
		{" leading-space: v", "", "", false},
		{"1starts-digit: v", "", "", false},
		{"", "", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			gotKey, gotVal, gotOK := parseHeaderLine(tc.input)
			if gotOK != tc.wantOK || gotKey != tc.wantKey || gotVal != tc.wantVal {
				t.Errorf("parseHeaderLine(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.input, gotKey, gotVal, gotOK,
					tc.wantKey, tc.wantVal, tc.wantOK)
			}
		})
	}
}

// ergWithTitle returns a minimal valid ticket with the given title.
// Uses the same base as validErgContent but with a parameterised title.
func ergWithTitle(title string) string {
	return "%erg v1\nTitle: " + title + "\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
}

func TestParseErg(t *testing.T) {
	t.Run("minimal valid ticket", func(t *testing.T) {
		path := writeErg(t, t.TempDir(), "0001-test.erg", ergWithTitle("My Title"))
		erg, diag := parseErg(path)
		if !erg.HasMagic {
			t.Error("expected HasMagic=true")
		}
		if erg.Title != "My Title" {
			t.Errorf("Title = %q, want %q", erg.Title, "My Title")
		}
		if !diag.HasLogSep {
			t.Error("expected ParseDiagnostics.HasLogSep=true")
		}
		if !diag.HasBodySep {
			t.Error("expected ParseDiagnostics.HasBodySep=true")
		}
	})

	t.Run("missing magic line", func(t *testing.T) {
		content := "Title: foo\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg, _ := parseErg(path)
		if erg.HasMagic {
			t.Error("expected HasMagic=false")
		}
	})

	t.Run("CRLF line endings", func(t *testing.T) {
		content := strings.ReplaceAll(ergWithTitle("CRLF Title"), "\n", "\r\n")
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg, _ := parseErg(path)
		if !erg.HasMagic {
			t.Error("expected HasMagic=true with CRLF endings")
		}
		if erg.Title != "CRLF Title" {
			t.Errorf("Title = %q, want %q", erg.Title, "CRLF Title")
		}
	})

	t.Run("no trailing newline", func(t *testing.T) {
		content := "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n--- log ---\n--- body ---"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg, diag := parseErg(path)
		if !erg.HasMagic {
			t.Error("expected HasMagic=true")
		}
		if !diag.HasLogSep {
			t.Error("expected HasLogSep=true (--- log --- on penultimate line)")
		}
		if !diag.HasBodySep {
			t.Error("expected HasBodySep=true (--- body --- as final line, no trailing newline)")
		}
	})

	t.Run("repeated Blocked-by header", func(t *testing.T) {
		content := "%erg v1\nTitle: A\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: 0002\nBlocked-by: 0003\n\n--- log ---\n--- body ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg, _ := parseErg(path)
		bb := erg.BlockedBys
		if len(bb) != 2 {
			t.Errorf("BlockedBys = %v (len=%d), want 2 values", bb, len(bb))
		}
	})

	t.Run("magic line padded with whitespace", func(t *testing.T) {
		// parseErg uses TrimSpace before comparing to MagicLine, so leading/trailing
		// spaces on the first line must still be accepted as a valid magic marker.
		content := "  %erg v1  \nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg, _ := parseErg(path)
		if !erg.HasMagic {
			t.Error("expected HasMagic=true for magic line padded with whitespace")
		}
	})

	t.Run("separator inside body section", func(t *testing.T) {
		// A second '--- log ---' inside the body must not change the
		// section or cause a panic. The literal becomes body text and
		// HasLogSep stays true (set on ANY sighting).
		content := "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n--- log ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg, diag := parseErg(path)
		if !erg.HasMagic {
			t.Error("expected HasMagic=true")
		}
		if !diag.HasLogSep {
			t.Error("expected ParseDiagnostics.HasLogSep=true")
		}
		if !strings.Contains(erg.Body, "--- log ---") {
			t.Errorf("erg.Body = %q, expected to contain the quoted '--- log ---' literal", erg.Body)
		}
	})

	t.Run("duplicate Title keeps first value", func(t *testing.T) {
		// Item 4: singleton "first occurrence wins" — Title is a singleton
		// header; when duplicated, the parser keeps the first value.
		content := "%erg v1\nTitle: First\nTitle: Second\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg, _ := parseErg(path)
		if erg.Title != "First" {
			t.Errorf("Title = %q, want %q (first occurrence wins)", erg.Title, "First")
		}
	})

	t.Run("empty Tag value skipped", func(t *testing.T) {
		// Item 5: empty Tag: values are silently skipped by the parser —
		// parseHeaderLine trims, and the parser skips empty val for Tag.
		content := "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nTag: \n\n--- log ---\n--- body ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg, _ := parseErg(path)
		if len(erg.Tags) != 0 {
			t.Errorf("Tags = %v (len=%d), want empty slice (empty Tag: value skipped)", erg.Tags, len(erg.Tags))
		}
	})

	t.Run("file does not exist", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nonexistent.erg")
		erg, _ := parseErg(path)
		if erg.HasMagic {
			t.Error("expected HasMagic=false for nonexistent file")
		}
		if erg.Path != path {
			t.Errorf("Path = %q, want %q", erg.Path, path)
		}
	})
}

func TestJsonEscape(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{`back\slash`, `back\\slash`},
		{`quote"here`, `quote\"here`},
		{"tab\there", `tab\there`},
		{"new\nline", `new\nline`},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := jsonEscape(tc.input)
			if got != tc.want {
				t.Errorf("jsonEscape(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestClosedWhitespaceDivergence pins the divergence between
// parseHeaderLine and isClosedHeaderLine (item 7).
// parseHeaderLine accepts `Closed : val` (space before colon) as a valid
// header, but isClosedHeaderLine requires the exact prefix `Closed:` (no
// space). When `Closed : merged` appears in the body section, the parser
// does not set diag.ClosedInBody, so the validator does not reject it.
// This test pins that current behavior.
func TestClosedWhitespaceDivergence(t *testing.T) {
	t.Run("Closed: in body triggers ClosedInBody", func(t *testing.T) {
		content := "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\nClosed: merged\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		_, diag := parseErg(path)
		if !diag.ClosedInBody {
			t.Error("expected ClosedInBody=true for 'Closed: merged' in body")
		}
	})

	t.Run("Closed_space_colon in body does NOT trigger ClosedInBody", func(t *testing.T) {
		// `Closed : merged` — parseHeaderLine would parse this as key=Closed,
		// but isClosedHeaderLine does not fire because it expects prefix `Closed:`.
		// This pins the current divergence; a future ticket may align them.
		content := "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\nClosed : merged\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		_, diag := parseErg(path)
		if diag.ClosedInBody {
			t.Error("expected ClosedInBody=false for 'Closed : merged' — isClosedHeaderLine requires exact 'Closed:' prefix")
		}
	})
}

// TestPathIsClosed exercises the path-component closure test from erg.go.
// Item 2: covers directory names, basename prefixes/suffixes, and
// case-insensitivity as specified in rules/tickets.md.
func TestPathIsClosed(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		// Empty path
		{"", false},

		// Directory component equals "closed"
		{"closed/0001-foo.erg", true},
		{"tickets/closed/0001-foo.erg", true},

		// Directory starts with "closed-"
		{"closed-2024/0001-foo.erg", true},

		// Directory starts with "closed."
		{"closed.old/0001-foo.erg", true},

		// Directory ends with "-closed"
		{"archive-closed/0001-foo.erg", true},

		// Basename (without extension) equals "closed"
		{"tickets/closed.erg", true},

		// Basename starts with "closed-"
		{"closed-foo.erg", true},

		// Basename ends with "-closed"
		{"0001-closed.erg", true},

		// Case insensitivity
		{"Closed/0001-foo.erg", true},
		{"CLOSED/0001-foo.erg", true},

		// Open paths — no closure signal
		{"tickets/0001-foo.erg", false},
		{"open/0001-foo.erg", false},

		// "disclosed" must not trigger — the implementation checks HasPrefix
		// on the full component, so "disclosed" does NOT match.
		{"disclosed/0001-foo.erg", false},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got := pathIsClosed(tc.path)
			if got != tc.want {
				t.Errorf("pathIsClosed(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// TestStaleBlockedBy exercises staleBlockedBy (check.go:33).
// Item 1: constructs two tickets — one closed blocker, one open ticket
// referencing it — and asserts the warning fires.
func TestStaleBlockedBy(t *testing.T) {
	t.Run("open ticket blocked by closed ticket emits warning", func(t *testing.T) {
		dir := t.TempDir()
		// 0001 is closed (has Closed: header)
		writeErg(t, dir, "0001-blocker.erg",
			"%erg v1\nTitle: Blocker\nCreated: 2024-01-01\nAuthor: test\nClosed: done\n\n--- log ---\n--- body ---\n")
		// 0002 is open and blocked by 0001
		writeErg(t, dir, "0002-feature.erg",
			"%erg v1\nTitle: Feature\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: 0001\n\n--- log ---\n--- body ---\n")

		tickets, _ := loadErgs(dir)
		warnings := staleBlockedBy(tickets)
		if len(warnings) == 0 {
			t.Fatal("expected a stale Blocked-by warning, got none")
		}
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "0001") && strings.Contains(w, "already closed") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected warning mentioning '0001' and 'already closed', got: %v", warnings)
		}
	})

	t.Run("open ticket blocked by open ticket emits no warning", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0001-blocker.erg",
			"%erg v1\nTitle: Blocker\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n")
		writeErg(t, dir, "0002-feature.erg",
			"%erg v1\nTitle: Feature\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: 0001\n\n--- log ---\n--- body ---\n")

		tickets, _ := loadErgs(dir)
		warnings := staleBlockedBy(tickets)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings, got: %v", warnings)
		}
	})

	t.Run("closed ticket blocked by closed ticket emits no warning", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0001-blocker.erg",
			"%erg v1\nTitle: Blocker\nCreated: 2024-01-01\nAuthor: test\nClosed: done\n\n--- log ---\n--- body ---\n")
		writeErg(t, dir, "0002-feature.erg",
			"%erg v1\nTitle: Feature\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: 0001\nClosed: done\n\n--- log ---\n--- body ---\n")

		tickets, _ := loadErgs(dir)
		warnings := staleBlockedBy(tickets)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings for closed-blocked-by-closed, got: %v", warnings)
		}
	})

	t.Run("forge ref does not trigger stale warning", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0001-feature.erg",
			"%erg v1\nTitle: Feature\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: github.com/foo/bar#1\n\n--- log ---\n--- body ---\n")

		tickets, _ := loadErgs(dir)
		warnings := staleBlockedBy(tickets)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings for forge ref, got: %v", warnings)
		}
	})
}
