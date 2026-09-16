package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stampedStore lays down a ticket store whose .erg-assets stamps BOTH managed
// assets at caller-chosen hashes, with caller-chosen bytes on disk. Every arm
// of the enforcement below is one (stamp, disk) pair, so the fixture takes the
// two independently rather than deriving one from the other -- deriving them is
// exactly the coupling that would make the noisy and silent arms untellable.
func stampedStore(t *testing.T, ergrcDisk, agentsDisk, ergrcStamp, agentsStamp string) string {
	t.Helper()
	return stampedStoreDated(t, "", ergrcDisk, agentsDisk, ergrcStamp, agentsStamp)
}

// stampedStoreDated is stampedStore with the manifest's date: field under the
// caller's control, which is what the rollback arm needs: "which side is newer"
// is read from that field against the running binary's buildDate. An empty date
// writes no field at all, which is the shape every other arm wants -- no date
// means no direction, and no arm but the rollback one is about direction.
func stampedStoreDated(t *testing.T, date, ergrcDisk, agentsDisk, ergrcStamp, agentsStamp string) string {
	t.Helper()
	root := t.TempDir()
	ticketsDir := filepath.Join(root, "tickets")
	if err := os.MkdirAll(ticketsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ticketsDir, ".ergrc"), []byte(ergrcDisk), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ticketsDir, "AGENTS.md"), []byte(agentsDisk), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := "# erg provenance manifest -- do not edit\nrev: fixture\n"
	if date != "" {
		manifest += "date: " + date + "\n"
	}
	manifest += "assets:\n" +
		"  .ergrc sha256:" + ergrcStamp + "\n" +
		"  AGENTS.md sha256:" + agentsStamp + "\n"
	if err := os.WriteFile(filepath.Join(ticketsDir, manifestName), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	// Fixture guard: without a parsing manifest every arm below reroutes into
	// the stampless branch and stops being about enforcement at all.
	stamps := readManifestFile(filepath.Join(ticketsDir, manifestName))
	if stamps == nil {
		t.Fatal("fixture guard: the manifest does not parse, so the stamped branch is never reached")
	}
	if stamps["AGENTS.md"] != agentsStamp {
		t.Fatalf("fixture guard: AGENTS.md stamp is %q, want %q", stamps["AGENTS.md"], agentsStamp)
	}
	return ticketsDir
}

