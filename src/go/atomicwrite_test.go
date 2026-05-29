package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validTicket is a minimal well-formed %erg file used as the "good" original
// across the data-safety unit tests.
const validTicket = `%erg 0.1
Title: Sample
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
hello
`

// TestAtomicWriteFileReplacesViaRename is the negative control for the atomic /
// crash-safety guard: a temp-then-rename replacement gives the target a NEW
// inode, so os.SameFile is false. An in-place truncating writer (os.WriteFile)
// keeps the same inode — this test fails the moment the write stops being a
// rename.
func TestAtomicWriteFileReplacesViaRename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0001-x.erg")
	if err := os.WriteFile(path, []byte("old contents\n"), 0644); err != nil {
		t.Fatal(err)
	}
	oldInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := atomicWriteFile(path, []byte("new contents\n"), 0644); err != nil {
		t.Fatalf("atomicWriteFile: %v", err)
	}

	newInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(oldInfo, newInfo) {
		t.Fatal("write was in-place (same inode) — not a temp-then-rename atomic replace")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new contents\n" {
		t.Fatalf("content = %q, want %q", got, "new contents\n")
	}
}

// TestAtomicWriteFileRoundTrip is the lossless guard at the byte level: bytes in
// equal bytes out, including a trailing newline and an embedded NUL.
func TestAtomicWriteFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt.erg")
	want := []byte("line1\nline2\n\twith tab\n\x00binary\n")
	if err := atomicWriteFile(path, want, 0644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("round-trip mismatch:\n got %q\nwant %q", got, want)
	}
}

// TestAtomicWriteFileLeavesNoTemp is the no-clobber/cleanup control: after a
// successful write the directory holds only the target — no leftover temp.
func TestAtomicWriteFileLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0001-x.erg")
	if err := atomicWriteFile(path, []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "0001-x.erg" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected only the target file, found: %v", names)
	}
}

// TestAtomicWriteFilePreservesMode confirms the published file carries the
// requested mode, not CreateTemp's 0600.
func TestAtomicWriteFilePreservesMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m.erg")
	if err := atomicWriteFile(path, []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0644 {
		t.Fatalf("mode = %o, want 0644", info.Mode().Perm())
	}
}

// TestWriteTicketAtomicPreservesExistingMode is the negative control for mode
// preservation: an atomic replace makes a new inode, so writeTicketAtomic must
// carry the original file's permissions forward rather than resetting them to
// 0644. A tightened 0600 ticket must not silently widen to 0644.
func TestWriteTicketAtomicPreservesExistingMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0001-x.erg")
	if err := os.WriteFile(path, []byte(validTicket), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeTicketAtomic(dir, path, []byte(validTicket)); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %o, want preserved 0600", info.Mode().Perm())
	}
}

// TestWriteTicketAtomicRefusesInvalid is the validate-before-replace guard with
// its negative control: writing content that no longer parses as %erg is
// refused and the good original is left byte-for-byte intact. Remove the
// validate step and the original would be clobbered with garbage.
func TestWriteTicketAtomicRefusesInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0001-x.erg")
	if err := os.WriteFile(path, []byte(validTicket), 0644); err != nil {
		t.Fatal(err)
	}

	garbage := []byte("not an erg file at all\n")
	err := writeTicketAtomic(dir, path, garbage)
	if err == nil {
		t.Fatal("expected writeTicketAtomic to refuse invalid content")
	}

	got, _ := os.ReadFile(path)
	if string(got) != validTicket {
		t.Fatalf("original was modified despite refusal:\n%s", got)
	}
}

// TestWriteTicketAtomicAllowsWhenOriginalInvalid locks the fix for the
// over-strict gate: when the file on disk is ALREADY invalid, a mutation that
// leaves it (still) invalid must succeed — the gate only protects clean
// tickets. Negative control: a refuse-on-any-error gate fails this test, which
// is exactly the regression that left dangling Blocked-by edges on dependents
// carrying unrelated violations.
func TestWriteTicketAtomicAllowsWhenOriginalInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0001-x.erg")
	// Original is invalid (unknown header) AND carries a Blocked-by edge.
	original := strings.Replace(validTicket, "Author: claude\n", "Author: claude\nBogus: x\nBlocked-by: 0009\n", 1)
	if _, errs := parseErgBytes([]byte(original), path); len(errs) == 0 {
		t.Fatal("fixture precondition: original should be invalid")
	}
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	// New content drops the Blocked-by line but is still invalid (Bogus stays).
	updated := strings.Replace(original, "Blocked-by: 0009\n", "", 1)
	if err := writeTicketAtomic(dir, path, []byte(updated)); err != nil {
		t.Fatalf("writeTicketAtomic must allow a mutation on an already-invalid file: %v", err)
	}
	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "Blocked-by: 0009") {
		t.Fatal("mutation did not land — edge not cleared on the invalid dependent")
	}
}

