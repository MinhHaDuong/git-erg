package main

import "testing"

func TestListEntryHas(t *testing.T) {
	e := listEntry{closed: false, blocked: true, labels: []string{"needs-human", "deferred"}}
	tests := []struct {
		term string
		want bool
	}{
		{"open", true},
		{"closed", false},
		{"blocked", true},
		{"needs-human", true},
		{"deferred", true},
		{"missing", false},
	}
	for _, tt := range tests {
		if got := e.has(tt.term); got != tt.want {
			t.Errorf("has(%q) = %v, want %v", tt.term, got, tt.want)
		}
	}

	closed := listEntry{closed: true}
	if !closed.has("closed") {
		t.Error("closed entry should have 'closed'")
	}
	if closed.has("open") {
		t.Error("closed entry should not have 'open'")
	}
}

func TestFilterMatches(t *testing.T) {
	e := listEntry{closed: false, blocked: true, labels: []string{"needs-human"}}
	tests := []struct {
		name     string
		positive []string
		negative []string
		want     bool
	}{
		{"empty filter matches", nil, nil, true},
		{"positive present", []string{"open", "needs-human"}, nil, true},
		{"positive absent", []string{"deferred"}, nil, false},
		{"negative absent", nil, []string{"deferred"}, true},
		{"negative present", nil, []string{"needs-human"}, false},
		{"positive ok but negative present", []string{"open"}, []string{"blocked"}, false},
		{"conjunction all required", []string{"open", "deferred"}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := filter{positive: tt.positive, negative: tt.negative}
			if got := f.matches(e); got != tt.want {
				t.Errorf("matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReferencesOpenClosed(t *testing.T) {
	tests := []struct {
		name     string
		positive []string
		negative []string
		want     bool
	}{
		{"neither", []string{"needs-human"}, []string{"deferred"}, false},
		{"positive open", []string{"open"}, nil, true},
		{"positive closed", []string{"closed"}, nil, true},
		{"negative open", nil, []string{"open"}, true},
		{"negative closed", nil, []string{"closed"}, true},
		{"blocked is not open/closed", []string{"blocked"}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := referencesOpenClosed(tt.positive, tt.negative); got != tt.want {
				t.Errorf("referencesOpenClosed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlockedByLabel(t *testing.T) {
	forge := blockedByEntry{kind: "forge", ref: "github.com/foo/bar#1"}
	if got := blockedByLabel(forge); got != "github.com/foo/bar#1" {
		t.Errorf("forge label = %q, want raw ref", got)
	}
	local := blockedByEntry{kind: "local", id: "0042"}
	if got := blockedByLabel(local); got != "0042" {
		t.Errorf("local label = %q, want id", got)
	}
}

func TestIsDirArg(t *testing.T) {
	tmp := t.TempDir()

	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{"pseudo-tag closed reserved", "closed", false},
		{"pseudo-tag open reserved", "open", false},
		{"pseudo-tag blocked reserved", "blocked", false},
		{"contains slash", "tickets/", true},
		{"current dir", ".", true},
		{"parent dir", "..", true},
		{"existing directory", tmp, true},
		{"bare tag name", "needs-human", false},
		{"nonexistent bare name", "no-such-thing-xyzzy", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDirArg(tt.arg); got != tt.want {
				t.Errorf("isDirArg(%q) = %v, want %v", tt.arg, got, tt.want)
			}
		})
	}
}

func TestLoadListEntries(t *testing.T) {
	dir := t.TempDir()

	// 0001: open, labeled, no blockers.
	writeErg(t, dir, "0001-alpha.erg", "%erg 0.1\nTitle: Alpha\nCreated: 2024-01-01\nAuthor: test\nLabel: needs-human\n\n--- log ---\n--- body ---\n")
	// 0002: closed, blocks others when referenced.
	writeErg(t, dir, "0002-beta.erg", "%erg 0.1\nTitle: Beta\nCreated: 2024-01-02\nAuthor: test\nClosed: done\n\n--- log ---\n--- body ---\n")
	// 0003: blocked by a forge ref (offline-unknown -> always blocking).
	writeErg(t, dir, "0003-gamma.erg", "%erg 0.1\nTitle: Gamma\nCreated: 2024-01-03\nAuthor: test\nBlocked-by: github.com/foo/bar#1\n\n--- log ---\n--- body ---\n")
	// 0004: blocked by open local 0001 (blocking) and closed local 0002 (satisfied).
	writeErg(t, dir, "0004-delta.erg", "%erg 0.1\nTitle: Delta\nCreated: 2024-01-04\nAuthor: test\nBlocked-by: 0001\nBlocked-by: 0002\n\n--- log ---\n--- body ---\n")
	// 0005: blocked by unknown local 9999 -> warning, treated as satisfied.
	writeErg(t, dir, "0005-epsilon.erg", "%erg 0.1\nTitle: Epsilon\nCreated: 2024-01-05\nAuthor: test\nBlocked-by: 9999\n\n--- log ---\n--- body ---\n")

	entries, warnings := loadListEntries(dir)

	if len(entries) != 5 {
		t.Fatalf("got %d entries, want 5", len(entries))
	}

	// Sorted by ID ascending.
	wantOrder := []string{"0001", "0002", "0003", "0004", "0005"}
	byID := make(map[string]listEntry, len(entries))
	for i, e := range entries {
		if e.id != wantOrder[i] {
			t.Errorf("entry %d id = %q, want %q (sort order)", i, e.id, wantOrder[i])
		}
		byID[e.id] = e
	}

	if got := byID["0001"]; got.closed || got.blocked || len(got.labels) != 1 || got.labels[0] != "needs-human" {
		t.Errorf("0001: closed=%v blocked=%v labels=%v, want open/unblocked/[needs-human]", got.closed, got.blocked, got.labels)
	}
	if !byID["0002"].closed {
		t.Error("0002 should be closed")
	}

	// 0003: forge blocker -> blocked, one forge entry.
	g := byID["0003"]
	if !g.blocked || len(g.blockedBy) != 1 || g.blockedBy[0].kind != "forge" {
		t.Errorf("0003: blocked=%v blockedBy=%+v, want blocked with one forge entry", g.blocked, g.blockedBy)
	}

	// 0004: open local 0001 blocks; closed local 0002 is satisfied (dropped).
	d := byID["0004"]
	if !d.blocked || len(d.blockedBy) != 1 || d.blockedBy[0].kind != "local" || d.blockedBy[0].id != "0001" {
		t.Errorf("0004: blocked=%v blockedBy=%+v, want one local blocker 0001", d.blocked, d.blockedBy)
	}

	// 0005: unknown local ref -> not blocking, one warning emitted.
	if byID["0005"].blocked {
		t.Error("0005 should not be blocked by an unknown local ref")
	}
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want 1: %v", len(warnings), warnings)
	}
}
