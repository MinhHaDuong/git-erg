package main

import "testing"

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Hello World", "hello-world"},
		{"My TICKET: with special—chars & more!", "my-ticket-with-special-chars-more"},
		{"em—dash collapsed", "em-dash-collapsed"},
		{"consecutive---hyphens", "consecutive-hyphens"},
		{"-leading and trailing-", "leading-and-trailing"},
		{"this is a very long title that exceeds forty characters definitely",
			// truncated to 40 chars: "this-is-a-very-long-title-that-exceeds-f"
			// trailing char is 'f' (from "forty"), not a hyphen, so TrimRight is a no-op
			"this-is-a-very-long-title-that-exceeds-f"},
		{"", "untitled"},
		{"!@#$%^&*()", "untitled"},
	}
	for _, c := range cases {
		got := slugify(c.in)
		if got != c.want {
			t.Errorf("slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
