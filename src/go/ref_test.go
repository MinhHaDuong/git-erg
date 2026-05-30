package main

import (
	"strings"
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
		input       string
		wantErr     bool
		errContains string // non-empty: err.Error() must contain this substring
		want        want
	}{
		// Empty ref
		{"", true, "", want{}},

		// Valid local refs (exactly 4 ASCII digits)
		{"0042", false, "", want{kind: RefLocal, id: "0042"}},
		{"0001", false, "", want{kind: RefLocal, id: "0001"}},
		{"9999", false, "", want{kind: RefLocal, id: "9999"}},

		// Not 4 digits
		{"123", true, "", want{}},
		{"12345", true, "", want{}},
		{"abcd", true, "", want{}},

		// Deprecated gh: scheme — must name the precise failure mode.
		// Distinguishing mutation: removing the gh: branch causes the
		// case-variant guard to fire, producing a "case-sensitive" message
		// instead of the correct "deprecated" message.
		{"gh:owner/repo#1", true, "deprecated", want{}},

		// Deprecated gh# scheme — same contract.
		{"gh#42", true, "deprecated", want{}},

		// Case-variant old schemes — must produce a "case-sensitive" message,
		// not the deprecated-scheme message.
		{"GH#42", true, "case-sensitive", want{}},
		{"Gh:x/y#1", true, "case-sensitive", want{}},

		// Valid forge refs
		{"github.com/owner/repo#123", false, "", want{kind: RefForge, host: "github.com", owner: "owner", repo: "repo", number: "123"}},
		{"gitlab.com/org/project#7", false, "", want{kind: RefForge, host: "gitlab.com", owner: "org", repo: "project", number: "7"}},

		// Invalid forge refs
		{"github.com/owner/repo#0", true, "", want{}},              // leading zero
		{"github.com/owner/repo#", true, "", want{}},               // empty number
		{"github.com//repo#1", true, "", want{}},                   // empty owner
		{"only-one-slash/here#1", true, "", want{}},                // not 3-part path
		{"host:port/owner/repo#1", true, "", want{}},               // host contains colon
		{"github.com/owner/repo/extra#1", true, "", want{}},        // 4-part path must be rejected (#5)
		{"github.com/owner/repo/a/b#1", true, "", want{}},          // 5-part path must be rejected (#5)
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseRef(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseRef(%q) = %+v, want error", tc.input, got)
					return
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("parseRef(%q) error = %q, want it to contain %q", tc.input, err.Error(), tc.errContains)
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
