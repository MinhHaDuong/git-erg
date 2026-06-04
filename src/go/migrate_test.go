package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateFile(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantContent string
		wantChanged bool
		check       func(t *testing.T, r migrateResult)
	}{
		{
			name:        "legacy magic line rewritten",
			input:       "%erg v1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantContent: "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantChanged: true,
			check: func(t *testing.T, r migrateResult) {
				if !r.magicRewritten {
					t.Error("magicRewritten = false, want true")
				}
			},
		},
		{
			name:        "status closed becomes Closed header",
			input:       "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\nStatus: closed\n\n--- log ---\n--- body ---\n",
			wantContent: "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\nClosed: migrated from Status: closed\n\n--- log ---\n--- body ---\n",
			wantChanged: true,
			check: func(t *testing.T, r migrateResult) {
				if !r.statusStripped || !r.wasClosed {
					t.Errorf("statusStripped=%v wasClosed=%v, want both true", r.statusStripped, r.wasClosed)
				}
			},
		},
		{
			name:        "status open is dropped without Closed header",
			input:       "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\nStatus: open\n\n--- log ---\n--- body ---\n",
			wantContent: "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantChanged: true,
			check: func(t *testing.T, r migrateResult) {
				if !r.statusStripped || r.wasClosed {
					t.Errorf("statusStripped=%v wasClosed=%v, want stripped without closed", r.statusStripped, r.wasClosed)
				}
			},
		},
		{
			name:        "Tag renamed to Label preserving value",
			input:       "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\nTag: needs-human\n\n--- log ---\n--- body ---\n",
			wantContent: "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\nLabel: needs-human\n\n--- log ---\n--- body ---\n",
			wantChanged: true,
			check: func(t *testing.T, r migrateResult) {
				if !r.labelsRewritten {
					t.Error("labelsRewritten = false, want true")
				}
			},
		},
		{
			name:        "legacy Tags converges to Label in one run",
			input:       "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\nTags: needs-human\n\n--- log ---\n--- body ---\n",
			wantContent: "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\nLabel: needs-human\n\n--- log ---\n--- body ---\n",
			wantChanged: true,
			check: func(t *testing.T, r migrateResult) {
				if !r.labelsRewritten {
					t.Error("labelsRewritten = false, want true")
				}
			},
		},
		{
			name:        "interior header blank swept",
			input:       "%erg 0.1\nTitle: T\nCreated: 2024-01-01\n\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantContent: "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantChanged: true,
			check: func(t *testing.T, r migrateResult) {
				if !r.blanksSwept {
					t.Error("blanksSwept = false, want true")
				}
			},
		},
		{
			name:        "already migrated file is a no-op",
			input:       "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantContent: "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n--- log ---\n--- body ---\n",
			wantChanged: false,
			check: func(t *testing.T, r migrateResult) {
				if r.statusStripped || r.labelsRewritten || r.magicRewritten || r.blanksSwept {
					t.Errorf("expected no transforms on clean file, got %+v", r)
				}
			},
		},
		{
			name:        "missing trailing newline is preserved",
			input:       "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\nStatus: closed\n\n--- log ---\n--- body ---",
			wantContent: "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\nClosed: migrated from Status: closed\n\n--- log ---\n--- body ---",
			wantChanged: true,
			check:       func(t *testing.T, r migrateResult) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeErg(t, dir, "0001-ticket.erg", tt.input)

			res, err := migrateFile(path)
			if err != nil {
				t.Fatalf("migrateFile: %v", err)
			}
			if res.changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", res.changed, tt.wantChanged)
			}

			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read back: %v", err)
			}
			if string(got) != tt.wantContent {
				t.Errorf("content mismatch:\n got: %q\nwant: %q", string(got), tt.wantContent)
			}
			tt.check(t, res)
		})
	}
}

