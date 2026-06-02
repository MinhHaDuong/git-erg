package main

import (
	"fmt"
	"os"
	"strings"
)

const summarySpec = "Print the %erg 0.1 format specification"

const helpSpec = `## erg spec

Print the embedded %erg 0.1 format specification to stdout.

This is the same content that older versions of erg deposited as
tickets/spec-erg-v1.md during init. It is now served on demand to keep
the tickets/ directory uncluttered.
`

func cmdSpec(args []string) int {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			fmt.Fprintf(os.Stderr, "spec: unknown flag %q\nUsage: erg spec\n", a)
			return 1
		}
	}
	content, ok := bootstrapAsset("tickets/spec-erg-v1.md")
	if !ok {
		fmt.Fprintln(os.Stderr, "spec: embedded asset not found")
		return 1
	}
	fmt.Print(content)
	return 0
}
