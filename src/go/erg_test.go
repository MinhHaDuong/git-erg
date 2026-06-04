package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestParseHeaderLine(t *testing.T) {
	cases := []struct {
		input   string
		wantKey string
		wantVal string
		wantOK  bool
	}{
		{"Title: foo", "Title", "foo", true},
		{"Blocked-by: 0042", "Blocked-by", "0042", true},
		{"Key_with_under: v", "Key_with_under", "v", true},
		{"Key  : v", "Key", "v", true}, // spaces before colon are allowed
		{"no-colon", "", "", false},
		{" leading-space: v", "", "", false},
		{"1starts-digit: v", "", "", false},
		{"", "", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			gotKey, gotVal, gotOK := parseHeaderLine(tc.input)
			if gotOK != tc.wantOK || gotKey != tc.wantKey || gotVal != tc.wantVal {
				t.Errorf("parseHeaderLine(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.input, gotKey, gotVal, gotOK,
					tc.wantKey, tc.wantVal, tc.wantOK)
			}
		})
	}
}

// ergWithTitle returns a minimal valid ticket with the given title.
// Uses the same base as validErgContent but with a parameterised title.
func ergWithTitle(title string) string {
	return "%erg 0.1\nTitle: " + title + "\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
}

func TestParseErg(t *testing.T) {
	t.Run("minimal valid ticket", func(t *testing.T) {
		path := writeErg(t, t.TempDir(), "0001-test.erg", ergWithTitle("My Title"))
		erg, parseErrs := parseErg(path)
		if errsContain(parseErrs, "missing magic first line") {
			t.Errorf("unexpected magic error for valid ticket: %v", parseErrs)
		}
		if erg.Title != "My Title" {
			t.Errorf("Title = %q, want %q", erg.Title, "My Title")
		}
		if errsContain(parseErrs, "missing '--- log ---'") {
			t.Errorf("expected no missing-log-separator error, got: %v", parseErrs)
		}
		if errsContain(parseErrs, "missing '--- body ---'") {
			t.Errorf("expected no missing-body-separator error, got: %v", parseErrs)
		}
	})

	t.Run("missing magic line", func(t *testing.T) {
		content := "Title: foo\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		_, parseErrs := parseErg(path)
		if !errsContain(parseErrs, "missing magic first line") {
			t.Errorf("expected magic error for missing magic line, got: %v", parseErrs)
		}
	})

	t.Run("CRLF line endings", func(t *testing.T) {
		content := strings.ReplaceAll(ergWithTitle("CRLF Title"), "\n", "\r\n")
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg, parseErrs := parseErg(path)
		if errsContain(parseErrs, "missing magic first line") {
			t.Errorf("unexpected magic error for CRLF content: %v", parseErrs)
		}
		if erg.Title != "CRLF Title" {
			t.Errorf("Title = %q, want %q", erg.Title, "CRLF Title")
		}
	})

	t.Run("no trailing newline", func(t *testing.T) {
		content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n--- log ---\n--- body ---"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		_, parseErrs := parseErg(path)
		if errsContain(parseErrs, "missing magic first line") {
			t.Errorf("unexpected magic error for no-trailing-newline content: %v", parseErrs)
		}
		if errsContain(parseErrs, "missing '--- log ---'") {
			t.Errorf("expected no missing-log-separator error (--- log --- on penultimate line), got: %v", parseErrs)
		}
		if errsContain(parseErrs, "missing '--- body ---'") {
			t.Errorf("expected no missing-body-separator error (--- body --- as final line, no trailing newline), got: %v", parseErrs)
		}
	})

	t.Run("repeated Blocked-by header", func(t *testing.T) {
		content := "%erg 0.1\nTitle: A\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: 0002\nBlocked-by: 0003\n\n--- log ---\n--- body ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg, _ := parseErg(path)
		bb := erg.BlockedBys
		if len(bb) != 2 {
			t.Fatalf("BlockedBys len=%d, want 2", len(bb))
		}
		if bb[0].ID != "0002" {
			t.Errorf("BlockedBys[0].ID = %q, want %q", bb[0].ID, "0002")
		}
		if bb[1].ID != "0003" {
			t.Errorf("BlockedBys[1].ID = %q, want %q", bb[1].ID, "0003")
		}
	})

	t.Run("magic line padded with whitespace", func(t *testing.T) {
		// parseErg uses TrimSpace before comparing to MagicLine, so leading/trailing
		// spaces on the first line must still be accepted as a valid magic marker.
		content := "  %erg 0.1  \nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		_, parseErrs := parseErg(path)
		if errsContain(parseErrs, "missing magic first line") {
			t.Errorf("unexpected magic error for whitespace-padded magic line: %v", parseErrs)
		}
	})

	t.Run("separator inside body section", func(t *testing.T) {
		// A second '--- log ---' inside the body must not change the
		// section or cause a panic. The literal becomes body text and
		// the parser counts the separator as present (set on ANY sighting,
		// so no missing-separator error fires).
		content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n--- log ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg, parseErrs := parseErg(path)
		if errsContain(parseErrs, "missing magic first line") {
			t.Errorf("unexpected magic error: %v", parseErrs)
		}
		if errsContain(parseErrs, "missing '--- log ---'") {
			t.Errorf("expected no missing-log-separator error, got: %v", parseErrs)
		}
		if !strings.Contains(erg.Body, "--- log ---") {
			t.Errorf("erg.Body = %q, expected to contain the quoted '--- log ---' literal", erg.Body)
		}
	})

	t.Run("duplicate Title keeps first value", func(t *testing.T) {
		// Item 4: singleton "first occurrence wins" -- Title is a singleton
		// header; when duplicated, the parser keeps the first value.
		content := "%erg 0.1\nTitle: First\nTitle: Second\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg, _ := parseErg(path)
		if erg.Title != "First" {
			t.Errorf("Title = %q, want %q (first occurrence wins)", erg.Title, "First")
		}
	})

	t.Run("empty Label value skipped", func(t *testing.T) {
		// Item 5: empty Label: values are silently skipped by the parser --
		// parseHeaderLine trims, and the parser skips empty val for Label.
		content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nLabel: \n\n--- log ---\n--- body ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg, _ := parseErg(path)
		if len(erg.Labels) != 0 {
			t.Errorf("Labels = %v (len=%d), want empty slice (empty Label: value skipped)", erg.Labels, len(erg.Labels))
		}
	})

	t.Run("CRLF body lines retain no carriage return", func(t *testing.T) {
		content := "%erg 0.1\r\nTitle: X\r\nCreated: 2024-01-01\r\nAuthor: test\r\n\r\n--- log ---\r\n--- body ---\r\nhello world\r\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg, _ := parseErg(path)
		if strings.Contains(erg.Body, "\r") {
			t.Errorf("Body contains \\r: %q", erg.Body)
		}
	})

	t.Run("CRLF log lines retain no carriage return", func(t *testing.T) {
		content := "%erg 0.1\r\nTitle: X\r\nCreated: 2024-01-01\r\nAuthor: test\r\n\r\n--- log ---\r\n2024-01-01T10:00Z author note\r\n--- body ---\r\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		erg, _ := parseErg(path)
		for i, l := range erg.LogLines {
			if strings.Contains(l, "\r") {
				t.Errorf("LogLines[%d] contains \\r: %q", i, l)
			}
		}
	})

	t.Run("UTF-8 BOM stripped", func(t *testing.T) {
		bom := "\xEF\xBB\xBF"
		content := bom + "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		_, parseErrs := parseErg(path)
		for _, e := range parseErrs {
			if strings.Contains(e, "missing magic first line") {
				t.Errorf("BOM caused magic line mismatch: %v", parseErrs)
			}
		}
	})

	t.Run("file does not exist", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nonexistent.erg")
		erg, _ := parseErg(path)
		if erg.Path != path {
			t.Errorf("Path = %q, want %q", erg.Path, path)
		}
	})
}