func TestMigrateErgrc(t *testing.T) {
	t.Run("rewrites [tags] to [labels]", func(t *testing.T) {
		dir := t.TempDir()
		path := dir + "/.ergrc"
		if err := os.WriteFile(path, []byte("[tags]\nneeds-human\ndeferred\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if !migrateErgrc(dir) {
			t.Fatal("migrateErgrc = false, want true (file changed)")
		}
		got, _ := os.ReadFile(path)
		want := "[labels]\nneeds-human\ndeferred\n"
		if string(got) != want {
			t.Errorf("got %q, want %q", string(got), want)
		}
		// Idempotent: a second run is a no-op.
		if migrateErgrc(dir) {
			t.Error("second migrateErgrc should be a no-op")
		}
	})

	t.Run("absent .ergrc is a no-op", func(t *testing.T) {
		if migrateErgrc(t.TempDir()) {
			t.Error("migrateErgrc on dir without .ergrc should return false")
		}
	})
}

func TestMigrateFileIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := writeErg(t, dir, "0001-ticket.erg",
		"%erg v1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\nStatus: closed\nTags: deferred\n\n--- log ---\n--- body ---\n")

	first, err := migrateFile(path)
	if err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if !first.changed {
		t.Fatal("first migrate should have changed the file")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after first: %v", err)
	}

	second, err := migrateFile(path)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if second.changed {
		t.Error("second migrate should be a no-op")
	}
	again, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after second: %v", err)
	}
	if string(again) != string(after) {
		t.Errorf("second migrate altered content:\n first: %q\nsecond: %q", string(after), string(again))
	}
}

// TestFoldLogLines exercises the pure folding function directly: stamp
// normalisation, multi-form continuation folding, and blank handling (the
// orphan-untouched invariant is covered by TestFoldLogLinesOrphan).
func TestFoldLogLines(t *testing.T) {
	in := []string{
		"2024-01-01 alice created",                          // date-only stamp -> normalise
		"2024-01-02T10:00Z bob note long detail that wraps", // full-timestamp entry with continuation
		"",                        // blank between entry and its continuation -> dropped
		"  indented continuation", // indented wrap
		"unindented continuation", // unindented prose
		"1. numbered list item",   // numbered-list continuation
		"",                        // inter-entry blank -> preserved
		"2024-01-03T12:00Z carol note Short entry", // clean second entry (boundary)
		"", // trailing blank before --- body --- -> preserved
	}
	want := []string{
		"2024-01-01T00:00Z alice created",
		"2024-01-02T10:00Z bob note long detail that wraps indented continuation unindented continuation 1. numbered list item",
		"",
		"2024-01-03T12:00Z carol note Short entry",
		"",
	}
	out, folded, stamped := foldLogLines(in)
	if !folded {
		t.Error("folded = false, want true")
	}
	if !stamped {
		t.Error("stamped = false, want true")
	}
	if len(out) != len(want) {
		t.Fatalf("len(out) = %d, want %d\n got: %#v\nwant: %#v", len(out), len(want), out, want)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Errorf("out[%d] = %q, want %q", i, out[i], want[i])
		}
	}
}

// TestFoldLogLinesOrphan confirms orphan content before the first timestamped
// entry is emitted verbatim and NOT folded (no parent entry to fold onto).
func TestFoldLogLinesOrphan(t *testing.T) {
	in := []string{
		"orphan line with no timestamp",
		"2024-01-02T10:00Z alice note Real entry",
	}
	out, folded, stamped := foldLogLines(in)
	if folded {
		t.Error("folded = true, want false (orphan must not fold)")
	}
	if stamped {
		t.Error("stamped = true, want false")
	}
	if len(out) != 2 || out[0] != "orphan line with no timestamp" {
		t.Errorf("orphan not preserved verbatim: %#v", out)
	}
}

