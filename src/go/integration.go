package main

import (
	"fmt"
	"os"
	"strings"
)

const summaryIntegration = "Print the setup guide for hooks and CI integration"

const helpIntegration = `## erg integration

Print the embedded setup guide for the pre-commit hook and CI integration
to stdout.

This is the same content that older versions of erg deposited as
tickets/integration.md during init. It is now served on demand to keep
the tickets/ directory uncluttered.
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
