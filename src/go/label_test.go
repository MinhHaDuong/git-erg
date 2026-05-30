package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveLabelLine(t *testing.T) {
	cases := []struct{ name, content, label, want string }{
		{
			name:    "removes single label line",
			content: "%erg 0.1\nLabel: needs-human\nTitle: foo\n\n--- log ---\n",
			label:   "needs-human",
			want:    "%erg 0.1\nTitle: foo\n\n--- log ---\n",
		},
		{
			name:    "removes label with trailing space",
			content: "%erg 0.1\nLabel: needs-human \nTitle: foo\n\n--- log ---\n",
			label:   "needs-human",
			want:    "%erg 0.1\nTitle: foo\n\n--- log ---\n",
		},
		{
			name:    "preserves other labels",
			content: "%erg 0.1\nLabel: needs-human\nLabel: deferred\n\n--- log ---\n",
			label:   "needs-human",
			want:    "%erg 0.1\nLabel: deferred\n\n--- log ---\n",
		},
		{
			name:    "no matching label leaves content unchanged",
			content: "%erg 0.1\nLabel: deferred\n\n--- log ---\n",
			label:   "needs-human",
			want:    "%erg 0.1\nLabel: deferred\n\n--- log ---\n",
		},
		{
			name:    "does not remove partial match",
			content: "%erg 0.1\nLabel: needs-human-review\n\n--- log ---\n",
			label:   "needs-human",
			want:    "%erg 0.1\nLabel: needs-human-review\n\n--- log ---\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := removeLabelLine(c.content, c.label)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestCmdLabel_AddsHeaderAndLog(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7001-labelable.erg", `%erg 0.1
Title: Labelable ticket
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

	rc := cmdLabel([]string{"7001", "needs-human", dir})
	if rc != 0 {
		t.Fatalf("cmdLabel returned %d, want 0", rc)
	}

	data, err := os.ReadFile(filepath.Join(dir, "7001-labelable.erg"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Label header must appear in preamble (before --- log ---)
	logIdx := strings.Index(content, "--- log ---")
	labelIdx := strings.Index(content, "Label: needs-human")
	if labelIdx < 0 || labelIdx > logIdx {
		t.Error("Label: needs-human header not found in preamble")
	}

	// Log entry must be present
	if !strings.Contains(content, "testuser label needs-human") {
		t.Error("log entry for label not found")
	}
}

func TestCmdLabel_Idempotent(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7002-already-labeled.erg", `%erg 0.1
Title: Already labeled
Created: 2026-01-01
Author: claude
Label: needs-human

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
`)

	before, _ := os.ReadFile(filepath.Join(dir, "7002-already-labeled.erg"))

	rc := cmdLabel([]string{"7002", "needs-human", dir})
	if rc != 0 {
		t.Fatalf("cmdLabel returned %d, want 0", rc)
	}

	after, _ := os.ReadFile(filepath.Join(dir, "7002-already-labeled.erg"))
	if string(before) != string(after) {
		t.Error("idempotent label modified the file")
	}
}

func TestCmdLabel_RejectsUnknownLabel(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7003-reject.erg", `%erg 0.1
Title: Reject unknown label
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
`)

	before, _ := os.ReadFile(filepath.Join(dir, "7003-reject.erg"))

	rc := cmdLabel([]string{"7003", "invalid-label", dir})
	if rc != 1 {
		t.Fatalf("cmdLabel returned %d, want 1", rc)
	}

	after, _ := os.ReadFile(filepath.Join(dir, "7003-reject.erg"))
	if string(before) != string(after) {
		t.Error("rejected label modified the file")
	}
}

func TestCmdUnlabel_RemovesHeaderAndLog(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7004-unlabelable.erg", `%erg 0.1
Title: Unlabelable ticket
Created: 2026-01-01
Author: claude
Label: needs-human

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
`)

	old := gitConfigUserName
	gitConfigUserName = func() string { return "" }
	defer func() { gitConfigUserName = old }()
	t.Setenv("ERG_AUTHOR", "testuser")

	rc := cmdUnlabel([]string{"7004", "needs-human", dir})
	if rc != 0 {
		t.Fatalf("cmdUnlabel returned %d, want 0", rc)
	}

	data, err := os.ReadFile(filepath.Join(dir, "7004-unlabelable.erg"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Label header must be gone
	if strings.Contains(content, "Label: needs-human") {
		t.Error("Label: needs-human header still present after unlabel")
	}

	// Log entry must be present
	if !strings.Contains(content, "testuser unlabel needs-human") {
		t.Error("log entry for unlabel not found")
	}
}

func TestCmdUnlabel_IdempotentNotLabeled(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "7005-not-labeled.erg", `%erg 0.1
Title: Not labeled
Created: 2026-01-01
Author: claude

--- log ---
2026-01-01T10:00Z claude created

--- body ---
Test body.
`)

	before, _ := os.ReadFile(filepath.Join(dir, "7005-not-labeled.erg"))

	rc := cmdUnlabel([]string{"7005", "needs-human", dir})
	if rc != 0 {
		t.Fatalf("cmdUnlabel returned %d, want 0", rc)
	}

	after, _ := os.ReadFile(filepath.Join(dir, "7005-not-labeled.erg"))
	if string(before) != string(after) {
		t.Error("idempotent unlabel modified the file")
	}
}

// writeTestTicket is a helper that writes a ticket file in dir.
func writeTestTicket(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
