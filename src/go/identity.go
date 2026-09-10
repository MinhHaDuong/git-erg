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

// sanitizeAuthor strips newlines and carriage returns from s so that a
// multi-line value cannot inject extra header lines into a ticket file.
func sanitizeAuthor(s string) string {
	return strings.NewReplacer("\n", "", "\r", "").Replace(s)
}

// actorToken renders an author as a single whitespace-delimited token, which is
// what a log line's actor slot structurally requires. The log format is
// positional -- `timestamp actor verb [detail]` -- so an actor containing a
// space pushes the verb over by one and every reader mis-parses the entry. That
// is not hypothetical: `git config user.name` is commonly "First Last", and
// resolveAuthor() returns it verbatim.
//
// Runs of whitespace collapse to a single "-", so "Minh Ha Duong" logs as
// "Minh-Ha-Duong" instead of corrupting the line. Returns "" when nothing
// survives (an all-whitespace value), leaving the caller to decide between an
// error and a fallback -- the two call sites want different answers.
//
// Only log lines need this. `Author:` headers hold the rest of the line, so
// spaces there are harmless and new.go keeps writing the author verbatim.
func actorToken(s string) string {
	return strings.Join(strings.Fields(sanitizeAuthor(s)), "-")
}

// resolveAuthor returns the first non-empty value from:
//  1. $ERG_AUTHOR      -- explicit override
//  2. git config user.name
//  3. $USER
//  4. "unknown"
//
// All values are stripped of newlines and carriage returns so that a
// multi-line env var cannot inject extra header lines into the ticket file.
func resolveAuthor() string {
	if v := sanitizeAuthor(os.Getenv("ERG_AUTHOR")); v != "" {
		return v
	}
	if v := sanitizeAuthor(gitConfigUserName()); v != "" {
		return v
	}
	if v := sanitizeAuthor(os.Getenv("USER")); v != "" {
		return v
	}
	return "unknown"
}
