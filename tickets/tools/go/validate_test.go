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
			name:       "Tags value not in closed set",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\nTags: invalid-tag\n\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "unknown Tags value",
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
			name:       "duplicate log separator",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- log ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "--- log ---",
		},
		{
			name:       "duplicate body separator",
			filename:   "0001-test.erg",
			content:    "%erg v1\nTitle: X\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n--- body ---\n",
			wantErrors: true,
			wantSubstr: "--- body ---",
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
			erg := parseErg(path)
			errs := validateErg(&erg, map[string]bool{})
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
		tickets := loadErgs(dir)
		errs := detectCycles(tickets)
		if len(errs) != 0 {
			t.Errorf("expected no errors for DAG, got: %v", errs)
		}
	})

	t.Run("self-loop A blocked-by A", func(t *testing.T) {
		dir := t.TempDir()
		makeTicket(t, dir, "0001", "0001")
		tickets := loadErgs(dir)
		errs := detectCycles(tickets)
		if len(errs) == 0 {
			t.Error("expected cycle error for self-loop, got none")
		}
	})

	t.Run("2-node cycle A->B B->A", func(t *testing.T) {
		dir := t.TempDir()
		makeTicket(t, dir, "0001", "0002")
		makeTicket(t, dir, "0002", "0001")
		tickets := loadErgs(dir)
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
		tickets := loadErgs(dir)
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
		tickets := loadErgs(dir)
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
		tickets := loadErgs(dir)
		errs := detectCycles(tickets)
		if len(errs) == 0 {
			t.Error("expected cycle errors for two disjoint cycles, got none")
		}
	})
}
