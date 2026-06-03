package main

import "testing"

func TestRefReferencesID(t *testing.T) {
	tests := []struct {
		ref, id string
		want    bool
	}{
		// Exact and bounded matches.
		{"0001", "0001", true},
		{"feat/0001", "0001", true},
		{"0001-foo", "0001", true},
		{"feat/0001-foo", "0001", true},
		{"release-0001-fixes", "0001", true},
		{"foo_0001_bar", "0001", true},
		{"0001/foo", "0001", true}, // start + '/' right boundary
		{"0001_foo", "0001", true}, // start + '_' right boundary
		{"_0001", "0001", true},    // '_' left + end boundary
		{"origin/feat/0140", "0140", true},
		// Word-boundary negatives (the substring would match but boundary fails).
		{"00010", "0001", false},
		{"00010-foo", "0001", false},
		{"feat/00010", "0001", false},
		{"f0001", "0001", false},
		{"foo0001", "0001", false},
		{"00001", "0001", false},
		{"some-0001thing", "0001", false},
		// Empty inputs.
		{"", "0001", false},
		{"feat/0001", "", false},
		// Empty id: the explicit `id == ""` short-circuit must fire for all
		// refName values, including pathological ones where an empty substring
		// would otherwise match at every offset.
		// Distinguishing mutation: removing `id == ""` from the guard makes
		// refReferencesID("",""), refReferencesID("a//b",""), etc. return true.
		{"", "", false},
		{"a//b", "", false},
		{"-", "", false},
		{"/", "", false},
		// Multiple matches in one ref still report true.
		{"0001-merge-0001", "0001", true},
		// A ref can reference more than one ticket.
		{"feat/0140-uses-0141", "0140", true},
		{"feat/0140-uses-0141", "0141", true},
		{"feat/0140-uses-0141", "0142", false},
	}
	for _, tt := range tests {
		got := refReferencesID(tt.ref, tt.id)
		if got != tt.want {
			t.Errorf("refReferencesID(%q, %q) = %v, want %v", tt.ref, tt.id, got, tt.want)
		}
	}
}
