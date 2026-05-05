package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseIDFromFilename(t *testing.T) {
	cases := []struct {
		name string
		want int
	}{
		{"0001-foo.erg", 1},
		{"0042-some-title.erg", 42},
		{"0100.erg", 100},
		{"9999-max.erg", 9999},
		{"0001-foo.txt", 0},
		{"readme.erg", 0},
		{"not-a-number-foo.erg", 0},
		{"", 0},
		{"0005.txt", 0},
	}
	for _, c := range cases {
		got := parseIDFromFilename(c.name)
		if got != c.want {
			t.Errorf("parseIDFromFilename(%q) = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestNextID(t *testing.T) {
	cases := []struct {
		desc  string
		files map[string]string // relative path -> content (content is irrelevant)
		want  string
	}{
		{
			desc:  "empty directory",
			files: nil,
			want:  "0001",
		},
		{
			desc:  "one ticket",
			files: map[string]string{"0001-foo.erg": ""},
			want:  "0002",
		},
		{
			desc:  "gap takes max+1 not first gap",
			files: map[string]string{"0001-a.erg": "", "0003-b.erg": ""},
			want:  "0004",
		},
		{
			desc:  "max existing 0099",
			files: map[string]string{"0099-high.erg": ""},
			want:  "0100",
		},
		{
			desc:  "non-erg files ignored",
			files: map[string]string{"0050-notes.txt": "", "0060-data.md": ""},
			want:  "0001",
		},
		{
			desc:  "non-numeric prefix ignored",
			files: map[string]string{"readme.erg": "", "abc-def.erg": ""},
			want:  "0001",
		},
		{
			desc: "mix of erg and non-erg with padded IDs",
			files: map[string]string{
				"0010-ticket.erg": "",
				"0020-other.txt":  "",
				"0005-also.erg":   "",
				"notes.md":        "",
			},
			want: "0011",
		},
		{
			desc: "tickets in closed/ subdir are counted",
			files: map[string]string{
				"closed/0010-old.erg": "",
				"0003-active.erg":     "",
			},
			want: "0011",
		},
		{
			desc: "tickets in archive/ subdir are counted",
			files: map[string]string{
				"archive/0050-ancient.erg": "",
				"0002-current.erg":         "",
			},
			want: "0051",
		},
		{
			desc: "subdirs combined with top-level",
			files: map[string]string{
				"0005-active.erg":           "",
				"closed/0020-done.erg":      "",
				"archive/0015-archived.erg": "",
			},
			want: "0021",
		},
		{
			desc: "nonexistent directory returns 0001",
			// handled specially below
			want: "0001",
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			if c.desc == "nonexistent directory returns 0001" {
				got := nextID("/nonexistent/path/that/does/not/exist")
				if got != c.want {
					t.Errorf("nextID(nonexistent) = %q, want %q", got, c.want)
				}
				return
			}

			tmp := t.TempDir()
			for relPath, content := range c.files {
				full := filepath.Join(tmp, relPath)
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			got := nextID(tmp)
			if got != c.want {
				t.Errorf("nextID() = %q, want %q", got, c.want)
			}
		})
	}
}
