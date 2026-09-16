package main

import (
	"os"
	"path/filepath"
	"testing"
)

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

	// 0003: an absolute/forge URI is unresolved -> optimistic warn, NOT blocked.
	g := byID["0003"]
	if g.blocked || len(g.blockedBy) != 0 {
		t.Errorf("0003: blocked=%v blockedBy=%+v, want not blocked (unresolved URI ref only warns)", g.blocked, g.blockedBy)
	}

	// 0004: open local 0001 blocks; closed local 0002 is satisfied (dropped).
	d := byID["0004"]
	if !d.blocked || len(d.blockedBy) != 1 || d.blockedBy[0].kind != "local" || d.blockedBy[0].id != "0001" {
		t.Errorf("0004: blocked=%v blockedBy=%+v, want one local blocker 0001", d.blocked, d.blockedBy)
	}

	// 0005: unknown local ref -> not blocking, warns.
	if byID["0005"].blocked {
		t.Error("0005 should not be blocked by an unknown local ref")
	}
	// Two unresolved refs warn: 0003's URI handle and 0005's unknown local 9999.
	if len(warnings) != 2 {
		t.Fatalf("got %d warnings, want 2: %v", len(warnings), warnings)
	}
}

// TestLoadListEntriesUnderTrippedAncestor covers `erg list` and `erg ready`
// against the ticket 0285 defect: loadListEntries is the seam both commands
// share, and its closed flag comes straight from IsClosed(). Addressed by an
// absolute path under an ancestor ending in "-closed", every open ticket used
// to read as closed, so both commands listed an empty store. A blocker whose
// target is thereby mis-read as closed is also silently satisfied, which is
// why 0002 carries a Blocked-by on 0001 here.
func TestLoadListEntriesUnderTrippedAncestor(t *testing.T) {
	store := filepath.Join(t.TempDir(), "dossier-closed", "tickets")
	if err := os.MkdirAll(store, 0755); err != nil {
		t.Fatal(err)
	}
	writeErg(t, store, "0001-normal-open-ticket.erg",
		"%erg 0.1\nTitle: Normal\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n")
	writeErg(t, store, "0002-another-open-one.erg",
		"%erg 0.1\nTitle: Another\nCreated: 2024-01-02\nAuthor: test\nBlocked-by: 0001\n\n--- log ---\n--- body ---\n")

	entries, _ := loadListEntries(store)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	for _, e := range entries {
		if e.closed {
			t.Errorf("%s: closed=true, want false -- the tripped component is above the store root", e.file)
		}
	}
	// erg ready filters on !closed && !blocked: 0001 is ready, 0002 is not.
	if entries[0].blocked {
		t.Errorf("0001 should not be blocked, got blockedBy=%+v", entries[0].blockedBy)
	}
	if !entries[1].blocked {
		t.Error("0002 should be blocked by the still-open 0001")
	}
}

// TestResolvePathRefUnderTrippedAncestor covers the quietest face of ticket
// 0285. A cross-module `Blocked-by: sibling/0042` is resolved by reading the
// sibling file directly, outside any store walk, so its closure state was
// decided from the whole absolute path: under a "*-closed" ancestor every
// such blocker resolved as closed and `erg ready` offered a genuinely blocked
// ticket for work. No message, no exit code -- just a wrong answer, which is
// why the corpus-wide failure this ticket started from was the lesser bug.
//
// The two fixtures differ only in the ancestor's name, so the untripped one
// is the positive control for the tripped one.
func TestResolvePathRefUnderTrippedAncestor(t *testing.T) {
	gitOrSkip(t)
	build := func(t *testing.T, ancestor string) string {
		t.Helper()
		top := filepath.Join(t.TempDir(), ancestor, "repo")
		main := filepath.Join(top, "main", "tickets")
		sibling := filepath.Join(top, "sibling", "tickets")
		for _, d := range []string{main, sibling} {
			if err := os.MkdirAll(d, 0755); err != nil {
				t.Fatal(err)
			}
		}
		// resolvePathRef anchors on the worktree top, so the fixture needs a
		// real repository for the sibling module to be reachable at all.
		gitRun(t, top, "init", "-q", ".")
		writeErg(t, main, "0001-depends.erg",
			"%erg 0.1\nTitle: Depends on sibling\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: sibling/0042\n\n--- log ---\n--- body ---\n")
		writeErg(t, sibling, "0042-the-sibling.erg",
			"%erg 0.1\nTitle: The sibling\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n")
		return main
	}

	blockedIn := func(t *testing.T, ancestor string) bool {
		t.Helper()
		entries, _ := loadListEntries(build(t, ancestor))
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		return entries[0].blocked
	}

	// Positive control: with an innocuous ancestor the open sibling blocks.
	// If this is false the fixture never resolved the ref and the assertion
	// below would pass without testing anything.
	if !blockedIn(t, "dossier-ok") {
		t.Fatal("control: an open cross-module blocker must block -- fixture did not resolve the ref")
	}
	if !blockedIn(t, "dossier-closed") {
		t.Error("a cross-module blocker read as closed because of an ancestor above the sibling store")
	}
}