// TestMagicLineDowngrade verifies that a file with the legacy "%erg v1"
// magic line is rejected with a migrate hint, not a generic "missing
// magic" error, and that "%erg v2" (unknown) still gets the generic error.
func TestMagicLineDowngrade(t *testing.T) {
	t.Run("legacy v1 emits migrate hint", func(t *testing.T) {
		content := "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		_, errs := parseErg(path)
		if !errsContain(errs, "erg migrate") {
			t.Errorf("expected 'erg migrate' hint for legacy v1, got: %v", errs)
		}
		if errsContain(errs, "missing magic first line") {
			t.Errorf("legacy v1 should NOT emit generic missing-magic error, got: %v", errs)
		}
	})

	t.Run("unknown v2 emits generic missing-magic error", func(t *testing.T) {
		content := "%erg v2\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		_, errs := parseErg(path)
		if !errsContain(errs, "missing magic first line") {
			t.Errorf("expected generic missing-magic error for v2, got: %v", errs)
		}
		if errsContain(errs, "erg migrate") {
			t.Errorf("v2 should NOT emit migrate hint, got: %v", errs)
		}
	})

	t.Run("current 0.1 accepted", func(t *testing.T) {
		content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		_, errs := parseErg(path)
		if errsContain(errs, "magic") || errsContain(errs, "migrate") {
			t.Errorf("expected no magic/migrate errors for current format, got: %v", errs)
		}
	})
}

