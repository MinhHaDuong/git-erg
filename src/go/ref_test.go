package main

import (
	"testing"
)

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
		{"github.com/owner/repo#0", true, want{}}, // leading zero
		{"github.com/owner/repo#", true, want{}},  // empty number
		{"github.com//repo#1", true, want{}},      // empty owner
		{"only-one-slash/here#1", true, want{}},   // not 3-part path
		{"host:port/owner/repo#1", true, want{}},  // host contains colon
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