// TestMigrateFileLogFold checks the end-to-end migrateFile rewrite: a fixture
// with a date-only stamp, a wrapped multi-line detail, and inter-entry blanks
// folds to a clean, validate-passing store.
func TestMigrateFileLogFold(t *testing.T) {
	dir := t.TempDir()
	input := "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n" +
		"--- log ---\n" +
		"2024-01-01 alice created\n" +
		"2024-01-02T10:00Z bob note This is a long note that wraps\n" +
		"onto this line\n" +
		"and this line too\n" +
		"\n" +
		"2024-01-03T12:00Z carol note Short entry\n" +
		"\n" +
		"--- body ---\n" +
		"body text\n"
	want := "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n" +
		"--- log ---\n" +
		"2024-01-01T00:00Z alice created\n" +
		"2024-01-02T10:00Z bob note This is a long note that wraps onto this line and this line too\n" +
		"\n" +
		"2024-01-03T12:00Z carol note Short entry\n" +
		"\n" +
		"--- body ---\n" +
		"body text\n"
	path := writeErg(t, dir, "0001-ticket.erg", input)

	res, err := migrateFile(path)
	if err != nil {
		t.Fatalf("migrateFile: %v", err)
	}
	if !res.folded || !res.stamped || !res.changed {
		t.Errorf("res = %+v, want folded && stamped && changed", res)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != want {
		t.Errorf("content mismatch:\n got: %q\nwant: %q", string(got), want)
	}
	// The migrated store must pass validation (no malformed log lines).
	if _, errs := parseErg(path); len(errs) > 0 {
		t.Errorf("migrated file does not validate: %v", errs)
	}
}

// TestMigrateFileIdempotentFold runs migrateFile twice on a fold fixture and
// byte-compares the two outputs. The fixture deliberately carries blanks
// (trailing pre-body blank, inter-entry blank, entry/continuation blank) so a
// blank-accumulation regression on the second run would be caught.
func TestMigrateFileIdempotentFold(t *testing.T) {
	dir := t.TempDir()
	input := "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n" +
		"--- log ---\n" +
		"2024-01-01 alice created\n" + // date-only stamp
		"2024-01-02T10:00Z bob note wraps\n" + // entry with continuation
		"\n" + // blank between entry and continuation -> dropped
		"  indented detail\n" +
		"unindented detail\n" +
		"1. numbered item\n" +
		"\n" + // inter-entry blank -> preserved
		"2024-01-03T12:00Z carol note clean\n" + // full-timestamp, no continuation (regression guard)
		"\n" + // trailing blank before body -> preserved (CRITICAL invariant)
		"--- body ---\n" +
		"body text\n"
	path := writeErg(t, dir, "0001-ticket.erg", input)

	first, err := migrateFile(path)
	if err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if !first.changed {
		t.Fatal("first migrate should have changed the file")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after first: %v", err)
	}
	// The richest fixture (indented + unindented + numbered-list continuation,
	// date-only stamp, all blank forms) must validate after one migrate pass --
	// this is exit criterion 1 ("validate passes on every file").
	if _, errs := parseErg(path); len(errs) > 0 {
		t.Errorf("migrated fixture does not validate: %v", errs)
	}

	second, err := migrateFile(path)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if second.changed {
		t.Error("second migrate should be a no-op")
	}
	again, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after second: %v", err)
	}
	if string(again) != string(after) {
		t.Errorf("second migrate altered content (byte-compare):\n first: %q\nsecond: %q", string(after), string(again))
	}
}

// TestMigrateFileOrphanUntouched confirms orphan content before the first
// timestamped log entry is left verbatim and still flagged by the validator.
func TestMigrateFileOrphanUntouched(t *testing.T) {
	dir := t.TempDir()
	input := "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\n\n" +
		"--- log ---\n" +
		"orphan line with no timestamp\n" +
		"2024-01-02T10:00Z alice note Real entry\n" +
		"\n" +
		"--- body ---\n"
	path := writeErg(t, dir, "0001-ticket.erg", input)

	res, err := migrateFile(path)
	if err != nil {
		t.Fatalf("migrateFile: %v", err)
	}
	if res.folded {
		t.Error("res.folded = true, want false (orphan must not fold)")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(got), "orphan line with no timestamp") {
		t.Errorf("orphan line not preserved verbatim:\n%q", string(got))
	}
	// Validator must still flag the orphan as a malformed log line.
	_, errs := parseErg(path)
	foundMalformed := false
	for _, e := range errs {
		if strings.Contains(e, "malformed log line") {
			foundMalformed = true
			break
		}
	}
	if !foundMalformed {
		t.Errorf("expected a malformed-log-line violation for the orphan, got: %v", errs)
	}
}

