package main

import (
	"os"
	"path/filepath"
	"strings"
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

	const (
		early = "2020-01-01T00:00:00Z"
		late  = "2099-01-01T00:00:00Z"
	)

	cases := []struct {
		name    string
		disk    string
		stamp   string
		known   []string
		stampAt string
		runAt   string
		want    bool
	}{
		// No dates recorded: every row below must behave exactly as it did
		// before the direction check existed (ticket 0279). Kept explicit
		// rather than silently satisfied -- this IS the no-regression claim.
		{"stamp matches disk -> clean upgrade", diskOld, stampOld, nil, "", "", true},
		{"stamp differs from disk -> local edit", diskOld, otherHash, nil, "", "", false},
		// Stamp absent: fall back to the known shipped hashes.
		{"no stamp, disk in history -> clean upgrade", diskOld, "", []string{otherHash, diskOld}, "", "", true},
		{"no stamp, disk not in history -> local edit", diskOld, "", []string{otherHash}, "", "", false},
		{"no stamp, empty history -> local edit", diskOld, "", nil, "", "", false},
		// Stamp takes precedence over history when present.
		{"stamp present but mismatched, even if in history -> local edit", diskOld, otherHash, []string{diskOld}, "", "", false},
		// Direction: a stamp match is an upgrade only when the stamp is older.
		{"stamp matches disk, stamp older -> clean upgrade", diskOld, stampOld, nil, early, late, true},
		{"stamp matches disk, stamp newer -> rollback, not an upgrade", diskOld, stampOld, nil, late, early, false},
		{"stamp matches disk, same date -> clean upgrade", diskOld, stampOld, nil, early, early, true},
		{"stamp matches disk, dateless stamp -> clean upgrade", diskOld, stampOld, nil, "", late, true},
		// The history branch is direction-free: a hash only reaches it by being
		// one this binary already ships as superseded, so it cannot be newer.
		{"no stamp, disk in history, dates present -> clean upgrade", diskOld, "", []string{diskOld}, late, early, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isCleanUpgrade(c.disk, c.stamp, c.known, c.stampAt, c.runAt); got != c.want {
				t.Errorf("isCleanUpgrade(%q, %q, %v, %q, %q) = %v, want %v", c.disk, c.stamp, c.known, c.stampAt, c.runAt, got, c.want)
			}
		})
	}
}

// TestIsRollback locks the lexical-compare contract in place, away from the
// three call sites that consume it. The ISO-8601 stamps compared here are
// fixed-width UTC by construction (Makefile), which is what makes string
// ordering chronological ordering -- and what makes time.Parse unnecessary.
func TestIsRollback(t *testing.T) {
	cases := []struct {
		name        string
		stamp, run  string
		wantRollbck bool
	}{
		{"stamp newer than binary -> rollback", "2026-06-29T10:00:00Z", "2026-06-05T10:00:00Z", true},
		{"stamp older than binary -> not a rollback", "2026-06-05T10:00:00Z", "2026-06-29T10:00:00Z", false},
		{"same instant -> not a rollback", "2026-06-05T10:00:00Z", "2026-06-05T10:00:00Z", false},
		{"one second apart, stamp newer -> rollback", "2026-06-05T10:00:01Z", "2026-06-05T10:00:00Z", true},
		// Absent provenance is not evidence of a rollback: it must fall through
		// to the pre-0279 behaviour, never to a refusal.
		{"no stamp date -> not a rollback", "", "2026-06-05T10:00:00Z", false},
		{"no running date (go test, no -ldflags) -> not a rollback", "2026-06-29T10:00:00Z", "", false},
		{"stamp date 'unknown' (unstamped build) -> not a rollback", "unknown", "2026-06-05T10:00:00Z", false},
		{"running date 'unknown' -> not a rollback", "2026-06-29T10:00:00Z", "unknown", false},
		{"both absent -> not a rollback", "", "", false},
		// Unparseable stamps: every one of these sorts ABOVE any digit, so an
		// unguarded lexical compare would read them all as a rollback. The
		// "y" row is not hypothetical -- tests/test_check.sh's drift fixture
		// carries exactly that stamp, and it is what caught the missing guard.
		{"stamp date 'y' (garbage) -> not a rollback", "y", "2026-06-05T10:00:00Z", false},
		{"stamp date is a free-text string -> not a rollback", "not-a-date", "2026-06-05T10:00:00Z", false},
		// Wrong shape, not garbage: still not comparable byte by byte.
		{"date-only stamp -> not a rollback", "2099-01-01", "2026-06-05T10:00:00Z", false},
		{"stamp with a UTC offset instead of Z -> not a rollback", "2099-01-01T10:00:00+02:00", "2026-06-05T10:00:00Z", false},
		{"stamp missing the trailing Z -> not a rollback", "2099-01-01T10:00:00", "2026-06-05T10:00:00Z", false},
		{"stamp with letters in the digit positions -> not a rollback", "yyyy-mm-ddThh:mm:ssZ", "2026-06-05T10:00:00Z", false},
		{"running date malformed -> not a rollback", "2099-01-01T10:00:00Z", "whenever", false},
		// Right byte shape, impossible calendar value. A digit-vs-literal shape
		// check alone passes these, and each sorts ABOVE any real stamp, so an
		// unguarded compare reads a hand-mangled manifest as a rollback and
		// freezes the store. "Not comparable" is the correct verdict -- the same
		// outcome as the "y" garbage row above, which must stay green.
		{"all-nines impossible calendar -> not a rollback", "9999-99-99T99:99:99Z", "2026-06-05T10:00:00Z", false},
		{"month 13 -> not a rollback", "2099-13-01T10:00:00Z", "2026-06-05T10:00:00Z", false},
		{"month 00 -> not a rollback", "2099-00-01T10:00:00Z", "2026-06-05T10:00:00Z", false},
		{"day 32 -> not a rollback", "2099-01-32T10:00:00Z", "2026-06-05T10:00:00Z", false},
		{"day 00 -> not a rollback", "2099-01-00T10:00:00Z", "2026-06-05T10:00:00Z", false},
		{"hour 24 -> not a rollback", "2099-01-01T24:00:00Z", "2026-06-05T10:00:00Z", false},
		{"minute 60 -> not a rollback", "2099-01-01T10:60:00Z", "2026-06-05T10:00:00Z", false},
		{"second 61 -> not a rollback", "2099-01-01T10:00:61Z", "2026-06-05T10:00:00Z", false},
		// Leap second: a legal value the stamper can emit, so it stays comparable.
		{"leap second 60 -> comparable, and newer", "2099-12-31T23:59:60Z", "2026-06-05T10:00:00Z", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isRollback(c.stamp, c.run); got != c.wantRollbck {
				t.Errorf("isRollback(%q, %q) = %v, want %v", c.stamp, c.run, got, c.wantRollbck)
			}
		})
	}
}

