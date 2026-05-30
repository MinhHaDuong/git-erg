package main

import (
	"os"
	"strings"
	"testing"
)

func TestResolveAuthor(t *testing.T) {
	origErgAuthor, ergSet := os.LookupEnv("ERG_AUTHOR")
	origUser, userSet := os.LookupEnv("USER")
	origGitConfig := gitConfigUserName
	defer func() {
		gitConfigUserName = origGitConfig
		if ergSet {
			os.Setenv("ERG_AUTHOR", origErgAuthor)
		} else {
			os.Unsetenv("ERG_AUTHOR")
		}
		if userSet {
			os.Setenv("USER", origUser)
		} else {
			os.Unsetenv("USER")
		}
	}()

	t.Run("ERG_AUTHOR wins over all", func(t *testing.T) {
		os.Setenv("ERG_AUTHOR", "explicit-author")
		gitConfigUserName = func() string { return "git-user" }
		os.Setenv("USER", "os-user")
		if got := resolveAuthor(); got != "explicit-author" {
			t.Errorf("got %q, want explicit-author", got)
		}
	})

	t.Run("git config wins when ERG_AUTHOR unset", func(t *testing.T) {
		os.Unsetenv("ERG_AUTHOR")
		gitConfigUserName = func() string { return "git-user" }
		os.Setenv("USER", "os-user")
		if got := resolveAuthor(); got != "git-user" {
			t.Errorf("got %q, want git-user", got)
		}
	})

	t.Run("USER wins when git config empty", func(t *testing.T) {
		os.Unsetenv("ERG_AUTHOR")
		gitConfigUserName = func() string { return "" }
		os.Setenv("USER", "os-user")
		if got := resolveAuthor(); got != "os-user" {
			t.Errorf("got %q, want os-user", got)
		}
	})

	t.Run("unknown when all empty", func(t *testing.T) {
		os.Unsetenv("ERG_AUTHOR")
		gitConfigUserName = func() string { return "" }
		os.Unsetenv("USER")
		if got := resolveAuthor(); got != "unknown" {
			t.Errorf("got %q, want unknown", got)
		}
	})

	// Security class guard: each resolveAuthor branch routes through
	// sanitizeAuthor. Feed control chars via every input path and assert
	// they are stripped in the returned value. Distinguishing mutation:
	// strings.NewReplacer("\n","","\r","") -> NewReplacer("\n","\n","\r","\r").
	t.Run("ERG_AUTHOR with embedded newline is sanitized", func(t *testing.T) {
		os.Setenv("ERG_AUTHOR", "evil\nTitle: pwn")
		gitConfigUserName = func() string { return "" }
		os.Unsetenv("USER")
		got := resolveAuthor()
		if strings.ContainsAny(got, "\n\r") {
			t.Errorf("resolveAuthor() via ERG_AUTHOR = %q: control chars not stripped", got)
		}
		if got != "evilTitle: pwn" {
			t.Errorf("resolveAuthor() = %q, want %q", got, "evilTitle: pwn")
		}
	})

	t.Run("git config with embedded CR is sanitized", func(t *testing.T) {
		os.Unsetenv("ERG_AUTHOR")
		gitConfigUserName = func() string { return "alice\rField: injected" }
		os.Unsetenv("USER")
		got := resolveAuthor()
		if strings.ContainsAny(got, "\n\r") {
			t.Errorf("resolveAuthor() via git config = %q: control chars not stripped", got)
		}
		if got != "aliceField: injected" {
			t.Errorf("resolveAuthor() = %q, want %q", got, "aliceField: injected")
		}
	})

	t.Run("USER with embedded newline is sanitized", func(t *testing.T) {
		os.Unsetenv("ERG_AUTHOR")
		gitConfigUserName = func() string { return "" }
		os.Setenv("USER", "bob\nAuthor: hijack")
		got := resolveAuthor()
		if strings.ContainsAny(got, "\n\r") {
			t.Errorf("resolveAuthor() via USER = %q: control chars not stripped", got)
		}
		if got != "bobAuthor: hijack" {
			t.Errorf("resolveAuthor() = %q, want %q", got, "bobAuthor: hijack")
		}
	})
}