// setupMigrateLayoutDir builds a hermetic project tree for migrateLayout tests:
// <tmp>/tickets/ named literally "tickets" so installAssets (which writes paths
// prefixed "tickets/" relative to root = filepath.Dir(dir)) targets it, plus a
// stub tickets/erg so migrateLayout skips the multi-MB os.Executable() self-copy.
// Returns the tickets dir to pass to migrateLayout.
func setupMigrateLayoutDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	ticketsDir := filepath.Join(root, "tickets")
	if err := os.MkdirAll(ticketsDir, 0755); err != nil {
		t.Fatalf("mkdir tickets: %v", err)
	}
	// Stub binary so migrateLayout does not copy os.Executable() into the tree.
	if err := os.WriteFile(filepath.Join(ticketsDir, "erg"), []byte("stub"), 0755); err != nil {
		t.Fatalf("write stub erg: %v", err)
	}
	return ticketsDir
}

// TestMigrateLayoutErgrc covers ticket 0224: migrate's asset refresh must stop
// touching .ergrc entirely (configuration delivery belongs to `erg init`), while
// AGENTS.md keeps the charter's force-overwrite behaviour.
func TestMigrateLayoutErgrc(t *testing.T) {
	// (a) A locally-edited .ergrc survives migrateLayout byte-for-byte.
	t.Run("locally-edited .ergrc survives migrateLayout", func(t *testing.T) {
		ticketsDir := setupMigrateLayoutDir(t)
		custom := []byte("[labels]\nMIGRATE-0224-UNIQUE-MARKER\nneeds-human\n")
		ergrcPath := filepath.Join(ticketsDir, ".ergrc")
		if err := os.WriteFile(ergrcPath, custom, 0644); err != nil {
			t.Fatalf("write .ergrc: %v", err)
		}
		if code := migrateLayout(ticketsDir); code != 0 {
			t.Fatalf("migrateLayout returned %d, want 0", code)
		}
		got, err := os.ReadFile(ergrcPath)
		if err != nil {
			t.Fatalf("read back .ergrc: %v", err)
		}
		if string(got) != string(custom) {
			t.Errorf(".ergrc was modified by migrateLayout:\n got: %q\nwant: %q", string(got), string(custom))
		}
	})

	// (b) An absent .ergrc is NOT created by migrateLayout.
	t.Run("absent .ergrc is not created by migrateLayout", func(t *testing.T) {
		ticketsDir := setupMigrateLayoutDir(t)
		ergrcPath := filepath.Join(ticketsDir, ".ergrc")
		if code := migrateLayout(ticketsDir); code != 0 {
			t.Fatalf("migrateLayout returned %d, want 0", code)
		}
		if _, err := os.Stat(ergrcPath); !os.IsNotExist(err) {
			t.Errorf(".ergrc should not exist after migrateLayout, stat err = %v", err)
		}
	})

	// (c) A diverged AGENTS.md IS force-overwritten by migrateLayout (charter
	// decision intact: agent docs track the binary). This guards the exclusion
	// from accidentally swallowing AGENTS.md.
	t.Run("diverged AGENTS.md is force-overwritten by migrateLayout", func(t *testing.T) {
		ticketsDir := setupMigrateLayoutDir(t)
		agentsPath := filepath.Join(ticketsDir, "AGENTS.md")
		diverged := []byte("DIVERGED\n")
		if err := os.WriteFile(agentsPath, diverged, 0644); err != nil {
			t.Fatalf("write AGENTS.md: %v", err)
		}
		if code := migrateLayout(ticketsDir); code != 0 {
			t.Fatalf("migrateLayout returned %d, want 0", code)
		}
		got, err := os.ReadFile(agentsPath)
		if err != nil {
			t.Fatalf("read back AGENTS.md: %v", err)
		}
		if string(got) == string(diverged) {
			t.Error("AGENTS.md was preserved, want force-overwrite with embedded content")
		}
		embedded, ok := bootstrapAsset("tickets/AGENTS.md")
		if !ok {
			t.Fatal("embedded tickets/AGENTS.md asset missing")
		}
		if string(got) != embedded {
			t.Errorf("AGENTS.md not overwritten with embedded content:\n got: %q", string(got))
		}
	})
}
