// Command erg validates and operates on %erg v1 ticket files. It depends only
// on the Go standard library.
//
// Usage:
//
//	erg validate <file ...>
//	erg check    [dir]
//	erg ready    [dir] [--json]
//	erg next-id  [dir]
//	erg new      <title> [dir]
//	erg close    <id|file> <reason> [dir]
//	erg log      <id> <line> [dir]
//	erg archive  [id...] [dir]
//	erg migrate  [dir]
//	erg init     [dir]
//	erg uninstall [dir]
//	erg version
//	erg update
package main

import (
	"fmt"
	"os"
)

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: erg COMMAND [args...]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  validate FILES...         Validate individual .erg files (format, headers, refs)")
	fmt.Fprintln(os.Stderr, "  check [DIR]               Corpus-level checks (duplicate IDs, cycles, refs)")
	fmt.Fprintln(os.Stderr, "  ready [DIR] [--json]      Show tickets ready for work")
	fmt.Fprintln(os.Stderr, "  next-id [DIR]             Print the next available ticket ID")
	fmt.Fprintln(os.Stderr, "  new TITLE [DIR]           Create a new ticket file atomically")
	fmt.Fprintln(os.Stderr, "  close ID REASON [DIR]     Close a ticket atomically")
	fmt.Fprintln(os.Stderr, "  log ID LINE [DIR]         Append a timestamped log entry to a ticket")
	fmt.Fprintln(os.Stderr, "  archive [ID...] [DIR]     Move closed tickets to tickets/closed/")
	fmt.Fprintln(os.Stderr, "  migrate [DIR]             Convert legacy Status: headers to Closed: form")
	fmt.Fprintln(os.Stderr, "  init [DIR]                Bootstrap tickets/ support files from embedded assets")
	fmt.Fprintln(os.Stderr, "  uninstall [DIR]           Remove files/fragments managed by `erg init`")
	fmt.Fprintln(os.Stderr, "  version                   Print version, path, build date, and obsolescence info")
	fmt.Fprintln(os.Stderr, "  update                    Fetch and replace binary from origin")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	rest := os.Args[2:]

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
	case "uninstall":
		exitCode = cmdUninstall(rest)
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
