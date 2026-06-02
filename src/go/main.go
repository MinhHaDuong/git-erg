// Command erg validates and operates on %erg 0.1 ticket files. It depends only
// on the Go standard library.
//
// Usage:
//
//	erg validate FILE...
//	erg check    [dir]
//	erg list     [dir] [label...] [not label...] [--all] [--json]
//	erg ready    [dir] [--json]
//	erg next-id  [dir]
//	erg new      TITLE [DIR]
//	erg close    ID|FILE REASON [DIR]
//	erg log      ID LINE [DIR]
//	erg label    ID LABELNAME [DIR]
//	erg unlabel  ID LABELNAME [DIR]
//	erg archive  [id...] [dir]
//	erg rm       ID|FILE [dir] [--force]
//	erg migrate  [dir]
//	erg init     [dir]
//	erg install  [dir] [--hooks] [--inject-agents]
//	erg spec
//	erg integration
//	erg version
//	erg update
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// osExecutable is a seam for testing: tests can replace it to return a path
// inside a controlled directory without building a real binary.
var osExecutable = os.Executable

// manualPreamble is the header printed by `erg --help --all` before the
// per-command sections. The %s placeholder receives the literal string
// "Generated from: erg" -- no build stamp is embedded so the committed
// docs/erg-manual.md is stable across machines and CI rebuilds. Runtime
// build metadata is available via `erg version`.
const manualPreamble = `# erg manual

Author: minh.ha-duong@cnrs.fr
%s

` + "`git-erg`" + ` is an agent-friendly local ticket system for development in disconnected
environments. Tickets are plain-text files committed alongside source code.
This manual describes all ` + "`erg`" + ` commands. For the ticket file format
specification, run ` + "`erg spec`" + `.

**Store auto-discovery.** When no DIR is given, ` + "`erg`" + ` tries three candidates in
order: (1) the directory containing the ` + "`erg`" + ` binary, (2) ` + "`tickets/`" + ` under the
current working directory, (3) the current working directory itself. A directory
qualifies as a ticket store if its basename is ` + "`tickets`" + `, or if it contains at
least one ` + "`.erg`" + ` file. The first qualifying candidate is used; if none qualify,
` + "`erg`" + ` exits with an error listing the directories it tried.

When the store is auto-discovered, ` + "`erg`" + ` refuses to use a store that lies in
a different git worktree than the working directory. Pass DIR explicitly to override.

**Exit codes (shared by ` + "`check`" + ` and ` + "`init`" + `).** ` + "`0`" + ` success;
` + "`1`" + ` a hard error (bad flag, unreadable directory, write failure, or a
corpus violation); ` + "`2`" + ` local edits were preserved and skipped
(` + "`init`" + ` only -- run with ` + "`--force`" + ` to overwrite). Any non-zero
status is a failure for scripting purposes. The value ` + "`1`" + ` always means a
hard failure -- it never doubles as "skipped".
`

// looksLikeTicketStore reports whether dir is a managed ticket store.
func looksLikeTicketStore(dir string) bool {
	if _, err := os.Stat(dir); err != nil {
		return false
	}
	if filepath.Base(dir) == "tickets" {
		return true
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".erg") {
			return true
		}
	}
	return false
}

// findTicketsDir returns the ticket store directory by trying candidates in order:
// (1) directory containing the binary, (2) "tickets" under cwd, (3) cwd itself.
// Each candidate must satisfy looksLikeTicketStore. Returns an error if none qualify.
// When a qualifying candidate is found, it is checked for worktree conflict: if the
// store lies in a different git worktree than cwd, an error is returned. Pass an
// explicit DIR to resolveDir to bypass this check.
func findTicketsDir() (string, error) {
	var candidates []string
	if exe, err := osExecutable(); err == nil {
		candidates = append(candidates, filepath.Dir(exe))
	}
	candidates = append(candidates, "tickets", ".")

	for _, c := range candidates {
		if looksLikeTicketStore(c) {
			abs, err := filepath.Abs(c)
			if err != nil {
				abs = c
			}
			if err := storeWorktreeConflict(abs); err != nil {
				return "", err
			}
			return abs, nil
		}
	}

	cwd, _ := os.Getwd()
	tried := strings.Join(candidates, ", ")
	return "", fmt.Errorf(
		"did not find the tickets store. I looked at: %s\nTo confirm that directory %q is an intended tickets store, I need to see at least one .erg file in it.",
		tried, cwd)
}

