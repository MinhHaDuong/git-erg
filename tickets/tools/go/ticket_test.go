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
		erg := parseErg(path)
		if !erg.HasMagic {
			t.Error("expected HasMagic=true")
		}
		if erg.Title() != "My Title" {
			t.Errorf("Title() = %q, want %q", erg.Title(), "My Title")
		}
		if !erg.HasLog {
			t.Error("expected HasLog=true")
		}
		if !erg.HasBody {
			t.Error("expected HasBody=true")
		}
	})

	t.Run("missing magic line", func(t *testing.T) {
		content := "Title: foo\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg := parseErg(path)
		if erg.HasMagic {
			t.Error("expected HasMagic=false")
		}
	})

	t.Run("CRLF line endings", func(t *testing.T) {
		content := strings.ReplaceAll(ergWithTitle("CRLF Title"), "\n", "\r\n")
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg := parseErg(path)
		if !erg.HasMagic {
			t.Error("expected HasMagic=true with CRLF endings")
		}
		if erg.Title() != "CRLF Title" {
			t.Errorf("Title() = %q, want %q", erg.Title(), "CRLF Title")
		}
	})

	t.Run("no trailing newline", func(t *testing.T) {
		content := "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n--- log ---\n--- body ---"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg := parseErg(path)
		if !erg.HasMagic {
			t.Error("expected HasMagic=true")
		}
	})

	t.Run("repeated Blocked-by header", func(t *testing.T) {
		content := "%erg v1\nTitle: A\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: 0002\nBlocked-by: 0003\n\n--- log ---\n--- body ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg := parseErg(path)
		bb := erg.BlockedBy()
		if len(bb) != 2 {
			t.Errorf("BlockedBy() = %v (len=%d), want 2 values", bb, len(bb))
		}
	})

	t.Run("file does not exist", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nonexistent.erg")
		erg := parseErg(path)
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
