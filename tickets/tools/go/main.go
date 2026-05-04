// Command erg validates and operates on %erg v1 ticket files. It depends only
// on the Go standard library.
//
// Usage:
//
//	erg validate [dir|file ...]
//	erg ready    [dir] [--json]
//	erg next-id  [dir]
//	erg close    <id|file> <reason> [dir]
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
	fmt.Fprintln(os.Stderr, "Usage: erg <command> [args...]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  validate [dir|files...]   Validate erg v1 ticket files")
	fmt.Fprintln(os.Stderr, "  ready [dir] [--json]      Show tickets ready for work")
	fmt.Fprintln(os.Stderr, "  next-id [dir]             Print the next available ticket ID")
	fmt.Fprintln(os.Stderr, "  close <id|file> <reason> [dir]  Close a ticket atomically")
	fmt.Fprintln(os.Stderr, "  migrate [dir]             Convert legacy Status: headers to Closed: form")
	fmt.Fprintln(os.Stderr, "  init [dir]                Bootstrap tickets/ support files from embedded assets")
	fmt.Fprintln(os.Stderr, "  uninstall [dir]           Remove files/fragments managed by `erg init`")
	fmt.Fprintln(os.Stderr, "  version                   Print sha256 of this binary")
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
	case "ready":
		exitCode = cmdReady(rest)
	case "next-id":
		exitCode = cmdNextID(rest)
	case "close":
		exitCode = cmdClose(rest)
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
