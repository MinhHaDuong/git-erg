package main

import (
	"fmt"
	"os"
	"strings"
)

const summaryInstall = "Wire up git hooks and agent instructions (opt-in)"

const helpInstall = `## erg install [DIR] [--hooks] [--inject-agents]

Wire up integration hooks and agent instructions for a project that already
has a ticket store (created by erg init).

By default -- with no flags -- install does nothing outside tickets/. Each
piece of wiring requires an explicit opt-in flag:

  --hooks            Install a pre-commit hook that runs erg validate and erg
                     check on every commit. The hook also rejects commits that
                     modify the traveling binary (tickets/erg) outside main.

  --inject-agents    Add a one-line pointer to tickets/AGENTS.md in the
                     project-root AGENTS.md. If the root AGENTS.md does not
                     exist, the flag is refused unless combined with an
                     explicit creation flag (see ticket 0208 for the full
                     consent flow).

Both flags default to off. erg install never overwrites an existing hook or
agent file -- it appends inside a managed block delimited by sentinels.

Requires tickets/erg (the binary) to already exist in the project, same as
erg init.
`

func cmdInstall(args []string) int {
	var positional []string
	hooks := false
	injectAgents := false
	for _, a := range args {
		switch a {
		case "--hooks":
			hooks = true
		case "--inject-agents":
			injectAgents = true
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(os.Stderr, "install: unknown flag %q\nUsage: erg install [DIR] [--hooks] [--inject-agents]\n", a)
				return 1
			}
			positional = append(positional, a)
		}
	}

	root := "."
	if len(positional) > 0 {
		root = positional[0]
	}

	_ = hooks
	_ = injectAgents

	fmt.Fprintf(os.Stderr, "install: stub -- wiring not yet implemented (dir=%s, hooks=%v, inject-agents=%v)\n", root, hooks, injectAgents)
	return 0
}