// TestClosedWhitespaceDivergence pins the divergence between
// parseHeaderLine and isClosedHeaderLine (item 7).
// parseHeaderLine accepts `Closed : val` (space before colon) as a valid
// header, but isClosedHeaderLine requires the exact prefix `Closed:` (no
// space). When `Closed : merged` appears in the body section, the parser
// does not emit the "found in body section" error.
// This test pins that current behavior.
func TestClosedWhitespaceDivergence(t *testing.T) {
	t.Run("Closed: in body triggers body-section error", func(t *testing.T) {
		content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\nClosed: merged\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		_, parseErrs := parseErg(path)
		if !errsContain(parseErrs, "found in body section") {
			t.Errorf("expected 'found in body section' error for 'Closed: merged' in body, got: %v", parseErrs)
		}
	})

	t.Run("Closed_space_colon in body does NOT trigger body-section error", func(t *testing.T) {
		// `Closed : merged` -- parseHeaderLine would parse this as key=Closed,
		// but isClosedHeaderLine does not fire because it expects prefix `Closed:`.
		// This pins the current divergence; a future ticket may align them.
		content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\nClosed : merged\n"
		path := writeErg(t, t.TempDir(), "0001-test.erg", content)
		_, parseErrs := parseErg(path)
		if errsContain(parseErrs, "found in body section") {
			t.Errorf("expected no 'found in body section' error for 'Closed : merged' -- isClosedHeaderLine requires exact 'Closed:' prefix, got: %v", parseErrs)
		}
	})
}

// TestPathIsClosed exercises the path-component closure test from erg.go.
// Item 2: covers directory names, basename prefixes/suffixes, and
// case-insensitivity as specified in tickets/spec-erg-v1.md.
func TestPathIsClosed(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		// Empty path
		{"", false},

		// Directory component equals "closed"
		{"closed/0001-foo.erg", true},
		{"tickets/closed/0001-foo.erg", true},

		// Directory starts with "closed-"
		{"closed-2024/0001-foo.erg", true},

		// Directory starts with "closed."
		{"closed.old/0001-foo.erg", true},

		// Directory ends with "-closed"
		{"archive-closed/0001-foo.erg", true},

		// Basename (without extension) equals "closed"
		{"tickets/closed.erg", true},

		// Basename starts with "closed-"
		{"closed-foo.erg", true},

		// Basename ends with "-closed"
		{"0001-closed.erg", true},

		// Case insensitivity
		{"Closed/0001-foo.erg", true},
		{"CLOSED/0001-foo.erg", true},

		// Open paths -- no closure signal
		{"tickets/0001-foo.erg", false},
		{"open/0001-foo.erg", false},

		// "disclosed" must not trigger -- the implementation checks HasPrefix
		// on the full component, so "disclosed" does NOT match.
		{"disclosed/0001-foo.erg", false},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got := pathIsClosed(tc.path)
			if got != tc.want {
				t.Errorf("pathIsClosed(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// TestStaleBlockedBy exercises staleBlockedBy (check.go:33).
// Item 1: constructs two tickets -- one closed blocker, one open ticket
// referencing it -- and asserts the warning fires.
func TestStaleBlockedBy(t *testing.T) {
	t.Run("open ticket blocked by closed ticket emits warning", func(t *testing.T) {
		dir := t.TempDir()
		// 0001 is closed (has Closed: header)
		writeErg(t, dir, "0001-blocker.erg",
			"%erg 0.1\nTitle: Blocker\nCreated: 2024-01-01\nAuthor: test\nClosed: done\n\n--- log ---\n--- body ---\n")
		// 0002 is open and blocked by 0001
		writeErg(t, dir, "0002-feature.erg",
			"%erg 0.1\nTitle: Feature\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: 0001\n\n--- log ---\n--- body ---\n")

		tickets, _ := loadErgs(dir)
		warnings := staleBlockedBy(tickets)
		if len(warnings) == 0 {
			t.Fatal("expected a stale Blocked-by warning, got none")
		}
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "0001") && strings.Contains(w, "already closed") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected warning mentioning '0001' and 'already closed', got: %v", warnings)
		}
	})

	t.Run("open ticket blocked by open ticket emits no warning", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0001-blocker.erg",
			"%erg 0.1\nTitle: Blocker\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n")
		writeErg(t, dir, "0002-feature.erg",
			"%erg 0.1\nTitle: Feature\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: 0001\n\n--- log ---\n--- body ---\n")

		tickets, _ := loadErgs(dir)
		warnings := staleBlockedBy(tickets)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings, got: %v", warnings)
		}
	})

	t.Run("closed ticket blocked by closed ticket emits no warning", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0001-blocker.erg",
			"%erg 0.1\nTitle: Blocker\nCreated: 2024-01-01\nAuthor: test\nClosed: done\n\n--- log ---\n--- body ---\n")
		writeErg(t, dir, "0002-feature.erg",
			"%erg 0.1\nTitle: Feature\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: 0001\nClosed: done\n\n--- log ---\n--- body ---\n")

		tickets, _ := loadErgs(dir)
		warnings := staleBlockedBy(tickets)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings for closed-blocked-by-closed, got: %v", warnings)
		}
	})

	t.Run("forge ref does not trigger stale warning", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0001-feature.erg",
			"%erg 0.1\nTitle: Feature\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: github.com/foo/bar#1\n\n--- log ---\n--- body ---\n")

		tickets, _ := loadErgs(dir)
		warnings := staleBlockedBy(tickets)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings for forge ref, got: %v", warnings)
		}
	})
}