// TestParseManifestDate covers the sibling reader added for the direction check:
// present, absent (an erg predating the field), and the "unknown" literal
// buildManifest writes for an unstamped build.
func TestParseManifestDate(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{"present", "rev: abc\ndate: 2026-06-29T10:00:00Z\nassets:\n  .ergrc sha256:aa\n", "2026-06-29T10:00:00Z"},
		{"absent (pre-date: manifest)", "rev: abc\nassets:\n  .ergrc sha256:aa\n", ""},
		{"unknown (unstamped build)", "rev: unknown\ndate: unknown\nassets:\n  .ergrc sha256:aa\n", "unknown"},
		{"empty file", "", ""},
		{"no date anywhere", "garbage\n{}\n", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseManifestDate([]byte(c.body)); got != c.want {
				t.Errorf("parseManifestDate(%q) = %q, want %q", c.body, got, c.want)
			}
		})
	}

	t.Run("readManifestDate on an absent file -> empty", func(t *testing.T) {
		if got := readManifestDate(t.TempDir()); got != "" {
			t.Errorf("expected %q for an absent manifest, got %q", "", got)
		}
	})

	t.Run("readManifestDate round-trips what buildManifest writes", func(t *testing.T) {
		setBuildDate(t, "2026-06-29T10:00:00Z")
		body, err := buildManifest(nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := parseManifestDate([]byte(body)); got != buildDate {
			t.Errorf("writer and reader disagree: wrote %q, read back %q", buildDate, got)
		}
	})
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

// stampFixture lays out a ticket store under a fresh temp root: the .erg-assets
// manifest given by manifestBody, an on-disk .ergrc holding ergrcContent, and an
// AGENTS.md byte-identical to the embedded copy (so it reports "unchanged" and
// never muddies the counts under test). Returns the root.
func stampFixture(t *testing.T, manifestBody, ergrcContent string) string {
	t.Helper()
	root := t.TempDir()
	ticketsDir := filepath.Join(root, "tickets")
	if err := os.MkdirAll(ticketsDir, 0755); err != nil {
		t.Fatal(err)
	}
	agents, ok := bootstrapAsset("tickets/AGENTS.md")
	if !ok {
		t.Fatal("embedded AGENTS.md missing")
	}
	if err := os.WriteFile(filepath.Join(ticketsDir, "AGENTS.md"), []byte(agents), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ticketsDir, ".ergrc"), []byte(ergrcContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ticketsDir, manifestName), []byte(manifestBody), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}

// setBuildDate pins the running binary's build stamp for the duration of a test.
// In `go test` buildDate is "" (no -ldflags), and an empty running date means
// "no provenance to compare" -- so a direction test MUST pin it, or it silently
// exercises the dateless fall-through instead of the case it names.
func setBuildDate(t *testing.T, date string) {
	t.Helper()
	prev := buildDate
	buildDate = date
	t.Cleanup(func() { buildDate = prev })
}

// manifestWith renders a fixture manifest: the given date (omitted entirely when
// empty, as an erg predating the date: field would write it) and the given
// .ergrc hash, with AGENTS.md always stamped at its embedded hash.
func manifestWith(t *testing.T, date, ergrcHash string) string {
	t.Helper()
	agents, ok := bootstrapAsset("tickets/AGENTS.md")
	if !ok {
		t.Fatal("embedded AGENTS.md missing")
	}
	head := "# erg provenance manifest -- do not edit\nrev: fixture\n"
	if date != "" {
		head += "date: " + date + "\n"
	}
	return head + "assets:\n" +
		"  .ergrc sha256:" + ergrcHash + "\n" +
		"  AGENTS.md sha256:" + sha256hex([]byte(agents)) + "\n"
}

// TestInstallAssetsRollbackPreserved is the direction test for ticket 0279: the
// dpkg 3-state compare must not read "on-disk matches the stamp" as a licence to
// overwrite when the stamp was written by a NEWER binary than the running one.
// Four arms, and the older-stamp arm is not decoration: the newer-stamp arm alone
// also passes for an implementation that refuses every upgrade, which is exactly
// the regression the ticket's invariants forbid.
func TestInstallAssetsRollbackPreserved(t *testing.T) {
	// Content a LATER release shipped: on disk now, recorded in the stamp, and
	// different from what this binary embeds.
	newer := "ERGRC FROM A LATER RELEASE -- pristine, not a local edit\n"
	older := "ERGRC FROM AN EARLIER RELEASE -- pristine, not a local edit\n"
	embedded, ok := bootstrapAsset("tickets/.ergrc")
	if !ok {
		t.Fatal("embedded .ergrc missing")
	}
	if newer == embedded || older == embedded {
		t.Fatal("test fixture collides with embedded content")
	}

	t.Run("newer stamp -> preserved", func(t *testing.T) {
		setBuildDate(t, "2026-01-01T00:00:00Z")
		root := stampFixture(t, manifestWith(t, "2099-01-01T00:00:00Z", sha256hex([]byte(newer))), newer)

		created, refreshed, skipped, unchanged, err := installAssets(root, initAssetPaths, true, false)
		if err != nil {
			t.Fatalf("installAssets: %v", err)
		}
		if skipped != 1 {
			t.Errorf("stamp is newer than this binary: expected skipped=1, got created=%d refreshed=%d skipped=%d unchanged=%d",
				created, refreshed, skipped, unchanged)
		}
		got, _ := os.ReadFile(filepath.Join(root, "tickets", ".ergrc"))
		if string(got) != newer {
			t.Errorf("a binary older than the stamp reverted the asset -- data loss")
		}
	})

	t.Run("newer stamp -> provenance survives, and so does the verdict", func(t *testing.T) {
		// A preserving run touched nothing, so it must not restamp the store
		// with its own older rev/date: that would erase the evidence of the
		// rollback and make the SECOND run misread it as a local edit --
		// still preserved, but for a reason that is not true.
		setBuildDate(t, "2026-01-01T00:00:00Z")
		const stamped = "2099-01-01T00:00:00Z"
		root := stampFixture(t, manifestWith(t, stamped, sha256hex([]byte(newer))), newer)

		if _, _, _, _, err := installAssets(root, initAssetPaths, true, false); err != nil {
			t.Fatalf("installAssets: %v", err)
		}
		if got := readManifestDate(root); got != stamped {
			t.Errorf("the preserving run overwrote the provenance stamp: %q, want %q", got, stamped)
		}

		stderr := captureStderr(t, func() {
			if _, _, skipped, _, err := installAssets(root, initAssetPaths, true, false); err != nil {
				t.Fatalf("second installAssets: %v", err)
			} else if skipped != 1 {
				t.Errorf("second run: expected skipped=1, got %d", skipped)
			}
		})
		if strings.Contains(stderr, "local edits") {
			t.Errorf("second run calls the rollback a local edit: %q", stderr)
		}
		if !strings.Contains(stderr, "erg update") {
			t.Errorf("second run lost the remedy: %q", stderr)
		}
	})

	t.Run("older stamp -> upgraded", func(t *testing.T) {
		// The positive control: the ordinary `erg update && erg init` path must
		// stay silent and automatic. An over-eager direction check breaks here.
		setBuildDate(t, "2026-01-01T00:00:00Z")
		root := stampFixture(t, manifestWith(t, "2020-01-01T00:00:00Z", sha256hex([]byte(older))), older)

		created, refreshed, skipped, unchanged, err := installAssets(root, initAssetPaths, true, false)
		if err != nil {
			t.Fatalf("installAssets: %v", err)
		}
		if refreshed != 1 || skipped != 0 {
			t.Errorf("stamp is older than this binary: expected refreshed=1 skipped=0, got created=%d refreshed=%d skipped=%d unchanged=%d",
				created, refreshed, skipped, unchanged)
		}
		got, _ := os.ReadFile(filepath.Join(root, "tickets", ".ergrc"))
		if string(got) != embedded {
			t.Errorf("a genuine upgrade was blocked -- .ergrc was not refreshed")
		}
	})

	t.Run("dateless stamp -> unchanged behaviour", func(t *testing.T) {
		// A store written by an erg predating the date: field. Absent provenance
		// is not evidence of a rollback, so this must upgrade exactly as today.
		setBuildDate(t, "2026-01-01T00:00:00Z")
		root := stampFixture(t, manifestWith(t, "", sha256hex([]byte(older))), older)

		created, refreshed, skipped, unchanged, err := installAssets(root, initAssetPaths, true, false)
		if err != nil {
			t.Fatalf("installAssets: %v", err)
		}
		if refreshed != 1 || skipped != 0 {
			t.Errorf("dateless stamp: expected today's behaviour (refreshed=1 skipped=0), got created=%d refreshed=%d skipped=%d unchanged=%d",
				created, refreshed, skipped, unchanged)
		}
	})

	t.Run("unknown stamp date -> unchanged behaviour", func(t *testing.T) {
		// buildManifest writes the literal "unknown" when buildDate is empty, so
		// an unstamped-build manifest must read as absent provenance, not as a
		// date that happens to sort above every ISO-8601 string.
		setBuildDate(t, "2026-01-01T00:00:00Z")
		root := stampFixture(t, manifestWith(t, "unknown", sha256hex([]byte(older))), older)

		_, refreshed, skipped, _, err := installAssets(root, initAssetPaths, true, false)
		if err != nil {
			t.Fatalf("installAssets: %v", err)
		}
		if refreshed != 1 || skipped != 0 {
			t.Errorf("unknown stamp date: expected today's behaviour (refreshed=1 skipped=0), got refreshed=%d skipped=%d", refreshed, skipped)
		}
	})

	t.Run("unparseable stamp date -> unchanged behaviour", func(t *testing.T) {
		// An unparseable date is absent provenance, not a rollback. Note the
		// direction of the trap: "y" sorts ABOVE every digit, so an unguarded
		// lexical compare would preserve here and quietly freeze every store
		// with a malformed stamp at its current assets.
		setBuildDate(t, "2026-01-01T00:00:00Z")
		root := stampFixture(t, manifestWith(t, "y", sha256hex([]byte(older))), older)

		_, refreshed, skipped, _, err := installAssets(root, initAssetPaths, true, false)
		if err != nil {
			t.Fatalf("installAssets: %v", err)
		}
		if refreshed != 1 || skipped != 0 {
			t.Errorf("unparseable stamp date: expected today's behaviour (refreshed=1 skipped=0), got refreshed=%d skipped=%d", refreshed, skipped)
		}
	})
}

// captureStderr runs fn with os.Stderr redirected to a temp file and returns
// what it wrote. The per-file init messages go to stderr, and the direction of a
// preserve or an overwrite is visible ONLY there -- the counts say how many, not
// which way.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stderr")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stderr
	os.Stderr = f
	defer func() {
		os.Stderr = prev
		f.Close()
	}()
	fn()
	f.Close()
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// TestInstallAssetsForceDowngradeLabel covers defect 3 of ticket 0279 on the one
// path where the revert is still performed: --force. The user asked for it, so
// it happens -- but the log must not call it a refresh. This is invisible to
// every count-based assertion: refreshed==1 either way.
func TestInstallAssetsForceDowngradeLabel(t *testing.T) {
	newer := "ERGRC FROM A LATER RELEASE -- pristine, not a local edit\n"
	setBuildDate(t, "2026-01-01T00:00:00Z")
	root := stampFixture(t, manifestWith(t, "2099-01-01T00:00:00Z", sha256hex([]byte(newer))), newer)

	// refuseDiverged=false is `erg init --force`.
	var refreshed int
	stderr := captureStderr(t, func() {
		var err error
		_, refreshed, _, _, err = installAssets(root, initAssetPaths, false, false)
		if err != nil {
			t.Fatalf("installAssets: %v", err)
		}
	})
	if refreshed != 1 {
		t.Errorf("--force must still perform the overwrite: expected refreshed=1, got %d", refreshed)
	}
	if !strings.Contains(stderr, "downgraded tickets/.ergrc") {
		t.Errorf("a forced revert was not reported as a downgrade: %q", stderr)
	}
	if strings.Contains(stderr, "refreshed tickets/.ergrc") {
		t.Errorf("a forced revert was narrated as a refresh: %q", stderr)
	}
	// The assets really were replaced, so the stamp must now say so.
	if got := readManifestDate(root); got != "2026-01-01T00:00:00Z" {
		t.Errorf("a completed --force downgrade must restamp the store, got date %q", got)
	}
}

// TestAssetDriftWarningsDirection is the ticket's fourth arm: it asserts the
// WARNING TEXT, not a count. Defect 1 of ticket 0279 is a message that names a
// direction the code never checked, and no exit-code assertion anywhere can see
// it -- an implementation that preserves correctly while still printing "binary
// upgraded since last init" passes every other test in this file.
func TestAssetDriftWarningsDirection(t *testing.T) {
	drifted := "SOME OTHER ERGRC -- hash differs from the embedded one\n"

	t.Run("rollback: names the remedy, never claims 'upgraded'", func(t *testing.T) {
		setBuildDate(t, "2026-01-01T00:00:00Z")
		root := stampFixture(t, manifestWith(t, "2099-01-01T00:00:00Z", sha256hex([]byte(drifted))), drifted)

		warnings := assetDriftWarnings(filepath.Join(root, "tickets"))
		if len(warnings) != 1 {
			t.Fatalf("expected exactly 1 drift warning, got %d: %v", len(warnings), warnings)
		}
		w := warnings[0]
		if strings.Contains(w, "upgraded") {
			t.Errorf("the binary is the OLDER side, yet the warning claims an upgrade: %q", w)
		}
		if !strings.Contains(w, "erg update") {
			t.Errorf("the rollback warning must name the remedy (erg update first): %q", w)
		}
	})

	t.Run("upgrade: keeps the stable signal erg update greps for", func(t *testing.T) {
		// update.go re-execs the NEW binary and greps its output for the OLD
		// binary's copy of assetDriftSignal, so this printed line is a
		// cross-version contract: it must keep containing the historical text.
		setBuildDate(t, "2026-01-01T00:00:00Z")
		root := stampFixture(t, manifestWith(t, "2020-01-01T00:00:00Z", sha256hex([]byte(drifted))), drifted)

		warnings := assetDriftWarnings(filepath.Join(root, "tickets"))
		if len(warnings) != 1 {
			t.Fatalf("expected exactly 1 drift warning, got %d: %v", len(warnings), warnings)
		}
		const historical = "differs from the .erg-assets stamp (binary upgraded since last init) -- run 'erg init' to refresh"
		if !strings.Contains(warnings[0], historical) {
			t.Errorf("upgrade-direction warning lost the cross-version grep target\n got: %q\nwant substring: %q", warnings[0], historical)
		}
	})
}

// TestInstallAssetsForceDowngradeLabelOnlyForStampedFile pins the per-file half
// of the direction rule on the --force / erg migrate leg (refuseDiverged ==
// false). "This store is a rollback" is a property of the MANIFEST; "this file
// is being reverted" is a property of the FILE. Only a file whose on-disk bytes
// match the newer stamp is demonstrably being reverted to an older version. A
// file with an ordinary local edit and no stamp entry at all has no established
// version ordering, so calling its overwrite a "downgrade" asserts a history the
// code never observed -- the same defect class as the ticket's defect 3, on the
// other leg. Its sibling three lines up (preserveRollback) already gets this
// right; this test is what keeps the two in step.
func TestInstallAssetsForceDowngradeLabelOnlyForStampedFile(t *testing.T) {
	setBuildDate(t, "2026-01-01T00:00:00Z")

	// .ergrc: pristine content from a LATER release, recorded in the stamp.
	// Overwriting it genuinely reverts it -- "downgraded" is true here.
	newer := "ERGRC FROM A LATER RELEASE -- pristine, not a local edit\n"
	// AGENTS.md: an ordinary local edit, with NO stamp entry. Nothing orders it
	// against the embedded copy, so "downgraded" would be an invention.
	edited := "AGENTS.MD WITH AN ORDINARY LOCAL EDIT -- never stamped\n"

	embedded, ok := bootstrapAsset("tickets/.ergrc")
	if !ok {
		t.Fatal("embedded .ergrc missing")
	}
	embeddedAgents, ok := bootstrapAsset("tickets/AGENTS.md")
	if !ok {
		t.Fatal("embedded AGENTS.md missing")
	}
	if newer == embedded || edited == embeddedAgents {
		t.Fatal("test fixture collides with embedded content")
	}

	// A manifest that stamps .ergrc ONLY: AGENTS.md is deliberately absent.
	manifest := "# erg provenance manifest -- do not edit\nrev: fixture\n" +
		"date: 2099-01-01T00:00:00Z\n" +
		"assets:\n" +
		"  .ergrc sha256:" + sha256hex([]byte(newer)) + "\n"

	root := stampFixture(t, manifest, newer)
	if err := os.WriteFile(filepath.Join(root, "tickets", "AGENTS.md"), []byte(edited), 0644); err != nil {
		t.Fatal(err)
	}

	stderr := captureStderr(t, func() {
		// refuseDiverged == false: the erg init --force / erg migrate leg.
		if _, _, _, _, err := installAssets(root, initAssetPaths, false, false); err != nil {
			t.Fatalf("installAssets: %v", err)
		}
	})

	if !strings.Contains(stderr, "downgraded tickets/.ergrc") {
		t.Errorf("a stamped file that IS being reverted lost its downgrade label: %q", stderr)
	}
	if strings.Contains(stderr, "downgraded tickets/AGENTS.md") {
		t.Errorf("an unstamped local edit was narrated as a version downgrade: %q", stderr)
	}
	if !strings.Contains(stderr, "refreshed tickets/AGENTS.md") {
		t.Errorf("an unstamped local edit must be reported as a refresh: %q", stderr)
	}
}

// stamplessFixture builds a store with NO .erg-assets manifest: AGENTS.md is
// laid down matching the embedded copy exactly (so it contributes nothing and
// the assertions can only be about .ergrc), and .ergrc gets the caller's
// content. It returns the root; the ticket store itself is root/tickets.
// Sibling of stampFixture, minus the stamp -- that absence is the whole point.
func stamplessFixture(t *testing.T, ergrcContent string) string {
	t.Helper()
	root := t.TempDir()
	ticketsDir := filepath.Join(root, "tickets")
	if err := os.MkdirAll(ticketsDir, 0755); err != nil {
		t.Fatal(err)
	}
	agents, ok := bootstrapAsset("tickets/AGENTS.md")
	if !ok {
		t.Fatal("embedded AGENTS.md missing")
	}
	if err := os.WriteFile(filepath.Join(ticketsDir, "AGENTS.md"), []byte(agents), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ticketsDir, ".ergrc"), []byte(ergrcContent), 0644); err != nil {
		t.Fatal(err)
	}
	// Guard: the fixture is only a *stampless* one if no manifest is present.
	// A later edit that starts stamping here would silently reroute every
	// assertion below into the stamped branch and prove nothing.
	if _, err := os.Stat(filepath.Join(ticketsDir, manifestName)); err == nil {
		t.Fatal("stamplessFixture wrote a manifest: the fixture no longer tests what it names")
	}
	return root
}

