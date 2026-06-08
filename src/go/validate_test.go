package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeErg writes content to a file named name inside dir and returns the path.
func writeErg(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeErg: %v", err)
	}
	return path
}

// errsContain reports whether any element of errs contains substr. Used
// pervasively in parser/validator tests where assertions are on error
// strings, not positional indices or trace booleans.
func errsContain(errs []string, substr string) bool {
	for _, e := range errs {
		if strings.Contains(e, substr) {
			return true
		}
	}
	return false
}

// validErgContent returns a minimal valid .erg ticket body (all rules satisfied).
func validErgContent() string {
	return "%erg 0.1\nTitle: My Ticket\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
}

func TestValidateErg(t *testing.T) {
	cases := []struct {
		name       string
		filename   string
		content    string
		wantErrors bool
		wantSubstr string // non-empty: at least one error must contain this
	}{
		{
			name:       "clean ticket",
			filename:   "0001-test.erg",
			content:    validErgContent(),
			wantErrors: false,
		},
		{
			name:       "missing magic line",
			filename:   "0001-test.erg",
			content:    "Title: foo\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "magic",
		},
		{
			name:       "missing required Title header",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "Title",
		},
		{
			name:       "Status header present",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nStatus: open\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "Status",
		},
		{
			name:       "unknown header",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nFoo: bar\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "unknown header",
		},
		{
			name:       "Created not a valid ISO date",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: not-a-date\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "ISO date",
		},
		{
			name:       "Blocked-by unknown ID",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: 0042\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "unknown ticket ID",
		},
		{
			name:       "missing log separator",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "log",
		},
		{
			name:       "missing body separator",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n",
			wantErrors: true,
			wantSubstr: "body",
		},
		{
			name:       "singleton header appears twice",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: First\nTitle: Again\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "non-repeatable",
		},
		{
			name:       "Label value not in closed set",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nLabel: invalid-label\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "unknown Label value",
		},
		{
			name:       "Label post-talk rejected (not in defaults)",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nLabel: post-talk\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "unknown Label value",
		},
		{
			name:       "Label post-conference rejected (not in defaults)",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nLabel: post-conference\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "unknown Label value",
		},
		{
			name:       "Tag header rejected with migration hint",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nTag: needs-human\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "renamed to 'Label:'",
		},
		{
			name:       "Tags header rejected with migration hint",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nTags: needs-human\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "renamed to 'Label:'",
		},
		{
			name:       "Closed header with empty value",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nClosed:\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "non-empty",
		},
		{
			name:       "Closed header in log section",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\nClosed: merged\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "log section",
		},
		{
			name:       "Closed header in body section",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\nClosed: merged\n",
			wantErrors: true,
			wantSubstr: "body section",
		},
		{
			name:       "filename does not match NNNN-slug pattern",
			filename:   "badname.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "NNNN-slug",
		},
		{
			name:       "malformed log line",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\nnot-a-valid-log-line\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "malformed log line",
		},
		{
			// Rule 12 relaxation (ticket 0116): duplicate separators are
			// no longer an error. The first occurrence transitions sections;
			// subsequent ones are body text.
			name:       "duplicate log separator accepted",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- log ---\n--- body ---\n",
			wantErrors: false,
		},
		{
			name:       "duplicate body separator accepted",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n--- body ---\n",
			wantErrors: false,
		},
		{
			name:       "invalid Blocked-by ref (deprecated gh: scheme)",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: gh:foo/bar#1\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "deprecated",
		},
		{
			name:       "missing required Created header",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "Created",
		},
		{
			name:       "missing required Author header",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "Author",
		},
		{
			name:       "invalid Blocked-by ref (deprecated bare gh# scheme)",
			filename:   "0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: gh#42\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "deprecated",
		},
		{
			name:       "Closed header with non-empty value accepted",
			filename:   "closed/0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nClosed: done\n\n--- log ---\n--- body ---\n",
			wantErrors: false,
		},
		{
			// Rule 15: a Superseded-by local ref pointing at a ticket ID not
			// present in the (single-ticket) corpus is an unknown-ref error.
			// 0099 is neither self nor present, so this isolates the cross-ref
			// check from the self-ref guard.
			name:       "Superseded-by unknown ID",
			filename:   "closed/0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nClosed: done\nSuperseded-by: 0099\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "unknown ticket ID",
		},
		{
			// Rule 15: self-reference is a parse-time error (a ticket cannot
			// supersede itself). 0001 in 0001-test.erg resolves in the
			// single-ticket corpus, so only the self-ref guard can flag it.
			name:       "Superseded-by self-ref",
			filename:   "closed/0001-test.erg",
			content:    "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nClosed: done\nSuperseded-by: 0001\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "self-reference",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if sub := filepath.Dir(tc.filename); sub != "." {
				os.MkdirAll(filepath.Join(dir, sub), 0755)
			}
			path := writeErg(t, dir, tc.filename, tc.content)
			erg, parseErrs := parseErg(path)
			// Run the corpus-level rules (10, 13, duplicate IDs) over a
			// single-ticket "corpus" so rule 10 (local-ref resolution)
			// still fires when the test fixture asserts on it. Filename
			// IDs are derived from the file basename; only "NNNN-..."
			// fixtures will populate the local-id set, matching today's
			// expectations.
			errs := validateCorpus([]Erg{erg}, [][]string{parseErrs}, nil)
			if tc.wantErrors && len(errs) == 0 {
				t.Errorf("expected at least one validation error, got none")
				return
			}
			if !tc.wantErrors && len(errs) > 0 {
				t.Errorf("expected no validation errors, got: %v", errs)
				return
			}
			if tc.wantSubstr != "" && !errsContain(errs, tc.wantSubstr) {
				t.Errorf("expected an error containing %q, got: %v", tc.wantSubstr, errs)
			}
		})
	}
}