// TestOpenCarrierSupersededBy exercises openCarrierSupersededBy (check.go).
// An OPEN ticket carrying a Superseded-by header is a transition smell and
// must warn; a CLOSED carrier (the normal pattern) must not.
func TestOpenCarrierSupersededBy(t *testing.T) {
	t.Run("open carrier warns", func(t *testing.T) {
		dir := t.TempDir()
		// 0001 is the replacement (open); 0002 carries Superseded-by but is
		// itself still OPEN -- the smell case.
		writeErg(t, dir, "0001-replacement.erg",
			"%erg 0.1\nTitle: Replacement\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n")
		writeErg(t, dir, "0002-open-carrier.erg",
			"%erg 0.1\nTitle: Old\nCreated: 2024-01-01\nAuthor: test\nSuperseded-by: 0001\n\n--- log ---\n--- body ---\n")

		tickets, _ := loadErgs(dir)
		warnings := openCarrierSupersededBy(tickets)
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "0002") && strings.Contains(w, "open ticket carries Superseded-by") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected an open-carrier warning mentioning 0002, got: %v", warnings)
		}
	})

	t.Run("closed carrier does not warn", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0001-replacement.erg",
			"%erg 0.1\nTitle: Replacement\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n")
		// 0002 carries Superseded-by AND is closed -- the normal pattern.
		writeErg(t, dir, "0002-closed-carrier.erg",
			"%erg 0.1\nTitle: Old\nCreated: 2024-01-01\nAuthor: test\nClosed: superseded\nSuperseded-by: 0001\n\n--- log ---\n--- body ---\n")

		tickets, _ := loadErgs(dir)
		warnings := openCarrierSupersededBy(tickets)
		if len(warnings) != 0 {
			t.Errorf("expected no warning for a closed carrier, got: %v", warnings)
		}
	})
}

// TestDispatchRegistrySync walks the switch statement in main.go and
// compares its case labels against the commands registry in helptext.go.
// If someone adds a commandEntry but forgets the switch case (or vice
// versa), this test catches the drift.
func TestDispatchRegistrySync(t *testing.T) {
	// 1. Extract case labels from main.go's dispatch switch.
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing main.go: %v", err)
	}

	// nonCommandCases are case labels in the switch that are NOT registry
	// commands: the --help/-h/help fallback, and the github / erg-github
	// dispatch hint (erg-github is a separate committed helper script, not an
	// erg subcommand -- the case only prints a pointer to tickets/erg-github).
	nonCommandCases := map[string]bool{
		"-h": true, "--help": true, "help": true,
		"github": true, "erg-github": true,
	}

	var switchCmds []string
	ast.Inspect(f, func(n ast.Node) bool {
		sw, ok := n.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		// Only look at the switch whose tag is the ident "cmd".
		tag, ok := sw.Tag.(*ast.Ident)
		if !ok || tag.Name != "cmd" {
			return true
		}
		for _, stmt := range sw.Body.List {
			cc, ok := stmt.(*ast.CaseClause)
			if !ok || cc.List == nil { // skip default
				continue
			}
			for _, expr := range cc.List {
				lit, ok := expr.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				val := strings.Trim(lit.Value, `"`)
				if nonCommandCases[val] {
					continue
				}
				switchCmds = append(switchCmds, val)
			}
		}
		return false // only need the first matching switch
	})

	// 2. Extract command names from the registry.
	var registryCmds []string
	for _, c := range commands {
		registryCmds = append(registryCmds, c.Name)
	}

	sort.Strings(switchCmds)
	sort.Strings(registryCmds)

	// 3. Compare.
	if strings.Join(switchCmds, ",") != strings.Join(registryCmds, ",") {
		t.Errorf("dispatch switch and commands registry are out of sync\n  switch cases: %v\n  registry:     %v",
			switchCmds, registryCmds)
	}
}

