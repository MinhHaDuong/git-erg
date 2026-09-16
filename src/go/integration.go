package main

import (
	"fmt"
	"os"
	"strings"
)

const summaryIntegration = "Print the setup guide and the long-form ticket conventions"

const helpIntegration = `## erg integration

Print the embedded long-form guide to stdout: the pre-commit hook and CI
integration, then the working conventions that are too long to keep in
tickets/AGENTS.md -- optimistic ID allocation and collision recovery, scanning
open PRs for a colliding ID, checking the merged default branch after a ticket
PR lands, decision records versus artifacts, the handoff-document section
template, and where project-specific ticket lore belongs.

tickets/AGENTS.md is resident context, re-read at the start of every agent
session, so it stays short and points here. This is the same content that older
versions of erg deposited as tickets/integration.md during init; it is now
served on demand to keep the tickets/ directory uncluttered.
`

func cmdIntegration(args []string) int {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			fmt.Fprintf(os.Stderr, "integration: unknown flag %q\nUsage: erg integration\n", a)
			return 1
		}
	}
	content, ok := bootstrapAsset("tickets/integration.md")
	if !ok {
		fmt.Fprintln(os.Stderr, "integration: embedded asset not found")
		return 1
	}
	fmt.Print(content)
	return 0
}
