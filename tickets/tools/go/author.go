package main

import (
	"os"
	"os/exec"
	"strings"
)

// gitConfigUserName is overridable in tests so they don't fork subprocesses.
var gitConfigUserName = func() string {
	out, err := exec.Command("git", "config", "user.name").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// resolveAuthor returns the first non-empty value from:
//  1. $ERG_AUTHOR      — explicit override
//  2. git config user.name
//  3. $USER
//  4. "unknown"
//
// All values are stripped of newlines and carriage returns so that a
// multi-line env var cannot inject extra header lines into the ticket file.
func resolveAuthor() string {
	sanitize := func(s string) string {
		return strings.NewReplacer("\n", "", "\r", "").Replace(s)
	}
	if v := sanitize(os.Getenv("ERG_AUTHOR")); v != "" {
		return v
	}
	if v := sanitize(gitConfigUserName()); v != "" {
		return v
	}
	if v := sanitize(os.Getenv("USER")); v != "" {
		return v
	}
	return "unknown"
}