// missingFromCMDS returns the mutating commands (classification[name]==true)
// that are NOT in the documented exclusions set and NOT present in cmdsSet.
// A non-empty result means the strict-write round-trip guard
// (tests/test_strictwrite.sh) would skip a command that mutates .erg files.
// Extracted as a helper so the meta-test's negative control can drive it with
// a doctored classification map (mirrors headerKeysWithoutFixture in
// validate_test.go).
func missingFromCMDS(classification, exclusions, cmdsSet map[string]bool) []string {
	var missing []string
	for name, mutating := range classification {
		if !mutating || exclusions[name] {
			continue
		}
		if !cmdsSet[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

// TestMutatingCommandsCMDSCoverage is a meta-test for the command axis of the
// strict-write guard, peer to TestDispatchRegistrySync (which guards the
// switch-vs-registry axis). TestV1HeaderKeys_FixtureCoverage (validate_test.go)
// closed the equivalent gap on the header-key axis; this closes it on the
// command axis. Every command in the dispatch registry (helptext.go's commands
// slice) must be explicitly classified as mutating or read-only here. The
// mutating set, minus the four documented exclusions, must exactly equal the
// CMDS list in tests/test_strictwrite.sh. If a new mutating subcommand enters
// the dispatch without joining CMDS, the round-trip guard silently shrinks --
// the exact drift this test exists to prevent.
func TestMutatingCommandsCMDSCoverage(t *testing.T) {
	// 1. Exhaustive in-test classification of all 19 registry commands.
	// true == mutates .erg files (writes a ticket). The totality loop below
	// fails loud if any registry command is missing from this map, so a new
	// subcommand cannot slip through unclassified.
	classification := map[string]bool{
		// mutating: write or rewrite .erg ticket files
		"new":     true,
		"close":   true,
		"log":     true,
		"label":   true,
		"unlabel": true,
		"archive": true,
		"migrate": true,
		// mutating but EXCLUDED from the strict-write round-trip. They mutate
		// state, so they are honestly classified mutating, but
		// test_strictwrite.sh's "Excluded by design" notes (near the CMDS=
		// line) deliberately omit them: rm deletes a file so there is nothing
		// to re-validate (covered by test_datasafety.sh Guard 6); init touches
		// non-.erg files (.ergrc); install/update touch the installed binary,
		// not tickets.
		"rm":      true,
		"init":    true,
		"install": true,
		"update":  true,
		// read-only: inspect or validate, never write
		"validate":    false,
		"check":       false,
		"list":        false,
		"ready":       false,
		"next-id":     false,
		"spec":        false,
		"integration": false,
		"version":     false,
	}

	// The four documented exclusions: mutating commands intentionally absent
	// from CMDS because the strict-write round-trip cannot drive them.
	exclusions := map[string]bool{
		"rm": true, "init": true, "install": true, "update": true,
	}

	// 2. Totality loop (anti-tautology core): every registry command must be
	// classified. An unclassified entry (e.g. a freshly added subcommand)
	// fails here rather than silently defaulting and escaping the comparison.
	for _, c := range commands {
		if _, ok := classification[c.Name]; !ok {
			t.Errorf("command %q is in the dispatch registry but not classified mutating/readonly in this test -- add it (and to CMDS in tests/test_strictwrite.sh if it mutates .erg files)", c.Name)
		}
	}

	// 3. Exclusions assertion (exit criterion 2): each of the four exclusions
	// must be present in the classification map AND marked mutating. A typo
	// (e.g. "instal") would otherwise silently narrow the mutating set without
	// notice; this records the intent explicitly and fails loud on drift.
	for name := range exclusions {
		mutating, ok := classification[name]
		if !ok {
			t.Errorf("exclusion %q is not present in the classification map -- exclusions must be classified mutating", name)
			continue
		}
		if !mutating {
			t.Errorf("exclusion %q is classified read-only; exclusions are mutating commands deliberately omitted from CMDS", name)
		}
	}

	// 4. Parse CMDS from the strict-write script. go test runs with cwd set to
	// the package dir (src/go), so the script is at ../../tests/.
	scriptPath := filepath.Join("..", "..", "tests", "test_strictwrite.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("reading %s: %v", scriptPath, err)
	}
	// (?m) is required: the assignment is at line ~136, not start-of-file, and
	// Go's RE2 anchors ^ to start-of-text without the multiline flag.
	re := regexp.MustCompile(`(?m)^CMDS="([^"]*)"`)
	m := re.FindSubmatch(data)
	if m == nil {
		t.Fatalf("no CMDS=\"...\" assignment found in %s -- regex mismatch or file moved", scriptPath)
	}
	cmdsSet := map[string]bool{}
	for _, name := range strings.Fields(string(m[1])) {
		cmdsSet[name] = true
	}
	// Non-empty guard: a broken regex or empty list must not pass vacuously.
	if len(cmdsSet) == 0 {
		t.Fatal("parsed CMDS set is empty -- regex mismatch or empty assignment, cannot validate")
	}

	// 5. Set equality, direction A: every mutating non-excluded command is in
	// CMDS (the guard covers all of them).
	if missing := missingFromCMDS(classification, exclusions, cmdsSet); len(missing) != 0 {
		t.Errorf("mutating commands missing from CMDS in %s: %v -- add them to the CMDS= line so the strict-write round-trip exercises them", scriptPath, missing)
	}

	// 6. Set equality, direction B: CMDS contains no phantom entry that is not
	// a mutating, non-excluded command (no undocumented or stale name).
	for name := range cmdsSet {
		mutating := classification[name]
		if !mutating || exclusions[name] {
			t.Errorf("CMDS in %s contains %q, which is not a non-excluded mutating command (mutating=%v, excluded=%v) -- remove it or fix the classification", scriptPath, name, mutating, exclusions[name])
		}
	}

	// 7. Negative control: inject a bogus mutating command and confirm the
	// helper reports it missing from CMDS. Guards against a helper that always
	// returns empty (a tautological pass). Mirrors the doctored-key control in
	// TestV1HeaderKeys_FixtureCoverage.
	doctored := map[string]bool{}
	for k, v := range classification {
		doctored[k] = v
	}
	doctored["frobnicate"] = true
	gaps := missingFromCMDS(doctored, exclusions, cmdsSet)
	found := false
	for _, g := range gaps {
		if g == "frobnicate" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("negative control: injected \"frobnicate\" not reported missing from CMDS (got %v) -- the helper has a tautology", gaps)
	}
}

// TestFolderClosure exercises folderClosure (check.go:12).
func TestFolderClosure(t *testing.T) {
	t.Run("open ticket in closed dir warns", func(t *testing.T) {
		dir := t.TempDir()
		closedDir := filepath.Join(dir, "closed")
		if err := os.MkdirAll(closedDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Open ticket (no Closed: header) placed inside closed/
		writeErg(t, closedDir, "0001-misplaced.erg",
			"%erg 0.1\nTitle: Misplaced\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n")
		tickets, _ := loadErgs(dir)
		warnings := folderClosure(tickets)
		if len(warnings) == 0 {
			t.Fatal("expected a warning for open ticket in closed/ directory, got none")
		}
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "open ticket in closed/ directory") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected warning about 'open ticket in closed/ directory', got: %v", warnings)
		}
	})

	t.Run("closed ticket not in closed dir warns", func(t *testing.T) {
		dir := t.TempDir()
		// Closed ticket (has Closed: header) placed in top-level, not in closed/
		writeErg(t, dir, "0001-stale.erg",
			"%erg 0.1\nTitle: Stale\nCreated: 2024-01-01\nAuthor: test\nClosed: done\n\n--- log ---\n--- body ---\n")
		tickets, _ := loadErgs(dir)
		warnings := folderClosure(tickets)
		if len(warnings) == 0 {
			t.Fatal("expected a warning for closed ticket not in closed/ directory, got none")
		}
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "closed ticket not in closed/ directory") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected warning about 'closed ticket not in closed/ directory', got: %v", warnings)
		}
	})

	t.Run("open ticket in open dir no warning", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0001-normal.erg",
			"%erg 0.1\nTitle: Normal\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n")
		tickets, _ := loadErgs(dir)
		warnings := folderClosure(tickets)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings, got: %v", warnings)
		}
	})

	t.Run("closed ticket in closed dir no warning", func(t *testing.T) {
		dir := t.TempDir()
		closedDir := filepath.Join(dir, "closed")
		if err := os.MkdirAll(closedDir, 0755); err != nil {
			t.Fatal(err)
		}
		writeErg(t, closedDir, "0001-archived.erg",
			"%erg 0.1\nTitle: Archived\nCreated: 2024-01-01\nAuthor: test\nClosed: done\n\n--- log ---\n--- body ---\n")
		tickets, _ := loadErgs(dir)
		warnings := folderClosure(tickets)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings, got: %v", warnings)
		}
	})

	t.Run("empty slice returns nil", func(t *testing.T) {
		warnings := folderClosure(nil)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings, got: %v", warnings)
		}
	})
}

