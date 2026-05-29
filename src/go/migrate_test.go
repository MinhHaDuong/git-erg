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
			name:        "Tags renamed to Tag preserving value",
			input:       "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\nTags: needs-human\n\n--- log ---\n--- body ---\n",
			wantContent: "%erg 0.1\nTitle: T\nCreated: 2024-01-01\nAuthor: test\nTag: needs-human\n\n--- log ---\n--- body ---\n",
			wantChanged: true,
			check: func(t *testing.T, r migrateResult) {
				if !r.tagsRenamed {
					t.Error("tagsRenamed = false, want true")
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
				if r.statusStripped || r.tagsRenamed || r.magicRewritten || r.blanksSwept {
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