// TestAssetDriftWarningsReportsStamplessDivergence is ticket 0283's red step.
// Path A's defect is SILENCE, not destruction: with no .erg-assets stamp
// assetDriftWarnings returned nil the instant stamps == nil, and because
// corpusWarnings just appends its return value, erg check, erg init's chained
// check and erg update's post-swap hint all went quiet for that one cause.
//
// The assertion is therefore on message CONTENT, never on a count or an exit
// code: the return value is empty both before the fix and after a fix that
// gates wrongly, so only a substring assertion can see the difference.
//
// The two arms are ordered deliberately. The noisy arm must be shown to FIRE
// first; only then does the silent arm's silence mean anything, since an
// implementation that never warns at all passes the silent arm trivially.
func TestAssetDriftWarningsReportsStamplessDivergence(t *testing.T) {
	embedded, ok := bootstrapAsset("tickets/.ergrc")
	if !ok {
		t.Fatal("embedded .ergrc missing")
	}

	t.Run("noisy arm: diverged asset, no stamp, reports the condition", func(t *testing.T) {
		diverged := "OLD SHIPPED ERGRC -- a pristine prior release\n"
		if diverged == embedded {
			t.Fatal("test fixture collides with embedded content")
		}
		root := stamplessFixture(t, diverged)

		got := strings.Join(assetDriftWarnings(filepath.Join(root, "tickets")), "\n")
		if !strings.Contains(got, assetStamplessSignal) {
			t.Fatalf("a stampless store with a diverged .ergrc must report it\n got: %q\nwant substring: %q", got, assetStamplessSignal)
		}
		if !strings.Contains(got, ".ergrc") {
			t.Errorf("the report must name the asset it is about: %q", got)
		}
		if !strings.Contains(got, "erg init") {
			t.Errorf("the report must name the remedy: %q", got)
		}
		if strings.Contains(got, assetDriftSignal) || strings.Contains(got, assetRollbackSignal) {
			t.Errorf("there is no stamp here, so no stamp-relative claim may be made: %q", got)
		}
	})

	t.Run("silent arm: asset matches the embedded copy exactly, no stamp", func(t *testing.T) {
		// The invariant: a store that never adopted erg's asset management is
		// never nagged. The gate is real divergence, not the absence of a
		// manifest -- without this arm an implementation that reports
		// unconditionally on "no manifest" passes the noisy arm and proves
		// nothing.
		root := stamplessFixture(t, embedded)

		for _, w := range assetDriftWarnings(filepath.Join(root, "tickets")) {
			if strings.Contains(w, assetStamplessSignal) {
				t.Errorf("assets matching the embedded copy exactly must stay silent: %q", w)
			}
		}
	})

	t.Run("silent arm: no assets on disk at all, no stamp", func(t *testing.T) {
		// The store that genuinely has nothing to compare -- the shape the
		// pre-0283 comment in update.go mistook for the stampless case at
		// large. This one really is silent, and stays so.
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "tickets"), 0755); err != nil {
			t.Fatal(err)
		}
		if w := assetDriftWarnings(filepath.Join(root, "tickets")); len(w) != 0 {
			t.Errorf("an empty store has nothing to compare and must stay silent, got: %v", w)
		}
	})
}