// TestStrayGoSource exercises strayGoSource (check.go:68).
func TestStrayGoSource(t *testing.T) {
	t.Run("warns when go files in dir", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644); err != nil {
			t.Fatal(err)
		}
		warnings := strayGoSource(dir)
		if len(warnings) == 0 {
			t.Fatal("expected a warning for stray .go file, got none")
		}
		if !strings.Contains(warnings[0], "Go source files found") {
			t.Errorf("unexpected warning text: %s", warnings[0])
		}
	})

	t.Run("warns when go.mod in dir", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x"), 0644); err != nil {
			t.Fatal(err)
		}
		warnings := strayGoSource(dir)
		if len(warnings) == 0 {
			t.Fatal("expected a warning for stray go.mod, got none")
		}
	})

	t.Run("warns when go files in tools/go subdir", func(t *testing.T) {
		dir := t.TempDir()
		toolsGoDir := filepath.Join(dir, "tools", "go")
		if err := os.MkdirAll(toolsGoDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(toolsGoDir, "helper.go"), []byte("package main"), 0644); err != nil {
			t.Fatal(err)
		}
		warnings := strayGoSource(dir)
		if len(warnings) == 0 {
			t.Fatal("expected a warning for stray .go in tools/go/, got none")
		}
		if !strings.Contains(warnings[0], "tools/go") {
			t.Errorf("expected warning to mention tools/go path, got: %s", warnings[0])
		}
	})

	t.Run("no warning for clean dir", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "0001-test.erg"), []byte("ticket"), 0644); err != nil {
			t.Fatal(err)
		}
		warnings := strayGoSource(dir)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings, got: %v", warnings)
		}
	})

	t.Run("nonexistent dir returns nil", func(t *testing.T) {
		warnings := strayGoSource(filepath.Join(t.TempDir(), "nonexistent"))
		if len(warnings) != 0 {
			t.Errorf("expected no warnings, got: %v", warnings)
		}
	})
}

