package main

import (
	"strings"
	"testing"
)

func TestParseRef(t *testing.T) {
	type want struct {
		kind   RefKind
		id     string // RefLocal and RefPath
		module string // RefPath only
	}
	cases := []struct {
		input       string
		wantErr     bool
		errContains string // non-empty: err.Error() must contain this substring
		want        want
	}{
		// Empty ref.
		{"", true, "empty", want{}},

		// Local: exactly 4 ASCII digits -> current store.
		{"0042", false, "", want{kind: RefLocal, id: "0042"}},
		{"0001", false, "", want{kind: RefLocal, id: "0001"}},
		{"9999", false, "", want{kind: RefLocal, id: "9999"}},

		// Path-ref: a relative path ending in /NNNN -> sibling module.
		{"auth/0012", false, "", want{kind: RefPath, module: "auth", id: "0012"}},
		{"libs/auth/0042", false, "", want{kind: RefPath, module: "libs/auth", id: "0042"}},

		// Absolute URI (scheme present) -> opaque, RefURI.
		{"https://github.com/o/r/raw/main/tickets/0042-x.erg", false, "", want{kind: RefURI}},
		{"file:/abs/path/0042.erg", false, "", want{kind: RefURI}},
		{"gh:owner/repo#1", false, "", want{kind: RefURI}}, // a scheme now, no longer a special error

		// Legacy forge spelling is now just an unresolvable relative handle, not
		// an error: a path with a fragment, not ending in /NNNN.
		{"github.com/owner/repo#123", false, "", want{kind: RefURI}},

		// Relative refs that name no local ticket are valid-but-unresolved
		// handles (optimistic policy warns on them; they are not errors).
		{"123", false, "", want{kind: RefURI}},
		{"abcd", false, "", want{kind: RefURI}},
		{"auth/12345", false, "", want{kind: RefURI}}, // 5 digits -> not /NNNN

		// Only a malformed URI-reference is an error: a space or control char.
		{"0042 extra", true, "space or control", want{}},
		{"a\tb", true, "space or control", want{}},
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
			if tc.want.kind == RefLocal || tc.want.kind == RefPath {
				if got.ID != tc.want.id {
					t.Errorf("ID = %q, want %q", got.ID, tc.want.id)
				}
			}
			if tc.want.kind == RefPath && got.Module != tc.want.module {
				t.Errorf("Module = %q, want %q", got.Module, tc.want.module)
			}
		})
	}
}