func TestDetectCycles(t *testing.T) {
	// Helper: write a minimal ticket with optional Blocked-by lines.
	makeTicket := func(t *testing.T, dir, id string, blockedBy ...string) {
		t.Helper()
		var sb strings.Builder
		sb.WriteString("%erg 0.1\n")
		sb.WriteString("Title: ticket " + id + "\n")
		sb.WriteString("Created: 2024-01-01\n")
		sb.WriteString("Author: test\n")
		for _, dep := range blockedBy {
			sb.WriteString("Blocked-by: " + dep + "\n")
		}
		sb.WriteString("\n--- log ---\n--- body ---\n")
		name := id + "-test.erg"
		writeErg(t, dir, name, sb.String())
	}

	t.Run("empty slice", func(t *testing.T) {
		errs := detectCycles([]Erg{})
		if len(errs) != 0 {
			t.Errorf("expected no errors, got: %v", errs)
		}
	})

	t.Run("simple DAG A depends on B", func(t *testing.T) {
		dir := t.TempDir()
		makeTicket(t, dir, "0001", "0002")
		makeTicket(t, dir, "0002")
		tickets, _ := loadErgs(dir)
		errs := detectCycles(tickets)
		if len(errs) != 0 {
			t.Errorf("expected no errors for DAG, got: %v", errs)
		}
	})

	t.Run("self-loop A blocked-by A", func(t *testing.T) {
		dir := t.TempDir()
		makeTicket(t, dir, "0001", "0001")
		tickets, _ := loadErgs(dir)
		errs := detectCycles(tickets)
		if len(errs) == 0 {
			t.Error("expected cycle error for self-loop, got none")
		}
	})

	t.Run("2-node cycle A->B B->A", func(t *testing.T) {
		dir := t.TempDir()
		makeTicket(t, dir, "0001", "0002")
		makeTicket(t, dir, "0002", "0001")
		tickets, _ := loadErgs(dir)
		errs := detectCycles(tickets)
		if len(errs) == 0 {
			t.Error("expected cycle error for 2-node cycle, got none")
		}
	})

	t.Run("3-node cycle A->B->C->A", func(t *testing.T) {
		dir := t.TempDir()
		makeTicket(t, dir, "0001", "0002")
		makeTicket(t, dir, "0002", "0003")
		makeTicket(t, dir, "0003", "0001")
		tickets, _ := loadErgs(dir)
		errs := detectCycles(tickets)
		if len(errs) == 0 {
			t.Error("expected cycle error for 3-node cycle, got none")
		}
	})

	t.Run("branched DAG A->B A->C B->D C->D", func(t *testing.T) {
		dir := t.TempDir()
		makeTicket(t, dir, "0001", "0002", "0003")
		makeTicket(t, dir, "0002", "0004")
		makeTicket(t, dir, "0003", "0004")
		makeTicket(t, dir, "0004")
		tickets, _ := loadErgs(dir)
		errs := detectCycles(tickets)
		if len(errs) != 0 {
			t.Errorf("expected no errors for branched DAG, got: %v", errs)
		}
	})

	t.Run("multiple disjoint cycles", func(t *testing.T) {
		// Two independent cycles: A->B->A and C->D->C.
		// Both must be detected -- the DFS must not stop after the first cycle.
		dir := t.TempDir()
		makeTicket(t, dir, "0001", "0002")
		makeTicket(t, dir, "0002", "0001")
		makeTicket(t, dir, "0003", "0004")
		makeTicket(t, dir, "0004", "0003")
		tickets, _ := loadErgs(dir)
		errs := detectCycles(tickets)
		if len(errs) == 0 {
			t.Error("expected cycle errors for two disjoint cycles, got none")
		}
	})
}

