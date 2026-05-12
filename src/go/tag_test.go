package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveTagLine(t *testing.T) {
	cases := []struct{ name, content, tag, want string }{
		{
			name:    "removes single tag line",
			content: "%erg 0.1\nTag: needs-human\nTitle: foo\n\n--- log ---\n",
			tag:     "needs-human",
			want:    "%erg 0.1\nTitle: foo\n\n--- log ---\n",
		},
		{
			name:    "removes tag with trailing space",
			content: "%erg 0.1\nTag: needs-human \nTitle: foo\n\n--- log ---\n",
			tag:     "needs-human",
			want:    "%erg 0.1\nTitle: foo\n\n--- log ---\n",
		},
		{
			name:    "preserves other tags",
			content: "%erg 0.1\nTag: needs-human\nTag: deferred\n\n--- log ---\n",
			tag:     "needs-human",
			want:    "%erg 0.1\nTag: deferred\n\n--- log ---\n",
		},
		{
			name:    "no matching tag leaves content unchanged",
			content: "%erg 0.1\nTag: deferred\n\n--- log ---\n",
			tag:     "needs-human",
			want:    "%erg 0.1\nTag: deferred\n\n--- log ---\n",
		},
		{
			name:    "does not remove partial match",
			content: "%erg 0.1\nTag: needs-human-review\n\n--- log ---\n",
			tag:     "needs-human",
			want:    "%erg 0.1\nTag: needs-human-review\n\n--- log ---\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := removeTagLine(c.content, c.tag)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestCmdTag_AddsHeaderAndLog(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7001-taggable.erg", `%erg 0.1
Title: Taggable ticket
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
`)

	old := gitConfigUserName
	gitConfigUserName = func() string { return "" }
	defer func() { gitConfigUserName = old }()
	t.Setenv("ERG_AUTHOR", "testuser")

	rc := cmdTag([]string{"7001", "needs-human", dir})
	if rc != 0 {
		t.Fatalf("cmdTag returned %d, want 0", rc)
	}

	data, err := os.ReadFile(filepath.Join(dir, "7001-taggable.erg"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Tag header must appear in preamble (before --- log ---)
	logIdx := strings.Index(content, "--- log ---")
	tagIdx := strings.Index(content, "Tag: needs-human")
	if tagIdx < 0 || tagIdx > logIdx {
		t.Error("Tag: needs-human header not found in preamble")
	}

	// Log entry must be present
	if !strings.Contains(content, "testuser tag needs-human") {
		t.Error("log entry for tag not found")
	}
}

func TestCmdTag_Idempotent(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7002-already-tagged.erg", `%erg 0.1
Title: Already tagged
Created: 2026-01-01
Author: claude
Tag: needs-human

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
`)

	before, _ := os.ReadFile(filepath.Join(dir, "7002-already-tagged.erg"))

	rc := cmdTag([]string{"7002", "needs-human", dir})
	if rc != 0 {
		t.Fatalf("cmdTag returned %d, want 0", rc)
	}

	after, _ := os.ReadFile(filepath.Join(dir, "7002-already-tagged.erg"))
	if string(before) != string(after) {
		t.Error("idempotent tag modified the file")
	}
}

func TestCmdTag_RejectsUnknownTag(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7003-reject.erg", `%erg 0.1
Title: Reject unknown tag
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
`)

	before, _ := os.ReadFile(filepath.Join(dir, "7003-reject.erg"))

	rc := cmdTag([]string{"7003", "invalid-tag", dir})
	if rc != 1 {
		t.Fatalf("cmdTag returned %d, want 1", rc)
	}

	after, _ := os.ReadFile(filepath.Join(dir, "7003-reject.erg"))
	if string(before) != string(after) {
		t.Error("rejected tag modified the file")
	}
}

func TestCmdUntag_RemovesHeaderAndLog(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7004-untaggable.erg", `%erg 0.1
Title: Untaggable ticket
Created: 2026-01-01
Author: claude
Tag: needs-human

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
`)

	old := gitConfigUserName
	gitConfigUserName = func() string { return "" }
	defer func() { gitConfigUserName = old }()
	t.Setenv("ERG_AUTHOR", "testuser")

	rc := cmdUntag([]string{"7004", "needs-human", dir})
	if rc != 0 {
		t.Fatalf("cmdUntag returned %d, want 0", rc)
	}

	data, err := os.ReadFile(filepath.Join(dir, "7004-untaggable.erg"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Tag header must be gone
	if strings.Contains(content, "Tag: needs-human") {
		t.Error("Tag: needs-human header still present after untag")
	}

	// Log entry must be present
	if !strings.Contains(content, "testuser untag needs-human") {
		t.Error("log entry for untag not found")
	}
}

func TestCmdUntag_IdempotentNotTagged(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7005-not-tagged.erg", `%erg 0.1
Title: Not tagged
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
`)

	before, _ := os.ReadFile(filepath.Join(dir, "7005-not-tagged.erg"))

	rc := cmdUntag([]string{"7005", "needs-human", dir})
	if rc != 0 {
		t.Fatalf("cmdUntag returned %d, want 0", rc)
	}

	after, _ := os.ReadFile(filepath.Join(dir, "7005-not-tagged.erg"))
	if string(before) != string(after) {
		t.Error("idempotent untag modified the file")
	}
}

// writeTestTicket is a helper that writes a ticket file in dir.
func writeTestTicket(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
