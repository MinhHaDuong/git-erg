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
		body, err := buildManifest()
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
