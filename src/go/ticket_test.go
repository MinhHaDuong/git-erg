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
		if erg.LogSepCount == 0 {
			t.Error("expected LogSepCount > 0")
		}
		if erg.BodySepCount == 0 {
			t.Error("expected BodySepCount > 0")
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

	t.Run("magic line padded with whitespace", func(t *testing.T) {
		// parseErg uses TrimSpace before comparing to MagicLine, so leading/trailing
		// spaces on the first line must still be accepted as a valid magic marker.
		content := "  %erg v1  \nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg := parseErg(path)
		if !erg.HasMagic {
			t.Error("expected HasMagic=true for magic line padded with whitespace")
		}
	})

	t.Run("separator inside body section", func(t *testing.T) {
		// A second '--- log ---' inside the body increments LogSepCount but must
		// not change the section or cause a panic.
		content := "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n--- log ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg := parseErg(path)
		if !erg.HasMagic {
			t.Error("expected HasMagic=true")
		}
		if erg.LogSepCount != 2 {
			t.Errorf("LogSepCount = %d, want 2 (separator counted even inside body)", erg.LogSepCount)
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

func TestParseRef(t *testing.T) {
	type want struct {
		kind   RefKind
		id     string // RefLocal only
		host   string // RefForge only
		owner  string
		repo   string
		number string
	}
	cases := []struct {
		input   string
		wantErr bool
		want    want
	}{
		// Empty ref
		{"", true, want{}},

		// Valid local refs (exactly 4 ASCII digits)
		{"0042", false, want{kind: RefLocal, id: "0042"}},
		{"0001", false, want{kind: RefLocal, id: "0001"}},
		{"9999", false, want{kind: RefLocal, id: "9999"}},

		// Not 4 digits
		{"123", true, want{}},
		{"12345", true, want{}},
		{"abcd", true, want{}},

		// Deprecated gh: scheme
		{"gh:owner/repo#1", true, want{}},

		// Deprecated gh# scheme
		{"gh#42", true, want{}},

		// Case-variant old schemes
		{"GH#42", true, want{}},
		{"Gh:x/y#1", true, want{}},

		// Valid forge refs
		{"github.com/owner/repo#123", false, want{kind: RefForge, host: "github.com", owner: "owner", repo: "repo", number: "123"}},
		{"gitlab.com/org/project#7", false, want{kind: RefForge, host: "gitlab.com", owner: "org", repo: "project", number: "7"}},

		// Invalid forge refs
		{"github.com/owner/repo#0", true, want{}},  // leading zero
		{"github.com/owner/repo#", true, want{}},   // empty number
		{"github.com//repo#1", true, want{}},       // empty owner
		{"only-one-slash/here#1", true, want{}},    // not 3-part path
		{"host:port/owner/repo#1", true, want{}},   // host contains colon
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseRef(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseRef(%q) = %+v, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRef(%q) unexpected error: %v", tc.input, err)
			}
			if got.Raw != tc.input {
				t.Errorf("Raw = %q, want %q", got.Raw, tc.input)
			}
			if got.Kind != tc.want.kind {
				t.Errorf("Kind = %d, want %d", got.Kind, tc.want.kind)
			}
			if tc.want.kind == RefLocal {
				if got.ID != tc.want.id {
					t.Errorf("ID = %q, want %q", got.ID, tc.want.id)
				}
			}
			if tc.want.kind == RefForge {
				if got.Host != tc.want.host {
					t.Errorf("Host = %q, want %q", got.Host, tc.want.host)
				}
				if got.Owner != tc.want.owner {
					t.Errorf("Owner = %q, want %q", got.Owner, tc.want.owner)
				}
				if got.Repo != tc.want.repo {
					t.Errorf("Repo = %q, want %q", got.Repo, tc.want.repo)
				}
				if got.Number != tc.want.number {
					t.Errorf("Number = %q, want %q", got.Number, tc.want.number)
				}
			}
		})
	}
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
