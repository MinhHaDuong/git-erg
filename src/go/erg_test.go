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
		erg, _ := parseErg(path)
		if !erg.HasMagic {
			t.Error("expected HasMagic=true")
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