// TestCreateExclusiveNoClobber is the no-clobber guard with its negative
// control: createExclusive (new's O_EXCL path) refuses to overwrite an existing
// file and leaves its contents intact. Drop O_EXCL and the second call would
// truncate/overwrite — failing this test.
func TestCreateExclusiveNoClobber(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0001-x.erg")

	if err := createExclusive(path, "first\n"); err != nil {
		t.Fatalf("first createExclusive: %v", err)
	}
	err := createExclusive(path, "second — must not land\n")
	if err == nil {
		t.Fatal("createExclusive overwrote an existing file (O_EXCL not enforced)")
	}
	if !os.IsExist(err) {
		t.Fatalf("expected an already-exists error, got: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "first\n" {
		t.Fatalf("existing file was modified: %q", got)
	}
}

// TestWriteTicketAtomicConfinement is the write-confinement guard with its
// negative control: a target outside the resolved store is refused with
// errOutsideStore and no file is created there. Remove the confinement check
// and the out-of-store file would be written.
func TestWriteTicketAtomicConfinement(t *testing.T) {
	store := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "0001-x.erg")

	err := writeTicketAtomic(store, target, []byte(validTicket))
	if err == nil {
		t.Fatal("expected writeTicketAtomic to refuse an out-of-store target")
	}
	if _, ok := err.(*errOutsideStore); !ok {
		t.Fatalf("error = %v, want *errOutsideStore", err)
	}
	if _, statErr := os.Stat(target); statErr == nil {
		t.Fatal("out-of-store file was created despite refusal")
	}
}

// TestWriteTicketAtomicConfinementParentEscape covers the "../escape" form: a
// path that climbs out of the store via .. is refused too.
func TestWriteTicketAtomicConfinementParentEscape(t *testing.T) {
	root := t.TempDir()
	store := filepath.Join(root, "tickets")
	if err := os.MkdirAll(store, 0755); err != nil {
		t.Fatal(err)
	}
	// Resolves to root/evil.erg — one level above the store.
	target := filepath.Join(store, "..", "evil.erg")
	err := writeTicketAtomic(store, target, []byte(validTicket))
	if err == nil {
		t.Fatal("expected refusal for ../ escape target")
	}
	if _, ok := err.(*errOutsideStore); !ok {
		t.Fatalf("error = %v, want *errOutsideStore", err)
	}
}

// TestWriteTicketAtomicHappyPath confirms a valid, in-store write succeeds and
// lands the content — so the guards above are refusing the bad case, not every
// case.
func TestWriteTicketAtomicHappyPath(t *testing.T) {
	store := t.TempDir()
	path := filepath.Join(store, "0001-x.erg")
	if err := writeTicketAtomic(store, path, []byte(validTicket)); err != nil {
		t.Fatalf("writeTicketAtomic on valid in-store content: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != validTicket {
		t.Fatalf("content mismatch:\n%s", got)
	}
	// A closed/ subdirectory ticket is also inside the store subtree.
	sub := filepath.Join(store, "closed")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	subPath := filepath.Join(sub, "0002-y.erg")
	if err := writeTicketAtomic(store, subPath, []byte(validTicket)); err != nil {
		t.Fatalf("writeTicketAtomic into closed/ subdir: %v", err)
	}
}

// TestWithinStore unit-checks the containment predicate directly.
func TestWithinStore(t *testing.T) {
	root := t.TempDir()
	store := filepath.Join(root, "tickets")
	cases := []struct {
		name   string
		target string
		want   bool
	}{
		{"direct child", filepath.Join(store, "0001-x.erg"), true},
		{"closed subdir", filepath.Join(store, "closed", "0001-x.erg"), true},
		{"store root itself", store, true},
		{"parent escape", filepath.Join(store, "..", "x.erg"), false},
		{"sibling dir", filepath.Join(root, "other", "0001-x.erg"), false},
		{"prefix-but-not-subdir", store + "-evil/0001.erg", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := withinStore(store, c.target)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("withinStore(%q, %q) = %v, want %v", store, c.target, got, c.want)
			}
		})
	}
}

// TestParseErgBytesAcceptsValidTicket guards the assumption writeTicketAtomic
// relies on: a well-formed ticket parses with zero errors, so the validate gate
// never blocks a legitimate mutation.
func TestParseErgBytesAcceptsValidTicket(t *testing.T) {
	if _, errs := parseErgBytes([]byte(validTicket), "0001-x.erg"); len(errs) > 0 {
		t.Fatalf("valid ticket reported errors: %s", strings.Join(errs, "; "))
	}
}
