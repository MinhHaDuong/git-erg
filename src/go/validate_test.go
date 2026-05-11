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

// validErgContent returns a minimal valid .erg ticket body (all rules satisfied).
func validErgContent() string {
	return "%erg v1\nTitle: My Ticket\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n"
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
			content:    "%erg v1\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "Title",
		},
		{
			name:       "Status header present",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nStatus: open\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "Status",
		},
		{
			name:       "unknown header",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nFoo: bar\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "unknown header",
		},
		{
			name:       "Created not a valid ISO date",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: not-a-date\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "ISO date",
		},
		{
			name:       "Blocked-by unknown ID",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: 0042\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "unknown ticket ID",
		},
		{
			name:       "missing log separator",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "log",
		},
		{
			name:       "missing body separator",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n",
			wantErrors: true,
			wantSubstr: "body",
		},
		{
			name:       "singleton header appears twice",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: First\nTitle: Again\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "non-repeatable",
		},
		{
			name:       "Tag value not in closed set",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nTag: invalid-tag\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "unknown Tag value",
		},
		{
			name:       "Tag post-talk accepted",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nTag: post-talk\n\n--- log ---\n--- body ---\n",
			wantErrors: false,
		},
		{
			name:       "Tag post-conference accepted",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nTag: post-conference\n\n--- log ---\n--- body ---\n",
			wantErrors: false,
		},
		{
			name:       "Tags header rejected with migration hint",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nTags: needs-human\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "renamed to 'Tag:'",
		},
		{
			name:       "Closed header with empty value",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nClosed:\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "non-empty",
		},
		{
			name:       "Closed header in log section",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\nClosed: merged\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "log section",
		},
		{
			name:       "Closed header in body section",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\nClosed: merged\n",
			wantErrors: true,
			wantSubstr: "body section",
		},
		{
			name:       "filename does not match NNNN-slug pattern",
			filename:   "badname.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "NNNN-slug",
		},
		{
			name:       "malformed log line",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\nnot-a-valid-log-line\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "malformed log line",
		},
		{
			// Rule 12 relaxation (ticket 0116): duplicate separators are
			// no longer an error. The first occurrence transitions sections;
			// subsequent ones are body text.
			name:       "duplicate log separator accepted",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- log ---\n--- body ---\n",
			wantErrors: false,
		},
		{
			name:       "duplicate body separator accepted",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n--- body ---\n",
			wantErrors: false,
		},
		{
			name:       "invalid Blocked-by ref (deprecated gh: scheme)",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: gh:foo/bar#1\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "deprecated",
		},
		{
			name:       "missing required Created header",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "Created",
		},
		{
			name:       "missing required Author header",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "Author",
		},
		{
			name:       "invalid Blocked-by ref (deprecated bare gh# scheme)",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nBlocked-by: gh#42\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "deprecated",
		},
		{
			name:       "Closed header with non-empty value accepted",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nClosed: done\n\n--- log ---\n--- body ---\n",
			wantErrors: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeErg(t, dir, tc.filename, tc.content)
			erg, diag := parseErg(path)
			errs := validateErg(&erg, diag, map[string]bool{})
			if tc.wantErrors && len(errs) == 0 {
				t.Errorf("expected at least one validation error, got none")
				return
			}
			if !tc.wantErrors && len(errs) > 0 {
				t.Errorf("expected no validation errors, got: %v", errs)
				return
			}
			if tc.wantSubstr != "" {
				found := false
				for _, e := range errs {
					if strings.Contains(e, tc.wantSubstr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected an error containing %q, got: %v", tc.wantSubstr, errs)
				}
			}
		})
	}
}

func TestDetectCycles(t *testing.T) {
	// Helper: write a minimal ticket with optional Blocked-by lines.
	makeTicket := func(t *testing.T, dir, id string, blockedBy ...string) {
		t.Helper()
		var sb strings.Builder
		sb.WriteString("%erg v1\n")
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
		// Both must be detected — the DFS must not stop after the first cycle.
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
			erg, diag := parseErg(path)
			errs := validateErg(&erg, diag, allIDs)
			if len(errs) != 0 {
				t.Errorf("expected no errors, got: %v", errs)
			}
		})
	}
}

func TestValidateErg_GoldenInvalid(t *testing.T) {
	// Each fixture must produce an error message containing the listed
	// substring. Without per-fixture matching, a fixture could fail for
	// any unrelated rule and the test would pass — turning the golden
	// suite into a tautology.
	wantSubstr := map[string]string{
		"0001-bad-created-date.erg":  "Created",
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
		"0001-wrong-magic.erg":       "%erg v1",
		"bad-filename.erg":           "filename",
	}
	fixtures, _ := filepath.Glob("testdata/invalid/*.erg")
	for _, path := range fixtures {
		t.Run(filepath.Base(path), func(t *testing.T) {
			erg, diag := parseErg(path)
			errs := validateErg(&erg, diag, map[string]bool{})
			if len(errs) == 0 {
				t.Fatalf("expected at least one error, got none")
			}
			want, ok := wantSubstr[filepath.Base(path)]
			if !ok {
				t.Fatalf("no wantSubstr entry for %q — add one to the map", filepath.Base(path))
			}
			found := false
			for _, e := range errs {
				if strings.Contains(e, want) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected an error containing %q, got: %v", want, errs)
			}
		})
	}
}

func TestValidateAll_GoldenDuplicateIDs(t *testing.T) {
	tickets, diags := loadErgs("testdata/invalid-duplicate")
	errs := validateAll(tickets, diags)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "duplicate ID") {
			found = true
			break
		}
	}
	if !found {
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
	content := "%erg v1\n" +
		"Title: Doc\n" +
		"Created: 2024-01-01\n" +
		"Author: test\n" +
		"\n--- log ---\n--- body ---\n" + body
	path := writeErg(t, t.TempDir(), "0001-doc.erg", content)

	erg, diag := parseErg(path)

	// (a) validator accepts the file (rule 12 relaxed).
	errs := validateErg(&erg, diag, map[string]bool{"0001": true})
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

	// (c) parser flags both separators as seen.
	if !diag.HasLogSep {
		t.Error("ParseDiagnostics.HasLogSep = false, want true")
	}
	if !diag.HasBodySep {
		t.Error("ParseDiagnostics.HasBodySep = false, want true")
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
		{"empty Title", "%erg v1\nTitle: \nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n", "Title"},
		{"empty Created", "%erg v1\nTitle: X\nCreated: \nAuthor: test\n\n--- log ---\n--- body ---\n", "Created"},
		{"empty Author", "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: \n\n--- log ---\n--- body ---\n", "Author"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeErg(t, t.TempDir(), "0001-test.erg", tc.content)
			erg, diag := parseErg(path)
			errs := validateErg(&erg, diag, map[string]bool{})
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
	content := "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nClosed: x\nClosed: \n\n--- log ---\n--- body ---\n"
	path := writeErg(t, t.TempDir(), "0001-test.erg", content)
	erg, diag := parseErg(path)
	errs := validateErg(&erg, diag, map[string]bool{})

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
