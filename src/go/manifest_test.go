package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestIsCleanUpgrade exercises the dpkg 3-state decision (ticket 0211) at the
// pure-function level, including the manifest-absent historical-hash branch
// that the running binary's table is too dormant to reach (knownAssetHashes
// only ships the current embedded hash; installAssets short-circuits
// onDisk==embedded before the table is consulted).
func TestIsCleanUpgrade(t *testing.T) {
	const (
		diskOld   = "aaaa" // a pristine prior release, on disk now
		stampOld  = "aaaa" // what the last init recorded (== diskOld)
		otherHash = "bbbb" // some unrelated hash
	)

	cases := []struct {
		name  string
		disk  string
		stamp string
		known []string
		want  bool
	}{
		// Stamp present: clean upgrade iff disk matches the stamp.
		{"stamp matches disk -> clean upgrade", diskOld, stampOld, nil, true},
		{"stamp differs from disk -> local edit", diskOld, otherHash, nil, false},
		// Stamp absent: fall back to the known shipped hashes.
		{"no stamp, disk in history -> clean upgrade", diskOld, "", []string{otherHash, diskOld}, true},
		{"no stamp, disk not in history -> local edit", diskOld, "", []string{otherHash}, false},
		{"no stamp, empty history -> local edit", diskOld, "", nil, false},
		// Stamp takes precedence over history when present.
		{"stamp present but mismatched, even if in history -> local edit", diskOld, otherHash, []string{diskOld}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isCleanUpgrade(c.disk, c.stamp, c.known); got != c.want {
				t.Errorf("isCleanUpgrade(%q, %q, %v) = %v, want %v", c.disk, c.stamp, c.known, got, c.want)
			}
		})
	}
}

// TestReadManifest covers parsing, absence, and corruption (corruption must
// read as absent -- nil -- so a bad stamp never fails init).
func TestReadManifest(t *testing.T) {
	t.Run("absent -> nil", func(t *testing.T) {
		dir := t.TempDir()
		if m := readManifest(dir); m != nil {
			t.Errorf("expected nil for absent manifest, got %v", m)
		}
	})

	t.Run("well-formed -> parsed map", func(t *testing.T) {
		dir := t.TempDir()
		writeManifestFile(t, dir, "# erg provenance manifest -- do not edit\nrev: x\ndate: y\nassets:\n  .ergrc sha256:abc123\n  AGENTS.md sha256:def456\n")
		m := readManifest(dir)
		if m[".ergrc"] != "abc123" || m["AGENTS.md"] != "def456" {
			t.Errorf("parsed map wrong: %v", m)
		}
	})

	t.Run("malformed (no asset lines) -> nil", func(t *testing.T) {
		dir := t.TempDir()
		writeManifestFile(t, dir, "this is not a manifest at all\n{garbage}\n")
		if m := readManifest(dir); m != nil {
			t.Errorf("expected nil for malformed manifest, got %v", m)
		}
	})
}

// TestKnownAssetHashesIncludesEmbedded confirms the table always recognizes the
// current embedded asset (the cold-start guarantee).
func TestKnownAssetHashesIncludesEmbedded(t *testing.T) {
	content, ok := bootstrapAsset("tickets/.ergrc")
	if !ok {
		t.Fatal("embedded .ergrc missing")
	}
	want := sha256hex([]byte(content))
	found := false
	for _, h := range knownAssetHashes("tickets/.ergrc") {
		if h == want {
			found = true
		}
	}
	if !found {
		t.Errorf("knownAssetHashes(.ergrc) does not include the current embedded hash %q", want)
	}
}

// TestInstallAssetsHistoricalUpgrade drives the manifest-absent + historical-
// match row end to end via an injected historical hash -- the branch the
// production table is too dormant to reach. With the past content's hash in
// extraHistoricalHashes and no .erg-assets stamp, a pristine old .ergrc is a
// clean upgrade (overwritten), not a local edit.
func TestInstallAssetsHistoricalUpgrade(t *testing.T) {
	embedded, ok := bootstrapAsset("tickets/.ergrc")
	if !ok {
		t.Fatal("embedded .ergrc missing")
	}
	oldContent := "OLD SHIPPED ERGRC -- a pristine prior release\n"
	if oldContent == embedded {
		t.Fatal("test fixture collides with embedded content")
	}

	// Inject the old content's hash as a known shipped historical hash.
	oldHash := sha256hex([]byte(oldContent))
	extraHistoricalHashes[".ergrc"] = append(extraHistoricalHashes[".ergrc"], oldHash)
	t.Cleanup(func() { delete(extraHistoricalHashes, ".ergrc") })

	root := t.TempDir()
	ticketsDir := filepath.Join(root, "tickets")
	if err := os.MkdirAll(ticketsDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Lay the pristine OLD asset on disk; deliberately NO .erg-assets stamp.
	if err := os.WriteFile(filepath.Join(ticketsDir, ".ergrc"), []byte(oldContent), 0644); err != nil {
		t.Fatal(err)
	}

	// refuseDiverged=true (init default), dryRun=false.
	created, refreshed, skipped, _, err := installAssets(root, initAssetPaths, true, false)
	if err != nil {
		t.Fatalf("installAssets: %v", err)
	}
	if refreshed != 1 {
		t.Errorf("expected 1 refreshed (.ergrc clean upgrade), got refreshed=%d created=%d skipped=%d", refreshed, created, skipped)
	}
	if skipped != 0 {
		t.Errorf("expected 0 skipped (old asset is a known shipped hash, not a local edit), got %d", skipped)
	}
	got, _ := os.ReadFile(filepath.Join(ticketsDir, ".ergrc"))
	if string(got) != embedded {
		t.Errorf(".ergrc was not upgraded to the embedded content")
	}
}

// TestInstallAssetsUnknownPreserved is the negative control: an on-disk asset
// matching NO known hash and with no stamp is a local edit -- preserved
// (skipped), never clobbered.
func TestInstallAssetsUnknownPreserved(t *testing.T) {
	root := t.TempDir()
	ticketsDir := filepath.Join(root, "tickets")
	if err := os.MkdirAll(ticketsDir, 0755); err != nil {
		t.Fatal(err)
	}
	local := "UNKNOWN LOCAL EDIT NEVER SHIPPED\n"
	if err := os.WriteFile(filepath.Join(ticketsDir, ".ergrc"), []byte(local), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, skipped, _, err := installAssets(root, initAssetPaths, true, false)
	if err != nil {
		t.Fatalf("installAssets: %v", err)
	}
	if skipped < 1 {
		t.Errorf("expected the unknown .ergrc to be preserved (skipped>=1), got skipped=%d", skipped)
	}
	got, _ := os.ReadFile(filepath.Join(ticketsDir, ".ergrc"))
	if string(got) != local {
		t.Errorf("unknown local edit was overwritten -- data loss")
	}
}

func writeManifestFile(t *testing.T, dir, content string) {
	t.Helper()
	td := filepath.Join(dir, "tickets")
	if err := os.MkdirAll(td, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(td, manifestName), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