// TestAssetLocalEditViolations is ticket 0289's enforcement: a local edit to the
// shipped tickets/AGENTS.md is a hard error on `erg check`, and the message
// names where the content should go instead (ticket 0288 built that
// destination). The controls matter as much as the failing arm, and each is a
// state a REAL adopter is in today -- the 2026-09-16 re-survey found all three
// known adopters in the "behind the binary" state below, and zero in the
// violating one.
func TestAssetLocalEditViolations(t *testing.T) {
	ergrc, ok := bootstrapAsset("tickets/.ergrc")
	if !ok {
		t.Fatal("embedded .ergrc missing")
	}
	agents, ok := bootstrapAsset("tickets/AGENTS.md")
	if !ok {
		t.Fatal("embedded AGENTS.md missing")
	}
	ergrcHash := sha256hex([]byte(ergrc))
	agentsHash := sha256hex([]byte(agents))

	t.Run("noisy arm: AGENTS.md edited away from its own stamp is a violation", func(t *testing.T) {
		edited := agents + "\n## Our local lore\n\nCI job is called 'verify'.\n"
		dir := stampedStore(t, ergrc, edited, ergrcHash, agentsHash)

		got := strings.Join(assetLocalEditViolations(dir), "\n")
		if got == "" {
			t.Fatalf("a locally-edited AGENTS.md must be a violation, got none")
		}
		if !strings.Contains(got, "AGENTS.md") {
			t.Errorf("the violation must name the asset it is about: %q", got)
		}
		if !strings.Contains(got, "erg integration") {
			t.Errorf("the violation must name the destination built by 0288: %q", got)
		}
		if !strings.Contains(got, "LOCAL.md") {
			t.Errorf("the violation must name where the local content goes: %q", got)
		}
	})

	t.Run("silent control: AGENTS.md matches a stamp that is BEHIND the binary", func(t *testing.T) {
		// This is the arm that protects every real adopter. On 2026-09-16 all
		// three known stores were exactly here: file pristine against their own
		// stamp, stamp older than what git-erg main now ships. Keying
		// enforcement on the EMBEDDED hash instead of the stamp turns all three
		// red, so this control is the regression fixture for that failure mode.
		behind := "# AGENTS.md as shipped by an EARLIER erg release\n"
		if behind == agents {
			t.Fatal("test fixture collides with embedded content")
		}
		behindHash := sha256hex([]byte(behind))
		if behindHash == agentsHash {
			t.Fatal("fixture guard: the 'behind' stamp must differ from the embedded hash")
		}
		dir := stampedStore(t, ergrc, behind, ergrcHash, behindHash)

		if v := assetLocalEditViolations(dir); len(v) != 0 {
			t.Fatalf("a file matching its own stamp is not a local edit, got: %v", v)
		}
		// And the designed nudge must survive as a WARNING, not be swallowed or
		// promoted: the stamp is behind the embedded version, so `erg check`
		// still says a re-init would refresh it.
		got := strings.Join(managedAssetWarnings(dir), "\n")
		if !strings.Contains(got, assetDriftSignal) {
			t.Errorf("the behind-the-binary nudge must remain a drift WARNING: %q", got)
		}
	})

	t.Run("silent control: a STAMPLESS store reports stamplessness, not this error", func(t *testing.T) {
		// The two causes of an unrecognized hash must stay distinguishable
		// (ticket 0283). A store that never recorded provenance is told it
		// cannot be attributed -- it is NOT accused of editing.
		diverged := "# AGENTS.md in a store that never ran a stamping erg\n"
		if diverged == agents {
			t.Fatal("test fixture collides with embedded content")
		}
		root := t.TempDir()
		ticketsDir := filepath.Join(root, "tickets")
		if err := os.MkdirAll(ticketsDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ticketsDir, "AGENTS.md"), []byte(diverged), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ticketsDir, ".ergrc"), []byte(ergrc), 0644); err != nil {
			t.Fatal(err)
		}

		if v := assetLocalEditViolations(ticketsDir); len(v) != 0 {
			t.Fatalf("a stampless store has no stamp to be edited away from, got: %v", v)
		}
		got := strings.Join(assetDriftWarnings(ticketsDir), "\n")
		if !strings.Contains(got, assetStamplessSignal) {
			t.Errorf("the stampless report must still fire: %q", got)
		}
		if strings.Contains(got, assetLocalEditSignal) {
			t.Errorf("stamplessness must not be reported as a confirmed local edit: %q", got)
		}
	})

	t.Run("silent control: a customised .ergrc is NOT a violation", func(t *testing.T) {
		// .ergrc is the documented, encouraged customisation -- AGENTS.md
		// itself sends readers there to define Label: values. Enforcing over
		// initAssetPaths rather than the enforced subset would make every
		// adopter who took that advice fail `erg check`.
		customised := ergrc + "\nlabels = needs-human, deferred, blocked-upstream\n"
		dir := stampedStore(t, customised, agents, ergrcHash, agentsHash)

		if v := assetLocalEditViolations(dir); len(v) != 0 {
			t.Fatalf("a customised .ergrc is supported, not a violation, got: %v", v)
		}
	})

	t.Run("silent control: AGENTS.md exactly as stamped and as shipped", func(t *testing.T) {
		dir := stampedStore(t, ergrc, agents, ergrcHash, agentsHash)
		if v := assetLocalEditViolations(dir); len(v) != 0 {
			t.Fatalf("a pristine store must be silent, got: %v", v)
		}
	})

	t.Run("silent control: a ROLLBACK store is not a violator", func(t *testing.T) {
		// The stamp is NEWER than the running binary and the file matches it
		// exactly: the adopter is running an erg that predates their last init.
		// That is a legitimate state with its own warning (ticket 0279), and
		// enforcement must read the stamp's HASH without letting the stamp's
		// DATE make a pristine file a violation. Without this arm, feeding the
		// real dates into isCleanUpgrade -- which is the natural-looking call
		// -- passes every other arm here and fails this whole population.
		setBuildDate(t, "2026-01-01T00:00:00Z")
		ahead := "# AGENTS.md from an erg NEWER than this binary\n"
		if ahead == agents {
			t.Fatal("test fixture collides with embedded content")
		}
		dir := stampedStoreDated(t, "2099-01-01T00:00:00Z", ergrc, ahead, ergrcHash, sha256hex([]byte(ahead)))

		if v := assetLocalEditViolations(dir); len(v) != 0 {
			t.Fatalf("a rollback store's pristine file is not a local edit, got: %v", v)
		}
		// Fixture guard: this really is the rollback direction, not a fixture
		// that silently degraded into an ordinary stamped store.
		got := strings.Join(managedAssetWarnings(dir), "\n")
		if !strings.Contains(got, assetRollbackSignal) {
			t.Fatalf("fixture guard: the rollback direction is not being reached: %q", got)
		}
	})

	t.Run("silent control: an unusable AGENTS.md stamp is left to the stampless path", func(t *testing.T) {
		// A manifest line that parses but does not hold a hash carries no
		// evidence either way (ticket 0292's looksLikeAssetHash lesson). The
		// enforcement must decline it rather than read "not equal to the stamp"
		// off a stamp that is not one.
		edited := "# AGENTS.md, locally rewritten\n"
		dir := stampedStore(t, ergrc, edited, ergrcHash, "not-a-hash")
		if v := assetLocalEditViolations(dir); len(v) != 0 {
			t.Fatalf("an unusable stamp is no evidence of an edit, got: %v", v)
		}
		got := strings.Join(managedAssetWarnings(dir), "\n")
		if !strings.Contains(got, assetStamplessSignal) {
			t.Errorf("the unusable stamp must route to the stampless report: %q", got)
		}
	})
}

