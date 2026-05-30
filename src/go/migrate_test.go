package main

import (
	"os"
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
