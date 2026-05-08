// Command erg validates and operates on %erg v1 ticket files. It depends only
// on the Go standard library.
//
// Usage:
//
//	erg validate FILE...
//	erg check    [dir]
//	erg ready    [dir] [--json]
//	erg next-id  [dir]
//	erg new      TITLE [DIR]
//	erg close    ID|FILE REASON [DIR]
//	erg log      ID LINE [DIR]
//	erg archive  [id...] [dir]
//	erg migrate  [dir]
//	erg init     [dir]
//	erg version
//	erg update
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
func findTicketsDir() (string, error) {
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(exe))
	}
	candidates = append(candidates, "tickets", ".")

	for _, c := range candidates {
		if looksLikeTicketStore(c) {
			return c, nil
		}
	}

	cwd, _ := os.Getwd()
	tried := strings.Join(candidates, ", ")
	return "", fmt.Errorf(
		"did not find the tickets store. I looked at: %s\nTo confirm that directory %q is an intended tickets store, I need to see at least one .erg file in it.",
		tried, cwd)
}

func printUsage() {
	fmt.Fprintln(os.Stdout, "Usage: erg COMMAND [--help] [args...]")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Commands:")
	fmt.Fprintln(os.Stdout, "  validate FILES...         Validate individual .erg files (format, headers, refs)")
	fmt.Fprintln(os.Stdout, "  check [DIR]               Corpus-level checks (duplicate IDs, cycles, refs)")
	fmt.Fprintln(os.Stdout, "  ready [DIR] [--json]      Show tickets ready for work")
	fmt.Fprintln(os.Stdout, "  next-id [DIR]             Print the next available ticket ID")
	fmt.Fprintln(os.Stdout, "  new TITLE [DIR]           Create a new ticket file atomically")
	fmt.Fprintln(os.Stdout, "  close ID REASON [DIR]     Close a ticket atomically")
	fmt.Fprintln(os.Stdout, "  log ID LINE [DIR]         Append a timestamped log entry to a ticket")
	fmt.Fprintln(os.Stdout, "  archive [ID...] [DIR]     Move closed tickets to tickets/closed/")
	fmt.Fprintln(os.Stdout, "  migrate [DIR]             Convert legacy Status: headers to Closed: form")
	fmt.Fprintln(os.Stdout, "  init [DIR]                Unpack AGENTS.md, spec-erg-v1.md, integration.md into tickets/")
	fmt.Fprintln(os.Stdout, "  version                   Print version, path, build date, and obsolescence info")
	fmt.Fprintln(os.Stdout, "  update                    Fetch and replace binary from origin")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	rest := os.Args[2:]

	// erg --help --all  OR  erg --help=all  → print all command help
	if cmd == "--help=all" || (cmd == "--help" || cmd == "-h") && len(rest) > 0 && rest[0] == "--all" {
		// "# erg manual" — all-lowercase is intentional; matches the kebab project name.
		fmt.Print("# erg manual\n\n")
		fmt.Print("Author: minh.ha-duong@cnrs.fr\n")
		genFrom := "Generated from: erg"
		if buildDate != "" {
			genFrom += " built " + buildDate
		}
		if vcsRevision != "" {
			genFrom += " rev " + vcsRevision
		}
		fmt.Print(genFrom + "\n\n")
		fmt.Print("`git-erg` is an agent-friendly local ticket system for development in disconnected\n")
		fmt.Print("environments. Tickets are plain-text files committed alongside source code.\n")
		fmt.Print("This manual describes all `erg` commands. For the ticket file format\n")
		fmt.Print("specification, see `tickets/spec-erg-v1.md`.\n")
		fmt.Print("\n")
		fmt.Print("**Store auto-discovery.** When no DIR is given, `erg` tries three candidates in\n")
		fmt.Print("order: (1) the directory containing the `erg` binary, (2) `tickets/` under the\n")
		fmt.Print("current working directory, (3) the current working directory itself. A directory\n")
		fmt.Print("qualifies as a ticket store if its basename is `tickets`, or if it contains at\n")
		fmt.Print("least one `.erg` file. The first qualifying candidate is used; if none qualify,\n")
		fmt.Print("`erg` exits with an error listing the directories it tried.\n")
		for _, c := range commandOrder {
			if text, ok := helpText[c]; ok {
				fmt.Print("\n" + text)
			}
		}
		os.Exit(0)
	}

	for _, arg := range rest {
		if arg == "--help" || arg == "-h" {
			if text, ok := helpText[cmd]; ok {
				fmt.Print(text)
			} else {
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
	case "archive":
		exitCode = cmdArchive(rest)
	case "migrate":
		exitCode = cmdMigrate(rest)
	case "init":
		exitCode = cmdInit(rest)
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