// vendoredFixture builds a store whose managed assets are byte-identical to the
// embedded copies -- so .ergrc and AGENTS.md contribute nothing and every
// assertion below can only be about the vendored helper -- and whose
// tickets/erg-github holds the caller's content. An empty ergGithub means the
// file is not written at all: that is the non-forge adopter, and it is a case,
// not a degenerate one. withManifest decides whether a .erg-assets stamp is
// present, because the vendored compare must survive BOTH branches of
// assetDriftWarnings: the stampless branch is reached by an early return, and
// an implementation that appends the vendored notes after that return reports
// nothing for exactly the stores most likely to be stale.
func vendoredFixture(t *testing.T, ergGithub string, withManifest bool) string {
	t.Helper()
	root := t.TempDir()
	ticketsDir := filepath.Join(root, "tickets")
	if err := os.MkdirAll(ticketsDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range initAssetPaths {
		content, ok := bootstrapAsset(rel)
		if !ok {
			t.Fatalf("embedded asset missing: %s", rel)
		}
		name := strings.TrimPrefix(rel, "tickets/")
		if err := os.WriteFile(filepath.Join(ticketsDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if ergGithub != "" {
		if err := os.WriteFile(filepath.Join(ticketsDir, "erg-github"), []byte(ergGithub), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if withManifest {
		body, err := buildManifest(nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ticketsDir, manifestName), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Guard: the fixture only isolates the vendored helper if the managed
	// assets are silent. An edit that lets .ergrc or AGENTS.md diverge here
	// would make every assertion below pass on the wrong warning.
	for _, w := range assetDriftWarnings(ticketsDir) {
		if strings.Contains(w, assetDriftSignal) || strings.Contains(w, assetRollbackSignal) || strings.Contains(w, assetStamplessSignal) {
			t.Fatalf("vendoredFixture is not isolating the vendored helper: a managed asset spoke: %q", w)
		}
	}
	return root
}

// TestAssetDriftWarningsReportsVendoredStaleness is ticket 0282's red step.
//
// tickets/erg-github is vendored: a plain committed POSIX-sh helper that
// travels with the clone. Because it was in none of erg's asset lists, it had
// no staleness channel at all -- not the .erg-assets stamp, not isCleanUpgrade,
// not assetDriftWarnings -- so an adopter carrying a year-old copy was told
// nothing, ever. The concrete bite: ticket 0255 fixed a ticket-ID extraction
// gap in this script's cmd_verify(), and the repo where that bug actually fired
// (aedist-technical-report) carries its own committed copy the fix never
// reaches.
//
// The defect is SILENCE, so the assertions are on message CONTENT -- never on a
// count, a length, or an exit code. Warnings are non-fatal by construction:
// erg check's exit code is identical before and after this fix, and a test
// reading it would pass in both worlds. The same holds for len(warnings): the
// noisy arm must be shown to FIRE before either silent arm means anything, or
// an implementation that never reports at all passes both of them trivially.
func TestAssetDriftWarningsReportsVendoredStaleness(t *testing.T) {
	embedded, ok := bootstrapAsset("tickets/erg-github")
	if !ok {
		t.Fatal("embedded erg-github missing: this binary ships no reference copy to compare against")
	}

	// A plausible year-old vendored copy: a real script, just not this one.
	const stale = "#!/bin/sh\n# erg-github -- an old vendored copy, predating the 0255 fix\nexit 0\n"
	if stale == embedded {
		t.Fatal("test fixture collides with the embedded content")
	}

	t.Run("noisy arm: a stale vendored copy is reported", func(t *testing.T) {
		for _, withManifest := range []bool{true, false} {
			name := "stamped store"
			if !withManifest {
				name = "stampless store"
			}
			t.Run(name, func(t *testing.T) {
				root := vendoredFixture(t, stale, withManifest)
				got := strings.Join(assetDriftWarnings(filepath.Join(root, "tickets")), "\n")
				if !strings.Contains(got, vendoredDriftSignal) {
					t.Fatalf("a vendored erg-github differing from the shipped copy must be reported\n got: %q\nwant substring: %q", got, vendoredDriftSignal)
				}
				if !strings.Contains(got, "erg-github") {
					t.Errorf("the report must name the file it is about: %q", got)
				}
			})
		}
	})

	t.Run("noisy arm: the report claims no stamp-relative direction", func(t *testing.T) {
		// erg never wrote this file, so the .erg-assets stamp says nothing
		// about it and the report must not borrow the managed assets'
		// vocabulary. "run 'erg init' to refresh" would be a false promise:
		// init does not touch a vendored file and never will.
		root := vendoredFixture(t, stale, true)
		got := strings.Join(assetDriftWarnings(filepath.Join(root, "tickets")), "\n")
		if strings.Contains(got, assetDriftSignal) || strings.Contains(got, assetRollbackSignal) || strings.Contains(got, assetStamplessSignal) {
			t.Errorf("a vendored file must not be reported through the managed-asset signals: %q", got)
		}
	})

	t.Run("silent arm: a current vendored copy says nothing", func(t *testing.T) {
		for _, withManifest := range []bool{true, false} {
			root := vendoredFixture(t, embedded, withManifest)
			for _, w := range assetDriftWarnings(filepath.Join(root, "tickets")) {
				if strings.Contains(w, vendoredDriftSignal) {
					t.Errorf("a copy identical to the shipped one must stay silent (manifest=%v): %q", withManifest, w)
				}
			}
		}
	})

	t.Run("silent arm: no vendored copy at all says nothing", func(t *testing.T) {
		// The invariant that keeps the forge layer OPTIONAL: a repo that never
		// adopted erg-github has nothing to compare and must never be nagged
		// into adopting it. Without this arm, an implementation that reports
		// on absence passes the noisy arm and wedges every non-forge adopter.
		for _, withManifest := range []bool{true, false} {
			root := vendoredFixture(t, "", withManifest)
			if _, err := os.Stat(filepath.Join(root, "tickets", "erg-github")); !os.IsNotExist(err) {
				t.Fatalf("fixture guard: erg-github must be absent for this arm (err=%v)", err)
			}
			for _, w := range assetDriftWarnings(filepath.Join(root, "tickets")) {
				if strings.Contains(w, vendoredDriftSignal) {
					t.Errorf("a store with no erg-github must stay silent (manifest=%v): %q", withManifest, w)
				}
			}
		}
	})

	t.Run("the shipped reference is the deployed helper, not a stub", func(t *testing.T) {
		// The compare is only worth anything if the embedded blob is the real
		// script. A truncated or placeholder asset would make every adopter
		// copy look stale -- the loudest possible false positive.
		if !strings.Contains(embedded, "cmd_verify") || !strings.Contains(embedded, "#!/bin/sh") {
			t.Errorf("embedded erg-github does not look like the forge helper (len=%d)", len(embedded))
		}
	})
}

// captureStdout is captureStderr's sibling for the channels that go to stdout:
// installAssets' dry-run preview lines and `erg init --show`. Kept separate
// rather than generalised into one helper taking a **os.File, because the two
// call sites read better named after the stream they are about and the saving
// would be three lines.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdout")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stdout
	os.Stdout = f
	defer func() {
		os.Stdout = prev
		f.Close()
	}()
	fn()
	f.Close()
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// TestInitDoesNotStampAPreservedLocalEdit is ticket 0292 defect 1's red step,
// and it is the state-corrupting one: `erg init` preserved a locally-edited
// asset (right) and then wrote a manifest stamping it with the EMBEDDED hash
// (wrong), so the manifest certified a customised file as byte-identical to
// what the binary ships. From the next run on, the stamped branch compared
// embedded against embedded, found them equal, and the condition went
// permanently silent with the divergence still on disk.
//
// The assertion is on the manifest CONTENT and on the message that follows,
// never on a count or an exit code: installAssets returns skipped=1 before and
// after the fix, and `erg check` exits 0 in both worlds. Only reading what was
// recorded, and what the next check says, can see the difference.
//
// Arm order is deliberate. The pre-init probe is the positive control for the
// whole test: it must be shown to FIRE before the post-init probe's persistence
// means anything, since a build that never reports at all would satisfy a
// post-init assertion phrased as "still reports" only by accident of the
// substring never appearing.
func TestInitDoesNotStampAPreservedLocalEdit(t *testing.T) {
	embedded, ok := bootstrapAsset("tickets/.ergrc")
	if !ok {
		t.Fatal("embedded .ergrc missing")
	}
	agents, ok := bootstrapAsset("tickets/AGENTS.md")
	if !ok {
		t.Fatal("embedded AGENTS.md missing")
	}
	// The documented, encouraged case: a store that customised .ergrc (AGENTS.md
	// sends readers there to define Label: values) and never stamped it.
	const customised = "# locally customised .ergrc -- never shipped by any erg\nlabels = deferred\n"
	if customised == embedded {
		t.Fatal("test fixture collides with embedded content")
	}

	t.Run("a preserved local edit is not stamped, and stays reportable", func(t *testing.T) {
		root := stamplessFixture(t, customised)
		ticketsDir := filepath.Join(root, "tickets")

		// Positive control, first: the condition is reported BEFORE init.
		before := strings.Join(assetDriftWarnings(ticketsDir), "\n")
		if !strings.Contains(before, assetStamplessSignal) {
			t.Fatalf("control: the stampless divergence must be reported before init\n got: %q\nwant substring: %q", before, assetStamplessSignal)
		}

		if _, _, skipped, _, err := installAssets(root, initAssetPaths, true, false); err != nil {
			t.Fatalf("installAssets: %v", err)
		} else if skipped != 1 {
			t.Fatalf("fixture guard: expected .ergrc to be preserved (skipped=1), got %d", skipped)
		}

		// The customisation is still on disk -- init's DECISION is not what
		// this ticket changes, and an arm that lost it would be testing a
		// clobber, not a stamp.
		if onDisk, _ := os.ReadFile(filepath.Join(ticketsDir, ".ergrc")); string(onDisk) != customised {
			t.Fatalf("fixture guard: init overwrote the customised .ergrc")
		}

		stamps := readManifest(root)
		if got, present := stamps[".ergrc"]; present {
			t.Errorf("init stamped a file it preserved: .ergrc recorded as %q, embedded hash is %q", got, sha256hex([]byte(embedded)))
		}

		after := strings.Join(assetDriftWarnings(ticketsDir), "\n")
		if !strings.Contains(after, assetStamplessSignal) {
			t.Errorf("following the advice erg check prints silenced the condition\n got: %q\nwant substring: %q", after, assetStamplessSignal)
		}
		if !strings.Contains(after, ".ergrc") {
			t.Errorf("the surviving report must still name the asset: %q", after)
		}
	})

	t.Run("positive control: an asset init did lay down IS stamped", func(t *testing.T) {
		// Without this arm, "stop writing manifests at all" passes the arm
		// above. AGENTS.md here is byte-identical to the embedded copy, so
		// init verified it this run and the manifest may say so.
		root := stamplessFixture(t, customised)
		if _, _, _, _, err := installAssets(root, initAssetPaths, true, false); err != nil {
			t.Fatalf("installAssets: %v", err)
		}
		stamps := readManifest(root)
		if got := stamps["AGENTS.md"]; got != sha256hex([]byte(agents)) {
			t.Errorf("AGENTS.md was installed this run and must be stamped: got %q, want %q", got, sha256hex([]byte(agents)))
		}
	})

	t.Run("a prior stamp for a preserved file is carried, not dropped", func(t *testing.T) {
		// "Don't stamp what you didn't touch" cuts both ways. The previous
		// manifest's entry is evidence about a PAST install -- it is what lets
		// the next run say "has local edits" and mean it. Dropping it would
		// swap one false record for a lost one, and the run after a re-init
		// would downgrade its own verdict to "reason unknown".
		older := "ERGRC FROM AN EARLIER RELEASE -- pristine, not a local edit\n"
		priorStamp := sha256hex([]byte(older))
		// The stamp records the earlier release; the disk now holds a local
		// edit, so the two differ and the file is preserved.
		root := stampFixture(t, manifestWith(t, "", priorStamp), customised)

		if _, _, skipped, _, err := installAssets(root, initAssetPaths, true, false); err != nil {
			t.Fatalf("installAssets: %v", err)
		} else if skipped != 1 {
			t.Fatalf("fixture guard: expected .ergrc preserved, got skipped=%d", skipped)
		}
		if got := readManifest(root)[".ergrc"]; got != priorStamp {
			t.Errorf("the preserving run rewrote the prior stamp: got %q, want the carried %q", got, priorStamp)
		}
		stderr := captureStderr(t, func() {
			if _, _, _, _, err := installAssets(root, initAssetPaths, true, false); err != nil {
				t.Fatalf("second installAssets: %v", err)
			}
		})
		if !strings.Contains(stderr, "local edits") {
			t.Errorf("the second run lost the evidence for its own verdict: %q", stderr)
		}
	})
}

// TestAssetDriftWarningsGatesPerAssetNotPerStore is ticket 0292 defect 3's red
// step: the same silence 0283 closed, one level down. The stamped branch skipped
// any asset with no entry (`if !ok || stamp == "" { continue }`) and did not fall
// back to the stampless compare, while parseManifest returns non-nil as soon as
// ONE line parses. So a manifest stamping .ergrc but not AGENTS.md silenced
// AGENTS.md divergence completely: not drift-warned (no stamp for it), not
// stampless-warned (the manifest is not nil).
//
// The gate under test is "is THIS asset stamped", not "does a manifest exist",
// so the fixture must carry a manifest that parses -- otherwise the assertion
// routes through the stampless branch and proves nothing about the stamped one.
func TestAssetDriftWarningsGatesPerAssetNotPerStore(t *testing.T) {
	ergrc, ok := bootstrapAsset("tickets/.ergrc")
	if !ok {
		t.Fatal("embedded .ergrc missing")
	}
	agentsEmbedded, ok := bootstrapAsset("tickets/AGENTS.md")
	if !ok {
		t.Fatal("embedded AGENTS.md missing")
	}

	// A manifest that stamps .ergrc at its embedded hash and says nothing at
	// all about AGENTS.md. This is what `erg init` itself now writes when it
	// preserved AGENTS.md, so the shape is reachable, not contrived.
	partial := "# erg provenance manifest -- do not edit\nrev: fixture\nassets:\n" +
		"  .ergrc sha256:" + sha256hex([]byte(ergrc)) + "\n"

	// build lays down a store with the partial manifest, a pristine .ergrc and
	// the caller's AGENTS.md.
	build := func(t *testing.T, agentsContent string) string {
		t.Helper()
		root := t.TempDir()
		ticketsDir := filepath.Join(root, "tickets")
		if err := os.MkdirAll(ticketsDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ticketsDir, ".ergrc"), []byte(ergrc), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ticketsDir, "AGENTS.md"), []byte(agentsContent), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ticketsDir, manifestName), []byte(partial), 0644); err != nil {
			t.Fatal(err)
		}
		// Fixture guard: the manifest must PARSE, or every assertion below
		// silently reroutes into the stampless branch and this test stops
		// being about the stamped one.
		stamps := readManifestFile(filepath.Join(ticketsDir, manifestName))
		if stamps == nil {
			t.Fatal("fixture guard: the partial manifest does not parse, so the stamped branch is never reached")
		}
		if _, present := stamps["AGENTS.md"]; present {
			t.Fatal("fixture guard: the partial manifest is not partial -- AGENTS.md is stamped")
		}
		return root
	}

	t.Run("noisy arm: an unstamped asset in a stamped store is reported", func(t *testing.T) {
		diverged := "# AGENTS.md, locally rewritten, never stamped\n"
		if diverged == agentsEmbedded {
			t.Fatal("test fixture collides with embedded content")
		}
		root := build(t, diverged)
		got := strings.Join(assetDriftWarnings(filepath.Join(root, "tickets")), "\n")
		if !strings.Contains(got, assetStamplessSignal) {
			t.Fatalf("an asset missing from an otherwise-valid manifest must be reported\n got: %q\nwant substring: %q", got, assetStamplessSignal)
		}
		if !strings.Contains(got, "AGENTS.md") {
			t.Errorf("the report must name the unstamped asset: %q", got)
		}
	})

	t.Run("silent control: the same partial manifest, matching content", func(t *testing.T) {
		// The sibling that keeps the noisy arm honest. Identical fixture
		// shape, identical gate, different bytes on disk -- so an
		// implementation that reports on "unstamped" rather than on
		// "unstamped AND diverged" fails here and passes above.
		root := build(t, agentsEmbedded)
		for _, w := range assetDriftWarnings(filepath.Join(root, "tickets")) {
			if strings.Contains(w, assetStamplessSignal) {
				t.Errorf("an unstamped asset matching the embedded copy must stay silent: %q", w)
			}
		}
	})

	t.Run("the stamped asset alongside it keeps its own branch", func(t *testing.T) {
		// .ergrc is stamped at the embedded hash, so it has nothing to say --
		// through EITHER branch. A fix that routed every asset through the
		// stampless compare would still be silent here (disk matches
		// embedded), so this arm is about the drift signal staying unclaimed.
		root := build(t, agentsEmbedded)
		got := strings.Join(assetDriftWarnings(filepath.Join(root, "tickets")), "\n")
		if strings.Contains(got, assetDriftSignal) || strings.Contains(got, assetRollbackSignal) {
			t.Errorf("a current stamp must raise no drift claim: %q", got)
		}
	})
}

// TestInstallAssetsPreserveReasonIsObserved is ticket 0292 defect 2's second
// half: `init: tickets/.ergrc has local edits -- preserving` was printed flatly
// for every preserved file, including one whose provenance the code never
// observed. With no stamp for the asset, all the compare established is that
// the bytes differ from what this binary ships -- "you edited it" is an
// attribution, and it is exactly the attribution assetStamplessSignal exists to
// refuse. It is also channel 2 of ticket 0283's exit criterion 1: this per-file
// line is how `erg init` and `erg init -n` report a stampless store's condition.
//
// The stamped arm is the control. Without it, a fix that simply deleted the
// "local edits" wording everywhere would pass the unstamped arm, and the
// justified verdict -- the file differs from the stamp recording what init
// itself last wrote -- would be lost with the unjustified one.
func TestInstallAssetsPreserveReasonIsObserved(t *testing.T) {
	const customised = "# locally customised .ergrc -- never shipped by any erg\nlabels = deferred\n"
	embedded, ok := bootstrapAsset("tickets/.ergrc")
	if !ok {
		t.Fatal("embedded .ergrc missing")
	}
	if customised == embedded {
		t.Fatal("test fixture collides with embedded content")
	}

	t.Run("no stamp for the asset: no attribution is claimed", func(t *testing.T) {
		root := stamplessFixture(t, customised)
		stderr := captureStderr(t, func() {
			if _, _, skipped, _, err := installAssets(root, initAssetPaths, true, false); err != nil {
				t.Fatalf("installAssets: %v", err)
			} else if skipped != 1 {
				t.Fatalf("fixture guard: expected .ergrc preserved, got skipped=%d", skipped)
			}
		})
		if strings.Contains(stderr, "local edits") {
			t.Errorf("init asserted an edit it never observed: %q", stderr)
		}
		if !strings.Contains(stderr, ".ergrc") {
			t.Errorf("the per-file line must name the asset: %q", stderr)
		}
		if !strings.Contains(stderr, "no usable .erg-assets stamp") {
			t.Errorf("init must report the condition it actually observed: %q", stderr)
		}
	})

	t.Run("dry run reports the same condition on stdout", func(t *testing.T) {
		// Channel 2 of 0283's criterion covers `erg init -n` too, and the
		// dry-run leg prints its own short label from a separate string.
		root := stamplessFixture(t, customised)
		stdout := captureStdout(t, func() {
			if _, _, _, _, err := installAssets(root, initAssetPaths, true, true); err != nil {
				t.Fatalf("installAssets: %v", err)
			}
		})
		if !strings.Contains(stdout, "would preserve") || !strings.Contains(stdout, ".ergrc") {
			t.Fatalf("the dry run must still preview the preservation: %q", stdout)
		}
		if strings.Contains(stdout, "(local edits)") {
			t.Errorf("the dry-run label claims an edit no stamp attests: %q", stdout)
		}
		if !strings.Contains(stdout, "no usable stamp") {
			t.Errorf("the dry-run label must name the observed condition: %q", stdout)
		}
	})

	t.Run("control: a stamp DOES justify the local-edit verdict", func(t *testing.T) {
		older := "ERGRC FROM AN EARLIER RELEASE -- pristine, not a local edit\n"
		root := stampFixture(t, manifestWith(t, "", sha256hex([]byte(older))), customised)
		stderr := captureStderr(t, func() {
			if _, _, skipped, _, err := installAssets(root, initAssetPaths, true, false); err != nil {
				t.Fatalf("installAssets: %v", err)
			} else if skipped != 1 {
				t.Fatalf("fixture guard: expected .ergrc preserved, got skipped=%d", skipped)
			}
		})
		if !strings.Contains(stderr, "local edits") {
			t.Errorf("a file differing from its own stamp IS a local edit and must be named one: %q", stderr)
		}
	})
}

// TestInitShowPrintsTheEmbeddedAsset is ticket 0292 defect 4's red step. The
// stampless NOTE tells a reader their asset differs from the copy the binary
// ships, and until now no erg subcommand could show them that copy: `erg init
// -n` reports only THAT a file differs, and spec/integration dump different
// embedded files entirely. The message therefore pointed at the store's version
// control, which a directory under none -- a shape erg check accepts -- does not
// have, and which an untracked asset does not have either.
//
// The assertion is byte equality against bootstrapAsset, not a substring: the
// whole use of the flag is piping it into a diff or a checksum, and a trailing
// banner or a missing final newline breaks that while looking fine on screen.
func TestInitShowPrintsTheEmbeddedAsset(t *testing.T) {
	for _, rel := range showableAssetPaths() {
		name := strings.TrimPrefix(rel, "tickets/")
		t.Run(name, func(t *testing.T) {
			want, ok := bootstrapAsset(rel)
			if !ok {
				t.Fatalf("embedded asset missing: %s", rel)
			}
			var rc int
			got := captureStdout(t, func() { rc = cmdInit([]string{"--show", name}) })
			if rc != 0 {
				t.Fatalf("erg init --show %s exited %d", name, rc)
			}
			if got != want {
				t.Errorf("--show %s is not byte-identical to the embedded copy: got %d bytes, want %d", name, len(got), len(want))
			}
		})
	}

	t.Run("the asset name is not read as DIR", func(t *testing.T) {
		// `erg init --show .ergrc` must never be parsed as an init of ./.ergrc.
		// The flag consumes its argument; a parser that let it fall through to
		// positional would try to initialise a directory named .ergrc, and the
		// only visible symptom would be a "binary not found" that looks like
		// an unrelated environment problem. The --show=NAME spelling is the
		// same contract written the other way and must agree.
		want, _ := bootstrapAsset("tickets/.ergrc")
		for _, args := range [][]string{{"--show", ".ergrc"}, {"--show=.ergrc"}, {"--show", "tickets/.ergrc"}} {
			var rc int
			got := captureStdout(t, func() { rc = cmdInit(args) })
			if rc != 0 || got != want {
				t.Errorf("%v: rc=%d, %d bytes, want 0 and %d bytes", args, rc, len(got), len(want))
			}
		}
	})

	t.Run("an unknown asset name is an error naming what is available", func(t *testing.T) {
		var rc int
		stderr := captureStderr(t, func() {
			captureStdout(t, func() { rc = cmdInit([]string{"--show", "no-such-asset"}) })
		})
		if rc == 0 {
			t.Errorf("--show with an unknown name must not report success")
		}
		if !strings.Contains(stderr, ".ergrc") || !strings.Contains(stderr, "AGENTS.md") {
			t.Errorf("the error must name the assets this binary can show: %q", stderr)
		}
	})

	t.Run("--show with no name is an error, not an init", func(t *testing.T) {
		var rc int
		stderr := captureStderr(t, func() {
			captureStdout(t, func() { rc = cmdInit([]string{"--show"}) })
		})
		if rc == 0 {
			t.Errorf("a bare --show must not fall through to an init of the current directory")
		}
		if !strings.Contains(stderr, "--show") {
			t.Errorf("the error must name the flag it is about: %q", stderr)
		}
	})
}

// TestAssetStamplessSignalKeepsItsShippedPrefix pins the cross-version contract
// by BYTES, against a hardcoded snapshot, because every other assertion in this
// package compares assetStamplessSignal with itself and is therefore invariant
// under any rewording -- self-referential, zero protection (PR #360 round 1).
//
// The premise is measured, not assumed: this exact text is inside the committed
// bootstrap binary, so a store that ran erg update carries an OLD binary that
// greps update.go's copy of it against a NEW binary's `erg check` output. Reword
// the front and the old side stops recognising its own signal, with no error
// anywhere. Extending the END is always safe; that is what HasPrefix allows.
//
// tests/test_check.sh guards the same contract from the other side, but by two
// PROSE ANCHORS inside the string -- a mid-string reword that respects both
// would pass there and still break the real grep. This test closes that gap:
// it is a byte comparison over the whole historical prefix.
//
// If this test fails, do not update the constant below to match. The constant
// below is the record of what shipped; the code is what must be repaired.
func TestAssetStamplessSignalKeepsItsShippedPrefix(t *testing.T) {
	// Verbatim from the binary committed at origin/main before ticket 0292.
	const shipped = "no .erg-assets stamp -- cannot tell whether this is a clean upgrade or a local edit; its git history can, and 'erg init' preserves the file either way but stamps it as if shipped"

	if !strings.HasPrefix(assetStamplessSignal, shipped) {
		t.Fatalf("assetStamplessSignal no longer starts with the text an already-shipped binary greps for.\n got: %q\nwant prefix: %q", assetStamplessSignal, shipped)
	}
	// And the printed line must carry it too -- a constant kept intact while
	// the format string around it drops or reflows it would pass the check
	// above and still break the grep.
	root := stamplessFixture(t, "# a .ergrc this binary does not ship\n")
	got := strings.Join(assetDriftWarnings(filepath.Join(root, "tickets")), "\n")
	if !strings.Contains(got, shipped) {
		t.Errorf("the PRINTED line dropped the shipped prefix: %q", got)
	}
}

// TestInstallAssetsRejectsAStampThatIsNotAHash is PR #360 round 1's red-team
// finding, turned into a guard. Carrying a preserved asset's prior stamp
// forward (ticket 0292, defect 1) replaced an unconditional restamp, and that
// restamp had been self-healing a case nobody had noticed: a manifest entry
// that parses but is not a SHA-256 -- truncated mid-line, or hand-edited.
//
// Before the fix, such an entry was carried verbatim into every later manifest,
// FOREVER, and because installAssets read stamp presence as the evidence for
// "has local edits", every subsequent run asserted an edit nothing had observed
// -- the exact false-reason class this ticket set out to remove, reached
// through a different door. The trade was a wrong-attribution defect for a
// never-self-heals one.
//
// Both halves are asserted, because fixing either alone leaves the other: the
// garbage must not survive the run, AND the verdict must not rest on it.
func TestInstallAssetsRejectsAStampThatIsNotAHash(t *testing.T) {
	const customised = "# locally customised .ergrc -- never shipped by any erg\nlabels = deferred\n"

	t.Run("a truncated stamp is dropped, not carried forward", func(t *testing.T) {
		truncated := sha256hex([]byte(customised))[:20]
		root := stampFixture(t, manifestWith(t, "", truncated), customised)
		// Fixture guard: the entry must PARSE, or this exercises the
		// no-manifest path and proves nothing about carry-forward.
		if got := readManifest(root)[".ergrc"]; got != truncated {
			t.Fatalf("fixture guard: the truncated stamp did not parse (got %q)", got)
		}

		stderr := captureStderr(t, func() {
			if _, _, skipped, _, err := installAssets(root, initAssetPaths, true, false); err != nil {
				t.Fatalf("installAssets: %v", err)
			} else if skipped != 1 {
				t.Fatalf("fixture guard: expected .ergrc preserved, got skipped=%d", skipped)
			}
		})
		if got, present := readManifest(root)[".ergrc"]; present {
			t.Errorf("a stamp that is not a hash survived the run: %q", got)
		}
		if strings.Contains(stderr, "has local edits") {
			t.Errorf("the verdict rested on a stamp that is not evidence: %q", stderr)
		}
		if !strings.Contains(stderr, "no usable .erg-assets stamp") {
			t.Errorf("init must name the condition it observed: %q", stderr)
		}
	})

	t.Run("control: a well-formed stamp is still carried and still believed", func(t *testing.T) {
		// Without this arm, "drop every carried stamp" passes above and
		// silently deletes the evidence defect 1's fix exists to keep.
		older := "ERGRC FROM AN EARLIER RELEASE -- pristine, not a local edit\n"
		good := sha256hex([]byte(older))
		root := stampFixture(t, manifestWith(t, "", good), customised)

		stderr := captureStderr(t, func() {
			if _, _, _, _, err := installAssets(root, initAssetPaths, true, false); err != nil {
				t.Fatalf("installAssets: %v", err)
			}
		})
		if got := readManifest(root)[".ergrc"]; got != good {
			t.Errorf("a well-formed prior stamp was dropped: got %q, want %q", got, good)
		}
		if !strings.Contains(stderr, "has local edits") {
			t.Errorf("a file differing from a real stamp is a local edit: %q", stderr)
		}
	})

	t.Run("looksLikeAssetHash: the shapes it must separate", func(t *testing.T) {
		real := sha256hex([]byte("anything"))
		for _, ok := range []string{real, strings.Repeat("0", 64), strings.Repeat("f", 64)} {
			if !looksLikeAssetHash(ok) {
				t.Errorf("rejected a valid hash shape: %q", ok)
			}
		}
		bad := []string{
			"",
			real[:63],               // one short
			real + "0",              // one long
			strings.ToUpper(real),   // uppercase: sha256hex never emits it
			strings.Repeat("g", 64), // right length, not hex
			real[:62] + "zz",        // right length, tail not hex
			"sha256:" + real[:57],   // a prefix pasted in with its label
		}
		for _, s := range bad {
			if looksLikeAssetHash(s) {
				t.Errorf("accepted something that is not a hash: %q", s)
			}
		}
	})
}

// TestStampNotAHashIsReadTheSameWayEverywhere is PR #360 round 2's blocker,
// turned into the guard the round-1 fix should have shipped with. Round 1 wired
// looksLikeAssetHash into two readers of "is this stamp evidence" and a comment
// claimed that was all of them. There was a third: managedAssetWarnings still
// gated on `stamp == ""`, so a manifest holding a non-hash stamp sent it into
// the STAMPED branch, where the stamp cannot equal the embedded hash -- and
// `erg check` printed a confident directional "binary upgraded since last init
// -- run 'erg init' to refresh" about a file `erg init`, on the same store,
// refuses to attribute and refuses to refresh.
//
// Two commands contradicting each other on one store is worse than either
// being wrong alone, so the assertion is on AGREEMENT, not on either message in
// isolation: whatever the reading, check and init must reach it together. That
// is also what makes this test survive a future change of wording.
//
// Fixture note, recorded because the first attempt at this repro produced a
// clean-looking false negative: the manifest separator is the literal
// " sha256:" (see parseManifest). A line written as "name hash" does not parse,
// the store falls through to the no-manifest branch, and the benign stampless
// NOTE appears instead of the WARN -- which reads exactly like "no defect
// here". The fixture guard below is what refuses that reading.
func TestStampNotAHashIsReadTheSameWayEverywhere(t *testing.T) {
	const customised = "# locally customised .ergrc -- never shipped by any erg\nlabels = deferred\n"
	// Non-empty, parses as an entry, is not a SHA-256: a manifest truncated
	// mid-line, or edited by hand.
	const notAHash = "c6c529c913c76291ab93"

	root := stampFixture(t, manifestWith(t, "", notAHash), customised)
	ticketsDir := filepath.Join(root, "tickets")

	// Fixture guard: the entry must PARSE and the manifest must be read as
	// present. Without this the test silently exercises the no-manifest path
	// and proves nothing about the stamped branch.
	stamps := readManifestFile(filepath.Join(ticketsDir, manifestName))
	if stamps == nil {
		t.Fatal("fixture guard: the manifest does not parse, so the stamped branch is never reached")
	}
	if stamps[".ergrc"] != notAHash {
		t.Fatalf("fixture guard: the malformed stamp did not survive the parse (got %q)", stamps[".ergrc"])
	}

	got := strings.Join(assetDriftWarnings(ticketsDir), "\n")
	if strings.Contains(got, assetDriftSignal) || strings.Contains(got, assetRollbackSignal) {
		t.Errorf("erg check made a directional stamp claim on a stamp that is not a hash: %q", got)
	}
	if !strings.Contains(got, assetStamplessSignal) {
		t.Errorf("the divergence must still be reported, through the honest channel: %q", got)
	}

	stderr := captureStderr(t, func() {
		if _, _, _, _, err := installAssets(root, initAssetPaths, true, false); err != nil {
			t.Fatalf("installAssets: %v", err)
		}
	})
	// The agreement assertion: check declined to attribute, so init must too.
	if strings.Contains(stderr, "has local edits") {
		t.Errorf("erg init attributed what erg check declined to attribute: %q", stderr)
	}
	if !strings.Contains(stderr, "no usable .erg-assets stamp") {
		t.Errorf("init must name the same condition check named: %q", stderr)
	}
}
