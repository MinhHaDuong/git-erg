package main

import (
	"strings"
	"testing"
)

func TestAppendLogLine(t *testing.T) {
	cases := []struct{ name, content, line, want string }{
		{
			name:    "inserts before body separator",
			content: "%erg 0.1\n--- log ---\n--- body ---\n",
			line:    "2026-01-01T10:00Z entry",
			want:    "%erg 0.1\n--- log ---\n2026-01-01T10:00Z entry\n--- body ---\n",
		},
		{
			name:    "fallback: appends to end when no body separator",
			content: "%erg 0.1\n--- log ---\n",
			line:    "2026-01-01T10:00Z entry",
			want:    "%erg 0.1\n--- log ---\n2026-01-01T10:00Z entry\n",
		},
		{
			name:    "adds newline before entry if content lacks trailing newline",
			content: "%erg 0.1\n--- log ---",
			line:    "2026-01-01T10:00Z entry",
			want:    "%erg 0.1\n--- log ---\n2026-01-01T10:00Z entry\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := appendLogLine(c.content, c.line)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestRemoveBlockedByLine(t *testing.T) {
	cases := []struct{ name, content, id, want string }{
		{
			name:    "single blocker line present is removed",
			content: "%erg 0.1\nBlocked-by: 0042\n\n--- log ---\n",
			id:      "0042",
			want:    "%erg 0.1\n\n--- log ---\n",
		},
		{
			name:    "multiple blocker lines for same ID all removed",
			content: "%erg 0.1\nBlocked-by: 0042\nTitle: foo\nBlocked-by: 0042\n\n--- log ---\n",
			id:      "0042",
			want:    "%erg 0.1\nTitle: foo\n\n--- log ---\n",
		},
		{
			name:    "blocker line for different ID preserved",
			content: "%erg 0.1\nBlocked-by: 0099\nBlocked-by: 0042\n\n--- log ---\n",
			id:      "0042",
			want:    "%erg 0.1\nBlocked-by: 0099\n\n--- log ---\n",
		},
		{
			name:    "no blocker lines leaves content unchanged",
			content: "%erg 0.1\nTitle: something\n\n--- log ---\n",
			id:      "0042",
			want:    "%erg 0.1\nTitle: something\n\n--- log ---\n",
		},
		{
			name:    "trailing whitespace on blocker line still matched",
			content: "%erg 0.1\nBlocked-by: 0042 \t\n\n--- log ---\n",
			id:      "0042",
			want:    "%erg 0.1\n\n--- log ---\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := removeBlockedByLine(c.content, c.id)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestInsertClosedHeader(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		headerLine string
		want       string
		wantErr    bool
	}{
		{
			name:       "normal ticket with blank line before log separator",
			content:    "%erg 0.1\nTitle: foo\nCreated: 2026-01-01\n\n--- log ---\n",
			headerLine: "Closed: done",
			want:       "%erg 0.1\nTitle: foo\nCreated: 2026-01-01\nClosed: done\n\n--- log ---\n",
			wantErr:    false,
		},
		{
			name:       "no blank line before log separator",
			content:    "%erg 0.1\nTitle: foo\n--- log ---\n",
			headerLine: "Closed: done",
			want:       "%erg 0.1\nTitle: foo\nClosed: done\n--- log ---\n",
			wantErr:    false,
		},
		{
			name:       "missing log separator returns error",
			content:    "%erg 0.1\nTitle: foo\n",
			headerLine: "Closed: done",
			want:       "",
			wantErr:    true,
		},
		{
			name:       "multiple blank lines before separator preserved after header",
			content:    "%erg 0.1\nTitle: foo\n\n\n--- log ---\n",
			headerLine: "Closed: done",
			want:       "%erg 0.1\nTitle: foo\nClosed: done\n\n\n--- log ---\n",
			wantErr:    false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := insertClosedHeader(c.content, c.headerLine)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "--- log ---") {
					t.Errorf("error message %q should mention missing separator", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
