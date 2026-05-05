package main

import "testing"

func TestAppendLogLine(t *testing.T) {
	cases := []struct{ name, content, line, want string }{
		{
			name:    "inserts before body separator",
			content: "%erg v1\n--- log ---\n--- body ---\n",
			line:    "2026-01-01T10:00Z entry",
			want:    "%erg v1\n--- log ---\n2026-01-01T10:00Z entry\n--- body ---\n",
		},
		{
			name:    "fallback: appends to end when no body separator",
			content: "%erg v1\n--- log ---\n",
			line:    "2026-01-01T10:00Z entry",
			want:    "%erg v1\n--- log ---\n2026-01-01T10:00Z entry\n",
		},
		{
			name:    "adds newline before entry if content lacks trailing newline",
			content: "%erg v1\n--- log ---",
			line:    "2026-01-01T10:00Z entry",
			want:    "%erg v1\n--- log ---\n2026-01-01T10:00Z entry\n",
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