// TestCheckFailsOnLocallyEditedAgentsMd is the end-to-end half: the violation
// must reach `erg check`'s EXIT CODE, not merely its warning stream. A unit
// test on assetLocalEditViolations alone would pass over an implementation that
// computed the violation and appended it to corpusWarnings, which is exactly the
// state ticket 0289 exists to leave behind.
func TestCheckFailsOnLocallyEditedAgentsMd(t *testing.T) {
	agents, ok := bootstrapAsset("tickets/AGENTS.md")
	if !ok {
		t.Fatal("embedded AGENTS.md missing")
	}
	ergrc, ok := bootstrapAsset("tickets/.ergrc")
	if !ok {
		t.Fatal("embedded .ergrc missing")
	}

	withTicket := func(t *testing.T, dir string) {
		t.Helper()
		// Gives the store a ticket so the arms below exercise the ordinary
		// populated-corpus path. Neither corpusWarnings nor cmdCheck needs it
		// any more -- both report the dir-based scans on an empty corpus too --
		// but that case has its own arms, and mixing the two here would hide
		// which path an assertion fired on.
		ticket := "%erg 0.1\nTitle: A ticket so the corpus is not empty\n" +
			"Created: 2026-09-16\nAuthor: fixture\n\n--- log ---\n" +
			"2026-09-16T10:00Z fixture created\n\n--- body ---\n\nNothing.\n"
		if err := os.WriteFile(filepath.Join(dir, "0001-fixture.erg"), []byte(ticket), 0644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("noisy arm: exit 1 on a locally-edited AGENTS.md", func(t *testing.T) {
		dir := stampedStore(t, ergrc, agents+"\nlocal edit\n", sha256hex([]byte(ergrc)), sha256hex([]byte(agents)))
		withTicket(t, dir)
		if code := cmdCheck([]string{dir}); code != 1 {
			t.Fatalf("erg check must FAIL on a locally-edited AGENTS.md, got exit %d", code)
		}
	})

	t.Run("noisy arm: exit 1 with an edited AGENTS.md and NO tickets at all", func(t *testing.T) {
		// The population most likely to trip this rule: a fresh adopter who ran
		// erg init, is reading AGENTS.md for the first time, and has not filed a
		// ticket yet. cmdCheck's empty-corpus early return used to hand them a
		// silent exit 0, and the withTicket helper above stepped around the hole
		// rather than exposing it -- which made the gap invisible and is the
		// worse half of the defect. No withTicket call here, deliberately.
		dir := stampedStore(t, ergrc, agents+"\nlocal edit\n", sha256hex([]byte(ergrc)), sha256hex([]byte(agents)))
		if code := cmdCheck([]string{dir}); code != 1 {
			t.Fatalf("an edited AGENTS.md must fail erg check even with an empty corpus, got exit %d", code)
		}
	})

	t.Run("silent control: an empty corpus with a PRISTINE store still exits 0", func(t *testing.T) {
		// The other half of the fix: a store that is genuinely fine keeps its
		// "No .erg files found." exit 0. Without this arm, an implementation
		// that simply deletes the early return passes the case above and
		// changes the exit semantics for every empty store.
		dir := stampedStore(t, ergrc, agents, sha256hex([]byte(ergrc)), sha256hex([]byte(agents)))
		if code := cmdCheck([]string{dir}); code != 0 {
			t.Fatalf("an empty but pristine store must still exit 0, got exit %d", code)
		}
	})

	t.Run("silent control: exit 0 on the same store, pristine", func(t *testing.T) {
		// Same fixture shape, same ticket, different bytes in one file. An
		// implementation that fails whenever a manifest is present passes the
		// arm above and dies here.
		dir := stampedStore(t, ergrc, agents, sha256hex([]byte(ergrc)), sha256hex([]byte(agents)))
		withTicket(t, dir)
		if code := cmdCheck([]string{dir}); code != 0 {
			t.Fatalf("a pristine store must pass erg check, got exit %d", code)
		}
	})
}
