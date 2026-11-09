package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Unit tests for ID claims (ticket 0306). The git-level behaviour (add/add
// conflict on merge and rebase, the hook, the silenced merges) is in
// tests/test_claims.sh.

func writeClaimTestTicket(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "%erg 0.1\nTitle: T\nCreated: 2026-11-08\nAuthor: a\n\n--- log ---\n2026-11-08T00:00Z a created\n\n--- body ---\n"
	if strings.Contains(name, "closed/") {
		content = "%erg 0.1\nTitle: T\nCreated: 2026-11-08\nAuthor: a\nClosed: done\n\n--- log ---\n2026-11-08T00:00Z a created\n\n--- body ---\n"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// claimStore makes dir a compliant store with plain files, as a hand would:
// .claims/.keep and, for every ticket already in dir that has no claim, one
// "SLUG 00000000" claim. Fixtures call it so a test fails for its own defect,
// not for the store's missing .claims/ (ticket 0307).
func claimStore(t testing.TB, dir string) {
	t.Helper()
	cd := claimsDir(dir)
	if err := os.MkdirAll(cd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cd, claimsKeepName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	byID, err := ticketPathsByID(dir)
	if err != nil {
		t.Fatal(err)
	}
	for id, paths := range byID {
		p := filepath.Join(cd, id)
		if _, err := os.Lstat(p); err == nil {
			continue
		}
		if err := os.WriteFile(p, []byte(ticketSlug(filepath.Base(paths[0]))+" 00000000\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func writeTestClaim(t *testing.T, dir, id, content string) {
	t.Helper()
	if err := os.MkdirAll(claimsDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claimsDir(dir), id), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClaimViolations(t *testing.T) {
	cases := []struct {
		name    string
		tickets []string
		claims  map[string]string // id -> content; nil map = no .claims/
		want    []string          // substrings, one per expected violation
	}{
		{"no .claims/: the store fails, naming the repair", []string{"0001-a.erg"}, nil,
			[]string{"has no .claims/: ID claims are required -- run: erg migrate --claims"}},
		{"claimed ticket passes", []string{"0001-a.erg"}, map[string]string{"0001": "a 0123abcd\n"}, nil},
		{"closed ticket claimed passes", []string{"closed/0001-a.erg"}, map[string]string{"0001": "a 0123abcd\n"}, nil},
		{"any second field accepted", []string{"0001-a.erg"}, map[string]string{"0001": "a 1762560000\n"}, nil},
		{"no trailing newline accepted", []string{"0001-a.erg"}, map[string]string{"0001": "a 0123abcd"}, nil},
		{"claim without ticket passes", nil, map[string]string{"0007": "gone 0123abcd\n"}, nil},
		{"dot names are not claims", []string{"0001-a.erg"}, map[string]string{"0001": "a x\n", ".keep": "", ".0002.tmp-1": "junk\njunk\n"}, nil},
		{"ticket without claim", []string{"0001-a.erg", "0002-b.erg"}, map[string]string{"0001": "a x\n"},
			[]string{`0002-b.erg: ticket has no claim`}},
		{"multi-line claim (merge=union)", []string{"0001-a.erg"}, map[string]string{"0001": "a x\nb y\n"},
			[]string{"claim must be exactly one line"}},
		{"empty claim", []string{"0001-a.erg"}, map[string]string{"0001": ""},
			[]string{"claim must be exactly one line"}},
		{"slug without nonce", []string{"0001-a.erg"}, map[string]string{"0001": "a\n"},
			[]string{"claim must be exactly one line"}},
		{"slug mismatch", []string{"0001-a.erg"}, map[string]string{"0001": "b x\n"},
			[]string{`0001-a.erg: claim`, `names slug "b", not "a"`}},
		// Reported once, by the duplicate-ID rule with its renumber hint
		// (TestCheckDuplicateHintSaysRenumber), not as a claim violation.
		{"two tickets under one claim (-X ours): left to the duplicate rule", []string{"0001-a.erg", "0001-b.erg"}, map[string]string{"0001": "a x\n"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, n := range c.tickets {
				writeClaimTestTicket(t, dir, n)
			}
			for id, content := range c.claims {
				writeTestClaim(t, dir, id, content)
			}
			tickets, _ := loadErgs(dir)
			got := claimViolations(dir, tickets)
			joined := strings.Join(got, "\n")
			if len(c.want) == 0 && len(got) != 0 {
				t.Fatalf("want no violation, got:\n%s", joined)
			}
			for _, w := range c.want {
				if !strings.Contains(joined, w) {
					t.Errorf("want a violation containing %q, got:\n%s", w, joined)
				}
			}
		})
	}
}

func TestClaimViolationPrintsPlainFileFix(t *testing.T) {
	dir := t.TempDir()
	writeClaimTestTicket(t, dir, "0003-hand-made.erg")
	if err := os.Mkdir(claimsDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	tickets, _ := loadErgs(dir)
	got := strings.Join(claimViolations(dir, tickets), "\n")
	want := `echo "hand-made $(date +%s)" > ` + filepath.Join(dir, ".claims", "0003")
	if !strings.Contains(got, want) {
		t.Fatalf("missing plain-file fix %q in:\n%s", want, got)
	}
}

func TestClaimFileSymlinkRejected(t *testing.T) {
	dir := t.TempDir()
	writeClaimTestTicket(t, dir, "0001-a.erg")
	outside := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.WriteFile(outside, []byte("a x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(claimsDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(claimsDir(dir), "0001")); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	tickets, _ := loadErgs(dir)
	got := strings.Join(claimViolations(dir, tickets), "\n")
	if !strings.Contains(got, "claim is a symlink") || strings.Contains(got, "has no claim") {
		t.Fatalf("want one symlink violation, got:\n%s", got)
	}
}

func TestClaimsSymlinkRefused(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, claimsDir(dir)); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	if err := requireClaims(dir); err == nil {
		t.Fatal("requireClaims followed a symlinked .claims")
	}
	if code := cmdNew([]string{"x", dir, "--author", "a"}); code == 0 {
		t.Fatal("erg new wrote through a symlinked .claims")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("a write escaped the store: %v", entries)
	}
}

func TestNextIDCountsClaims(t *testing.T) {
	dir := t.TempDir()
	writeClaimTestTicket(t, dir, "0001-a.erg")
	writeClaimTestTicket(t, dir, "0003-c.erg")
	// 0002 is a hole; 0005 was claimed and its ticket removed.
	writeTestClaim(t, dir, "0005", "removed x\n")
	writeTestClaim(t, dir, ".keep", "")
	got, err := nextID(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0006" {
		t.Fatalf("nextID = %s, want 0006 (above the highest claim, hole 0002 not reused)", got)
	}
}

func TestIdOfStorePath(t *testing.T) {
	cases := []struct {
		parent, name string
		want         int
	}{
		{".claims", "0042", 42},
		{".claims", ".keep", 0},
		{".claims", "42", 0},
		{".claims", "0000", 0},
		{".claims", "0042-x.erg", 0},
		{"tickets", "0042", 0},
		{"tickets", "0042-x.erg", 42},
	}
	for _, c := range cases {
		if got := idOfStorePath(c.parent, c.name); got != c.want {
			t.Errorf("idOfStorePath(%q, %q) = %d, want %d", c.parent, c.name, got, c.want)
		}
	}
}

// TestCmdNewClaimRace races two creations of the same number in one checkout:
// both compute 0004 (the seam), the claim's exclusive create lets exactly one
// take it, and the other moves on to the next number.
func TestCmdNewClaimRace(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(claimsDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	forced := map[string]bool{}
	orig := allocateID
	defer func() { allocateID = orig }()
	var caller string
	allocateID = func(d string) (string, error) {
		if !forced[caller] {
			forced[caller] = true
			return "0004", nil
		}
		return orig(d)
	}
	caller = "first"
	if code := cmdNew([]string{"first", dir, "--author", "a"}); code != 0 {
		t.Fatalf("first cmdNew = %d", code)
	}
	caller = "second"
	if code := cmdNew([]string{"second", dir, "--author", "a"}); code != 0 {
		t.Fatalf("second cmdNew = %d", code)
	}
	for _, n := range []string{"0004-first.erg", "0005-second.erg"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Errorf("%s missing: %v", n, err)
		}
	}
	if m, _ := filepath.Glob(filepath.Join(dir, "0004-*.erg")); len(m) != 1 {
		t.Errorf("number 0004 held by %d tickets, want 1: %v", len(m), m)
	}
	for id, slug := range map[string]string{"0004": "first", "0005": "second"} {
		data, err := os.ReadFile(filepath.Join(claimsDir(dir), id))
		if err != nil || !strings.HasPrefix(string(data), slug+" ") {
			t.Errorf("claim %s = %q (%v), want slug %s", id, data, err, slug)
		}
	}
	// No temp left behind by the losing link.
	entries, _ := os.ReadDir(claimsDir(dir))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			t.Errorf("leftover temp %s", e.Name())
		}
	}
}

func TestMigrateClaims(t *testing.T) {
	dir := t.TempDir()
	writeClaimTestTicket(t, dir, "0001-a.erg")
	writeClaimTestTicket(t, dir, "closed/0002-b.erg")
	written, kept, shared, err := migrateClaims(dir, mustTicketPaths(t, dir))
	if err != nil || written != 2 || kept != 0 || len(shared) != 0 {
		t.Fatalf("first run: written=%d kept=%d shared=%v err=%v", written, kept, shared, err)
	}
	snapshot := func() map[string]string {
		m := map[string]string{}
		entries, _ := os.ReadDir(claimsDir(dir))
		for _, e := range entries {
			b, _ := os.ReadFile(filepath.Join(claimsDir(dir), e.Name()))
			m[e.Name()] = string(b)
		}
		return m
	}
	before := snapshot()
	if _, ok := before[".keep"]; !ok {
		t.Error(".keep not written")
	}
	for id, slug := range map[string]string{"0001": "a", "0002": "b"} {
		if !strings.HasPrefix(before[id], slug+" ") {
			t.Errorf("claim %s = %q, want slug %s", id, before[id], slug)
		}
	}
	tickets, _ := loadErgs(dir)
	if v := claimViolations(dir, tickets); len(v) != 0 {
		t.Errorf("migrated store fails the claim rules: %v", v)
	}
	written, kept, _, err = migrateClaims(dir, mustTicketPaths(t, dir))
	if err != nil || written != 0 || kept != 2 {
		t.Fatalf("second run: written=%d kept=%d err=%v", written, kept, err)
	}
	after := snapshot()
	if len(after) != len(before) {
		t.Fatalf("second run changed the file set: %v -> %v", before, after)
	}
	for k, v := range before {
		if after[k] != v {
			t.Errorf("second run rewrote %s: %q -> %q", k, v, after[k])
		}
	}
}

// TestMigrationNonce pins the derived nonce to a literal computed outside
// the function (printf '0001 a 2026-11-08' | sha256sum | cut -c1-8): two
// clones that migrate the same ticket must write the same claim, or their
// independent migrations add/add-conflict on every claim.
func TestMigrationNonce(t *testing.T) {
	if got := migrationNonce("0001", "a", "2026-11-08"); got != "b298394c" {
		t.Errorf("migrationNonce = %q, want b298394c", got)
	}
	dir := t.TempDir()
	writeClaimTestTicket(t, dir, "0001-a.erg")
	if _, _, _, err := migrateClaims(dir, mustTicketPaths(t, dir)); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(claimsDir(dir), "0001"))
	if string(data) != "a b298394c\n" {
		t.Errorf("migrated claim = %q, want the derived nonce", data)
	}
}

func TestMigrateClaimsRefusesSharedNumber(t *testing.T) {
	dir := t.TempDir()
	writeClaimTestTicket(t, dir, "0001-a.erg")
	writeClaimTestTicket(t, dir, "0002-b.erg")
	writeClaimTestTicket(t, dir, "closed/0002-c.erg")
	written, _, blockers, err := migrateClaims(dir, mustTicketPaths(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if written != 0 || len(blockers) != 1 || !strings.Contains(blockers[0], "0002 is held by 2 tickets") {
		t.Fatalf("written=%d blockers=%v, want nothing written and the shared pair as the one blocker", written, blockers)
	}
	// The precheck writes nothing at all: not even 0001's claim or .claims/.
	if _, err := os.Stat(claimsDir(dir)); !os.IsNotExist(err) {
		t.Errorf("a blocked migrate created .claims/ (err=%v)", err)
	}
	if code := cmdMigrate([]string{"--claims", dir}); code == 0 {
		t.Error("erg migrate --claims exited 0 on a shared number")
	}
}

func TestMigrateWithoutClaimsFlagLeavesClaimsOff(t *testing.T) {
	dir := t.TempDir()
	writeClaimTestTicket(t, dir, "0001-a.erg")
	if code := cmdMigrate([]string{dir}); code != 0 {
		t.Fatalf("erg migrate = %d", code)
	}
	if _, err := os.Stat(claimsDir(dir)); !os.IsNotExist(err) {
		t.Errorf("bare erg migrate switched claims on (err=%v)", err)
	}
}

// TestWriteClaimWithoutHardLinks simulates a filesystem that refuses hard
// links: the claim is still written, and EEXIST is still the signal.
func TestWriteClaimWithoutHardLinks(t *testing.T) {
	orig := linkClaim
	defer func() { linkClaim = orig }()
	linkClaim = func(_, _ string) error { return &os.LinkError{Op: "link", Err: os.ErrPermission} }
	dir := t.TempDir()
	if err := os.Mkdir(claimsDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeClaim(dir, "0001", "a", "0123abcd"); err != nil {
		t.Fatalf("writeClaim without hard links: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(claimsDir(dir), "0001"))
	if !strings.HasPrefix(string(data), "a ") {
		t.Errorf("claim = %q", data)
	}
	if err := writeClaim(dir, "0001", "b", "0123abcd"); !os.IsExist(err) {
		t.Errorf("second claim of 0001: err = %v, want EEXIST", err)
	}
}

func mustTicketPaths(t *testing.T, dir string) map[string][]string {
	t.Helper()
	byID, err := ticketPathsByID(dir)
	if err != nil {
		t.Fatal(err)
	}
	return byID
}