// TestEncodingWarnings exercises encodingWarnings (check.go).
func TestEncodingWarnings(t *testing.T) {
	t.Run("CRLF file warns", func(t *testing.T) {
		dir := t.TempDir()
		content := "%erg 0.1\r\nTitle: X\r\nCreated: 2024-01-01\r\nAuthor: test\r\n\r\n--- log ---\r\n--- body ---\r\n"
		writeErg(t, dir, "0001-test.erg", content)
		warnings := encodingWarnings(dir)
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "CRLF") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected CRLF warning, got: %v", warnings)
		}
	})

	t.Run("BOM file warns", func(t *testing.T) {
		dir := t.TempDir()
		bom := "\xEF\xBB\xBF"
		content := bom + "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
		writeErg(t, dir, "0001-test.erg", content)
		warnings := encodingWarnings(dir)
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "BOM") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected BOM warning, got: %v", warnings)
		}
	})

	t.Run("BOM and CRLF file warns both", func(t *testing.T) {
		dir := t.TempDir()
		bom := "\xEF\xBB\xBF"
		content := bom + "%erg 0.1\r\nTitle: X\r\nCreated: 2024-01-01\r\nAuthor: test\r\n\r\n--- log ---\r\n--- body ---\r\n"
		writeErg(t, dir, "0001-test.erg", content)
		warnings := encodingWarnings(dir)
		if len(warnings) < 2 {
			t.Errorf("expected 2 warnings (BOM + CRLF), got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("clean LF file no warnings", func(t *testing.T) {
		dir := t.TempDir()
		content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
		writeErg(t, dir, "0001-test.erg", content)
		warnings := encodingWarnings(dir)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings for clean LF file, got: %v", warnings)
		}
	})

	t.Run("non-erg files ignored", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello\r\nworld\r\n"), 0644); err != nil {
			t.Fatal(err)
		}
		warnings := encodingWarnings(dir)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings for non-erg file, got: %v", warnings)
		}
	})

	t.Run("empty dir returns nil", func(t *testing.T) {
		dir := t.TempDir()
		warnings := encodingWarnings(dir)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings for empty dir, got: %v", warnings)
		}
	})
}

// TestParse_BlankLineInsideHeaderBlock pins the read-tolerance tier from
// ticket 0141: a stray blank line inside the header block must not discard
// the headers below it. Before the fix both Label and Blocked-by landed in the
// silently-dropped gap, so the ticket validated clean and masked real errors.
func TestParse_BlankLineInsideHeaderBlock(t *testing.T) {
	src := "%erg 0.1\nTitle: t\nCreated: 2026-05-27\nAuthor: a\n\n" +
		"Label: needs-human\nBlocked-by: 0002\n\n" +
		"--- log ---\n2026-05-27T10:00Z a created\n\n--- body ---\n"
	e, _ := parseErgBytes([]byte(src), "0001-x.erg")
	if len(e.Labels) != 1 || e.Labels[0] != "needs-human" {
		t.Fatalf("Label after interior blank was dropped: %v", e.Labels)
	}
	if len(e.BlockedBys) != 1 {
		t.Fatalf("Blocked-by after interior blank was dropped: %v", e.BlockedBys)
	}
}