func TestValidateErg_GoldenValid(t *testing.T) {
	fixtures, _ := filepath.Glob("testdata/valid/*.erg")
	// Build allIDs from the fixture filenames so adding a new fixture
	// doesn't require updating this test.
	allIDs := map[string]bool{}
	for _, path := range fixtures {
		base := filepath.Base(path)
		if len(base) >= 4 {
			allIDs[base[:4]] = true
		}
	}
	for _, path := range fixtures {
		t.Run(filepath.Base(path), func(t *testing.T) {
			erg, errs := parseErg(path)
			// Per-file errors must be empty. Rule 10 (local ref resolution)
			// lives in validateCorpus, but valid fixtures should not have
			// dangling local refs; re-check inline against the synthesized
			// allIDs.
			for _, ref := range erg.BlockedBys {
				if ref.Kind == RefLocal && !allIDs[ref.ID] {
					errs = append(errs, "unresolved local ref "+ref.ID)
				}
			}
			if len(errs) != 0 {
				t.Errorf("expected no errors, got: %v", errs)
			}
		})
	}
}

func TestValidateErg_GoldenInvalid(t *testing.T) {
	// Each fixture must produce an error message containing the listed
	// substring. Without per-fixture matching, a fixture could fail for
	// any unrelated rule and the test would pass -- turning the golden
	// suite into a tautology.
	wantSubstr := map[string]string{
		"0001-bad-created-date.erg":  "Created",
		"0001-bad-label.erg":         "unknown Label value",
		"0001-bad-superseded-by.erg": "references unknown ticket ID",
		"0001-empty-closed.erg":      "non-empty",
		"0001-bad-forge-host.erg":    "malformed ref",
		"0001-bad-log-timestamp.erg": "log line",
		"0001-bad-log-verb.erg":      "log line",
		"0001-bad-status.erg":        "Status",
		"0001-duplicate-title.erg":   "non-repeatable",
		"0001-missing-body.erg":      "body",
		"0001-missing-log.erg":       "log",
		"0001-missing-created.erg":   "Created",
		"0001-missing-author.erg":    "Author",
		"0001-missing-title.erg":     "Title",
		"0001-unknown-header.erg":    "unknown header",
		"0001-wrong-magic.erg":       "%erg 0.1",
		"0001-legacy-v1.erg":         "erg migrate",
		"bad-filename.erg":           "filename",
	}
	fixtures, _ := filepath.Glob("testdata/invalid/*.erg")
	for _, path := range fixtures {
		t.Run(filepath.Base(path), func(t *testing.T) {
			// Run the full validator, not just parseErg: some header-value
			// rules (e.g. Label vocabulary) live in validateCorpus, not the
			// parser. validateCorpus merges parseErrs and only appends corpus
			// errors, so every parser-level fixture keeps its error too.
			// nil cfg uses the default label set (matching TestValidateErg).
			erg, parseErrs := parseErg(path)
			errs := validateCorpus([]Erg{erg}, [][]string{parseErrs}, nil)
			if len(errs) == 0 {
				t.Fatalf("expected at least one error, got none")
			}
			want, ok := wantSubstr[filepath.Base(path)]
			if !ok {
				t.Fatalf("no wantSubstr entry for %q -- add one to the map", filepath.Base(path))
			}
			if !errsContain(errs, want) {
				t.Errorf("expected an error containing %q, got: %v", want, errs)
			}
		})
	}
}