// storeWorktreeConflict returns an error if store is inside a different git
// worktree than the current working directory. Returns nil when either
// worktree top cannot be determined (git unavailable, not in a worktree) or
// when both are the same -- the caller may proceed.
func storeWorktreeConflict(store string) error {
	storeTop := worktreeTopFor(store)
	cwdTop := worktreeTopFor(".")
	if storeTop == "" || cwdTop == "" || storeTop == cwdTop {
		return nil
	}
	return fmt.Errorf("erg: store found at %s (branch: %s)\n     but cwd is in a different worktree (branch: %s)\n     pass an explicit DIR to override",
		store, gitBranch(store), gitBranch("."))
}

type notADirError struct{ Path string }

func (e *notADirError) Error() string { return e.Path + " is not a directory" }

// Callers that need MkdirAll semantics (cmdNew) should not use resolveDir.
func resolveDir(explicit string) (string, error) {
	dir := explicit
	if dir == "" {
		var err error
		dir, err = findTicketsDir()
		if err != nil {
			return "", err
		}
	}
	dir = filepath.Clean(dir)
	info, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", &notADirError{Path: dir}
	}
	if branch := gitBranch(dir); branch != "" {
		cwd, _ := os.Getwd()
		fmt.Fprintf(os.Stderr, "erg: branch %s (%s)\n", branch, displayPath(cwd, dir))
	}
	return dir, nil
}

// displayPath returns a relative path from cwd to dir when the relative form
// does not escape (i.e. does not start with ".."), falling back to the
// absolute path of dir.
func displayPath(cwd, dir string) string {
	abs := dir
	if !filepath.IsAbs(dir) {
		abs = filepath.Clean(filepath.Join(cwd, dir))
	}
	rel, err := filepath.Rel(cwd, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return abs
	}
	return rel
}

func resolveTicketByID(dir, id string) (string, error) {
	pattern := filepath.Join(dir, fmt.Sprintf("%s-*.erg", id))
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", fmt.Errorf("no ticket found for ID %s in %s", id, dir)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous ID %s -- matches: %s", id, strings.Join(matches, ", "))
	}
	return matches[0], nil
}

func printUsage() {
	fmt.Fprintln(os.Stdout, "Usage: erg COMMAND [--help] [args...]")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Commands:")
	for _, c := range commands {
		nameArgs := c.Name
		if c.Args != "" {
			nameArgs += " " + c.Args
		}
		fmt.Fprintf(os.Stdout, "  %-26s%s\n", nameArgs, c.Summary)
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	rest := os.Args[2:]

	if canonical, ok := commandAliases[cmd]; ok {
		cmd = canonical
	}

	// erg --help --all  OR  erg --help=all  -> print all command help
	if cmd == "--help=all" || (cmd == "--help" || cmd == "-h") && len(rest) > 0 && rest[0] == "--all" {
		// "Generated from: erg" -- no build stamp embedded in the manual.
		// The committed docs/erg-manual.md is the same on every machine; the
		// running binary self-reports build/rev via `erg version`. Including
		// the stamp here would otherwise churn docs/erg-manual.md on every
		// local `make docs` because the CI bake-in lags the squash-merge SHA.
		fmt.Printf(manualPreamble, "Generated from: erg")
		for _, c := range commands {
			fmt.Print("\n" + c.Help)
		}
		os.Exit(0)
	}

	for _, arg := range rest {
		if arg == "--help" || arg == "-h" {
			found := false
			for _, c := range commands {
				if c.Name == cmd {
					fmt.Print(c.Help)
					found = true
					break
				}
			}
			if !found {
				printUsage()
			}
			os.Exit(0)
		}
	}

	var exitCode int
	switch cmd {
	case "validate":
		exitCode = cmdValidate(rest)
	case "check":
		exitCode = cmdCheck(rest)
	case "list":
		exitCode = cmdList(rest)
	case "ready":
		exitCode = cmdReady(rest)
	case "next-id":
		exitCode = cmdNextID(rest)
	case "new":
		exitCode = cmdNew(rest)
	case "close":
		exitCode = cmdClose(rest)
	case "log":
		exitCode = cmdLog(rest)
	case "label":
		exitCode = cmdLabel(rest)
	case "unlabel":
		exitCode = cmdUnlabel(rest)
	case "archive":
		exitCode = cmdArchive(rest)
	case "rm":
		exitCode = cmdRm(rest)
	case "migrate":
		exitCode = cmdMigrate(rest)
	case "init":
		exitCode = cmdInit(rest)
	case "install":
		exitCode = cmdInstall(rest)
	case "spec":
		exitCode = cmdSpec(rest)
	case "integration":
		exitCode = cmdIntegration(rest)
	case "version":
		exitCode = cmdVersion(rest)
	case "update":
		exitCode = cmdUpdate(rest)
	case "-h", "--help", "help":
		printUsage()
		exitCode = 0
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		exitCode = 1
	}
	os.Exit(exitCode)
}