// TestParse_BlankBeforeRequiredHeaderFound pins the second failure mode: a
// blank line after Title must not hide Created/Author below it (they used to
// land in the dropped gap, producing misleading "missing required header"
// errors).
func TestParse_BlankBeforeRequiredHeaderFound(t *testing.T) {
	src := "%erg 0.1\nTitle: t\n\nCreated: 2026-05-27\nAuthor: a\n\n--- log ---\n--- body ---\n"
	_, errs := parseErgBytes([]byte(src), "0001-x.erg")
	for _, e := range errs {
		if strings.Contains(e, "missing or empty required header") {
			t.Fatalf("required header below interior blank reported missing: %v", errs)
		}
	}
}

// TestBlankEndsHeaderBlock_TerminatesNormally guards the boundary: the blank
// that legitimately precedes --- log --- must still end the header block, so
// trailing gap content is not swept back into the headers.
func TestParse_TrailingBlankStillEndsHeaderBlock(t *testing.T) {
	src := "%erg 0.1\nTitle: t\nCreated: 2026-05-27\nAuthor: a\n\nignored gap text\n--- log ---\n--- body ---\n"
	e, _ := parseErgBytes([]byte(src), "0001-x.erg")
	if e.Title != "t" || e.Created != "2026-05-27" || e.Author != "a" {
		t.Fatalf("headers before the terminating blank were lost: %+v", e)
	}
}

func TestHasInteriorHeaderBlank(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "interior blank between header clusters",
			src:  "%erg 0.1\nTitle: t\nCreated: 2026-05-27\nAuthor: a\n\nLabel: needs-human\n--- log ---\n--- body ---\n",
			want: true,
		},
		{
			name: "blank right after magic before headers",
			src:  "%erg 0.1\n\nTitle: t\nCreated: 2026-05-27\nAuthor: a\n--- log ---\n--- body ---\n",
			want: true,
		},
		{
			name: "only the terminating blank before separator",
			src:  "%erg 0.1\nTitle: t\nCreated: 2026-05-27\nAuthor: a\n\n--- log ---\n--- body ---\n",
			want: false,
		},
		{
			name: "no blank at all",
			src:  "%erg 0.1\nTitle: t\nCreated: 2026-05-27\nAuthor: a\n--- log ---\n--- body ---\n",
			want: false,
		},
		{
			name: "trailing gap text after blank is not interior",
			src:  "%erg 0.1\nTitle: t\nCreated: 2026-05-27\nAuthor: a\n\ngap line\n--- log ---\n--- body ---\n",
			want: false,
		},
		{
			name: "blank lines in the body are untouched",
			src:  "%erg 0.1\nTitle: t\nCreated: 2026-05-27\nAuthor: a\n\n--- log ---\n--- body ---\npara one\n\npara two\n",
			want: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasInteriorHeaderBlank([]byte(c.src)); got != c.want {
				t.Errorf("hasInteriorHeaderBlank = %v, want %v", got, c.want)
			}
		})
	}
}

func TestCollapseHeaderBlanks(t *testing.T) {
	t.Run("removes interior blank keeps terminating blank", func(t *testing.T) {
		src := "%erg 0.1\nTitle: t\nCreated: 2026-05-27\nAuthor: a\n\nLabel: needs-human\nBlocked-by: 0002\n\n--- log ---\n2026-05-27T10:00Z a created\n--- body ---\nbody\n"
		want := "%erg 0.1\nTitle: t\nCreated: 2026-05-27\nAuthor: a\nLabel: needs-human\nBlocked-by: 0002\n\n--- log ---\n2026-05-27T10:00Z a created\n--- body ---\nbody\n"
		got := string(collapseHeaderBlanks([]byte(src)))
		if got != want {
			t.Errorf("collapseHeaderBlanks\n got: %q\nwant: %q", got, want)
		}
	})

	t.Run("idempotent on clean file", func(t *testing.T) {
		src := "%erg 0.1\nTitle: t\nCreated: 2026-05-27\nAuthor: a\n\n--- log ---\n--- body ---\n"
		got := string(collapseHeaderBlanks([]byte(src)))
		if got != src {
			t.Errorf("collapse altered a clean file:\n got: %q\nwant: %q", got, src)
		}
	})

	t.Run("body blank lines untouched", func(t *testing.T) {
		src := "%erg 0.1\nTitle: t\nCreated: 2026-05-27\nAuthor: a\n\n--- log ---\n--- body ---\npara one\n\npara two\n"
		got := string(collapseHeaderBlanks([]byte(src)))
		if got != src {
			t.Errorf("collapse touched body blanks:\n got: %q\nwant: %q", got, src)
		}
	})
}