// headerKeysWithoutFixture returns the keys in keys that have no invalid
// fixture exercising their validation path. A key is "covered" when some
// testdata/invalid/*.erg references it -- either the key (lowercased) appears
// in the fixture's filename (e.g. "0001-bad-label.erg" for Label) or the
// fixture content carries a literal "Key:" header line (e.g.
// "0001-bad-forge-host.erg" carries a "Blocked-by:" line). The exemptions set
// names keys deliberately left without a fixture; it is empty by design --
// every header key should have a negative test. Extracted as a helper so the
// meta-test's negative control can drive it with a doctored key set.
func headerKeysWithoutFixture(keys map[string]bool, fixtureDir string, exemptions map[string]bool) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(fixtureDir, "*.erg"))
	if err != nil {
		return nil, err
	}
	type fixture struct {
		name    string
		content string
	}
	fixtures := make([]fixture, 0, len(paths))
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		fixtures = append(fixtures, fixture{name: filepath.Base(p), content: string(data)})
	}
	var missing []string
	for key := range keys {
		if exemptions[key] {
			continue
		}
		covered := false
		lowerKey := strings.ToLower(key)
		headerLine := key + ":"
		for _, f := range fixtures {
			if strings.Contains(strings.ToLower(f.name), lowerKey) {
				covered = true
				break
			}
			// Match the header at a line start so "Label:" inside a body or a
			// substring of another key does not falsely count.
			for _, line := range strings.Split(f.content, "\n") {
				if strings.HasPrefix(line, headerLine) {
					covered = true
					break
				}
			}
			if covered {
				break
			}
		}
		if !covered {
			missing = append(missing, key)
		}
	}
	return missing, nil
}

// TestV1HeaderKeys_FixtureCoverage is a meta-test: every recognised header key
// (v1HeaderKeys, ergspecv1.go) must have a golden invalid fixture exercising
// its validation path, so the validation surface cannot silently shrink when a
// key is added. The exemption map is empty by design. A negative control runs
// the same check against a key set with an injected bogus key and asserts the
// gap is reported -- proving the check has teeth.
func TestV1HeaderKeys_FixtureCoverage(t *testing.T) {
	exemptions := map[string]bool{} // empty by design: every key gets a fixture

	missing, err := headerKeysWithoutFixture(v1HeaderKeys, "testdata/invalid", exemptions)
	if err != nil {
		t.Fatalf("scanning fixtures: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("header keys with no invalid fixture: %v -- add a testdata/invalid/*.erg exercising each (or an explicit exemption)", missing)
	}

	// Negative control: inject a key that no fixture can possibly reference and
	// confirm the check flags it. Guards against a predicate that always
	// reports "covered".
	doctored := map[string]bool{"Bogusnonexistentkey": true}
	gaps, err := headerKeysWithoutFixture(doctored, "testdata/invalid", exemptions)
	if err != nil {
		t.Fatalf("negative control scan: %v", err)
	}
	if len(gaps) != 1 || gaps[0] != "Bogusnonexistentkey" {
		t.Errorf("negative control: expected injected key reported as missing, got %v", gaps)
	}
}

func TestValidateCorpus_GoldenDuplicateIDs(t *testing.T) {
	tickets, parseErrs := loadErgs("testdata/invalid-duplicate")
	errs := validateCorpus(tickets, parseErrs, nil)
	if !errsContain(errs, "duplicate ID") {
		t.Errorf("expected at least one 'duplicate ID' error, got: %v", errs)
	}
}

// TestSeparatorLiteralInBodyAccepted pins the rule 12 relaxation from
// ticket 0116: a body that quotes the `--- log ---` / `--- body ---`
// literals must validate AND round-trip through erg.Body byte-for-byte.
// Also pins the parser invariants that HasLogSep/HasBodySep are set on
// ANY sighting and that body lines are preserved verbatim.
func TestSeparatorLiteralInBodyAccepted(t *testing.T) {
	body := "Example of the format:\n--- log ---\n--- body ---\nEnd.\n"
	content := "%erg 0.1\n" +
		"Title: Doc\n" +
		"Created: 2024-01-01\n" +
		"Author: test\n" +
		"\n--- log ---\n--- body ---\n" + body
	path := writeErg(t, t.TempDir(), "0001-doc.erg", content)

	erg, errs := parseErg(path)

	// (a) parser accepts the file (rule 12 relaxed).
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got: %v", errs)
	}

	// (b) erg.Body preserves the literal lines verbatim. Byte-exact
	// comparison catches silent line loss or extra blank lines. The
	// parser preserves the trailing newline (one element per "\n"
	// split, joined back), so the want value matches the input body
	// exactly.
	if erg.Body != body {
		t.Errorf("erg.Body = %q, want %q", erg.Body, body)
	}

	// (c) parser counts both separators as seen -- the absence of the
	// missing-separator errors is the post-merge assertion (formerly
	// diag.HasLogSep / diag.HasBodySep).
	if errsContain(errs, "missing '--- log ---'") {
		t.Errorf("expected no missing-log-separator error, got: %v", errs)
	}
	if errsContain(errs, "missing '--- body ---'") {
		t.Errorf("expected no missing-body-separator error, got: %v", errs)
	}
}

// TestRequiredHeaderEmptyValueRejected pins the rule 2 tightening from
// ticket 0116: a required header that is present with an empty value
// must fail validation. Today's behavior accepted `Title: ` (key present,
// empty value) because rule 2 only checked key presence.
func TestRequiredHeaderEmptyValueRejected(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"empty Title", "%erg 0.1\nTitle: \nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n", "Title"},
		{"empty Created", "%erg 0.1\nTitle: X\nCreated: \nAuthor: test\n\n--- log ---\n--- body ---\n", "Created"},
		{"empty Author", "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: \n\n--- log ---\n--- body ---\n", "Author"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeErg(t, t.TempDir(), "0001-test.erg", tc.content)
			_, errs := parseErg(path)
			found := false
			for _, e := range errs {
				if strings.Contains(e, tc.want) && strings.Contains(e, "empty") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected an error mentioning %q with 'empty', got: %v", tc.want, errs)
			}
		})
	}
}

// TestClosedDuplicateWithEmpty pins that `Closed: x` followed by
// `Closed: ` triggers both the repeated-singleton error and the
// empty-value error. Confirms ClosedEmpty is set on ANY empty value,
// not only the first occurrence.
func TestClosedDuplicateWithEmpty(t *testing.T) {
	content := "%erg 0.1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nClosed: x\nClosed: \n\n--- log ---\n--- body ---\n"
	path := writeErg(t, t.TempDir(), "0001-test.erg", content)
	_, errs := parseErg(path)

	var sawRepeated, sawEmpty bool
	for _, e := range errs {
		if strings.Contains(e, "non-repeatable") && strings.Contains(e, "Closed") {
			sawRepeated = true
		}
		if strings.Contains(e, "Closed:") && strings.Contains(e, "non-empty") {
			sawEmpty = true
		}
	}
	if !sawRepeated {
		t.Errorf("expected a 'non-repeatable' Closed error, got: %v", errs)
	}
	if !sawEmpty {
		t.Errorf("expected a Closed empty-value error, got: %v", errs)
	}
}

// TestGlobLocalIDs exercises globLocalIDs (validate.go:247).
// Item 3: verifies non-recursive scan of .erg files and ID extraction.
func TestGlobLocalIDs(t *testing.T) {
	t.Run("extracts IDs from NNNN-slug.erg files", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0001-alpha.erg", validErgContent())
		writeErg(t, dir, "0042-beta.erg", validErgContent())

		ids := globLocalIDs(dir)
		if !ids["0001"] {
			t.Error("expected ID '0001' in result")
		}
		if !ids["0042"] {
			t.Error("expected ID '0042' in result")
		}
		if len(ids) != 2 {
			t.Errorf("expected 2 IDs, got %d: %v", len(ids), ids)
		}
	})

	t.Run("ignores subdirectories", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0001-alpha.erg", validErgContent())
		sub := filepath.Join(dir, "closed")
		if err := os.MkdirAll(sub, 0755); err != nil {
			t.Fatal(err)
		}
		writeErg(t, sub, "0002-beta.erg", validErgContent())

		ids := globLocalIDs(dir)
		if ids["0002"] {
			t.Error("globLocalIDs should not recurse into subdirectories")
		}
		if len(ids) != 1 {
			t.Errorf("expected 1 ID, got %d: %v", len(ids), ids)
		}
	})

	t.Run("ignores non-erg files", func(t *testing.T) {
		dir := t.TempDir()
		writeErg(t, dir, "0001-alpha.erg", validErgContent())
		// Write a non-.erg file
		if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0644); err != nil {
			t.Fatal(err)
		}

		ids := globLocalIDs(dir)
		if len(ids) != 1 {
			t.Errorf("expected 1 ID, got %d: %v", len(ids), ids)
		}
	})

	t.Run("empty directory returns empty map", func(t *testing.T) {
		dir := t.TempDir()
		ids := globLocalIDs(dir)
		if len(ids) != 0 {
			t.Errorf("expected empty map, got: %v", ids)
		}
	})

	t.Run("nonexistent directory returns empty map", func(t *testing.T) {
		ids := globLocalIDs(filepath.Join(t.TempDir(), "nonexistent"))
		if len(ids) != 0 {
			t.Errorf("expected empty map, got: %v", ids)
		}
	})
}

// titleErg builds a minimal .erg with the given Title. When closed is true a
// non-empty Closed: header is added so the ticket is grandfathered (rule 14).
func titleErg(title string, closed bool) []byte {
	c := "%erg 0.1\nTitle: " + title + "\nCreated: 2026-01-01\nAuthor: claude\n"
	if closed {
		c += "Closed: superseded\n"
	}
	c += "\n--- log ---\n--- body ---\n"
	return []byte(c)
}

// TestTitleStatusWordRule pins rule 14 (ticket 0145): a Title may not begin
// or end with a status word, the rule names the offending word and edge, it
// ignores surrounding punctuation, a mid-title status word is fine, and
// closed tickets are grandfathered.
func TestTitleStatusWordRule(t *testing.T) {
	cases := []struct {
		name     string
		title    string
		closed   bool
		wantErr  bool
		wantWord string // substring required when wantErr (e.g. "'ready'")
		wantEdge string // "begins with" or "ends with" when wantErr
	}{
		{name: "begins ready", title: "ready: demote claimed signal", wantErr: true, wantWord: "'ready'", wantEdge: "begins with"},
		{name: "begins done", title: "done queue draining", wantErr: true, wantWord: "'done'", wantEdge: "begins with"},
		{name: "begins closed", title: "closed-loop retry handling", wantErr: true, wantWord: "'closed'", wantEdge: "begins with"},
		{name: "begins open", title: "open the config reader", wantErr: true, wantWord: "'open'", wantEdge: "begins with"},
		{name: "ends ready", title: "make the queue ready", wantErr: true, wantWord: "'ready'", wantEdge: "ends with"},
		{name: "ends done", title: "mark the migration done", wantErr: true, wantWord: "'done'", wantEdge: "ends with"},
		{name: "ends closed", title: "ensure the handle is closed", wantErr: true, wantWord: "'closed'", wantEdge: "ends with"},
		{name: "ends open", title: "leave the port open", wantErr: true, wantWord: "'open'", wantEdge: "ends with"},
		{name: "ends with trailing punctuation", title: "ensure the handle is closed.", wantErr: true, wantWord: "'closed'", wantEdge: "ends with"},
		{name: "case-insensitive", title: "Ready to ship the thing", wantErr: true, wantWord: "'ready'", wantEdge: "begins with"},
		{name: "mid-title status word ok", title: "respect the open flag in the parser", wantErr: false},
		{name: "status word as substring ok", title: "openness audit for the reader", wantErr: false},
		{name: "clean title ok", title: "add the rm command", wantErr: false},
		{name: "grandfathered closed begins", title: "ready: demote claimed signal", closed: true, wantErr: false},
		{name: "grandfathered closed ends", title: "make the queue ready", closed: true, wantErr: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, errs := parseErgBytes(titleErg(c.title, c.closed), "0001-x.erg")
			got := errsContain(errs, "status word")
			if got != c.wantErr {
				t.Fatalf("status-word error = %v, want %v (errs: %v)", got, c.wantErr, errs)
			}
			if c.wantErr {
				if !errsContain(errs, c.wantWord) {
					t.Errorf("expected error naming word %s, got: %v", c.wantWord, errs)
				}
				if !errsContain(errs, c.wantEdge) {
					t.Errorf("expected error naming edge %q, got: %v", c.wantEdge, errs)
				}
			}
		})
	}
}

// TestTitleStatusWordRule_Fixtures runs the golden invalid-title fixtures:
// every file must yield a status-word error pointing at filename:LINE.
func TestTitleStatusWordRule_Fixtures(t *testing.T) {
	fixtures, _ := filepath.Glob("testdata/invalid-title/*.erg")
	if len(fixtures) == 0 {
		t.Fatal("no testdata/invalid-title fixtures found")
	}
	for _, path := range fixtures {
		t.Run(filepath.Base(path), func(t *testing.T) {
			_, errs := parseErg(path)
			if !errsContain(errs, "status word") {
				t.Errorf("expected a status-word error, got: %v", errs)
			}
			// Error must point at the Title line (filename:LINE form).
			if !errsContain(errs, filepath.Base(path)+":2:") {
				t.Errorf("expected error pointing at %s:2, got: %v", filepath.Base(path), errs)
			}
		})
	}
}

// TestSupersededByCorpus exercises rule 15 (Superseded-by cross-reference) in a
// multi-ticket corpus -- the single-ticket TestValidateErg harness cannot hold
// a resolving ref without it being a self-reference, so a genuine "accepted"
// case needs two tickets.
func TestSupersededByCorpus(t *testing.T) {
	t.Run("closed carrier with resolving ref accepted", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "closed"), 0755)
		// 0001 is the replacement (open); 0002 is the closed old ticket that
		// points at it. The header lives on the CLOSED ticket (in closed/).
		writeErg(t, dir, "0001-replacement.erg",
			"%erg 0.1\nTitle: Replacement\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n")
		writeErg(t, dir, "closed/0002-old.erg",
			"%erg 0.1\nTitle: Old\nCreated: 2024-01-01\nAuthor: test\nClosed: superseded\nSuperseded-by: 0001\n\n--- log ---\n--- body ---\n")

		tickets, parseErrs := loadErgs(dir)
		errs := validateCorpus(tickets, parseErrs, nil)
		if len(errs) != 0 {
			t.Errorf("expected no errors for a resolving Superseded-by ref, got: %v", errs)
		}
	})

	t.Run("repeatable: two Superseded-by lines accepted", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "closed"), 0755)
		// One-to-many supersession: 0003 superseded by both 0001 and 0002.
		writeErg(t, dir, "0001-repl-a.erg",
			"%erg 0.1\nTitle: A\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n")
		writeErg(t, dir, "0002-repl-b.erg",
			"%erg 0.1\nTitle: B\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n")
		writeErg(t, dir, "closed/0003-old.erg",
			"%erg 0.1\nTitle: Old\nCreated: 2024-01-01\nAuthor: test\nClosed: superseded\nSuperseded-by: 0001\nSuperseded-by: 0002\n\n--- log ---\n--- body ---\n")

		tickets, parseErrs := loadErgs(dir)
		// Both refs must be parsed (catches accidental singleton-key
		// registration, which would error on the second line).
		var old Erg
		for i := range tickets {
			if tickets[i].FilenameID() == "0003" {
				old = tickets[i]
			}
		}
		if len(old.SupersededBys) != 2 {
			t.Fatalf("expected 2 parsed Superseded-by refs, got %d", len(old.SupersededBys))
		}
		errs := validateCorpus(tickets, parseErrs, nil)
		if len(errs) != 0 {
			t.Errorf("expected no errors for two resolving Superseded-by refs, got: %v", errs)
		}
	})
}
