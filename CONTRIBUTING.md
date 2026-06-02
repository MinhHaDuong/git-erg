# Contributing to git-erg

Thanks for helping out. git-erg is a small, single-static-binary Go tool
plus a POSIX-first text format. This guide covers the contributor loop;
agent operating policy lives in `AGENTS.md`, and the test policy lives in
`tests/README.md`.

## Prerequisites

- **Go 1.21+** (see `src/go/go.mod`). Needed only to build/test the binary;
  the POSIX path needs nothing.
- A POSIX `sh` for the integration tests.

## Build and test

```bash
make build          # build the binary to build/erg
make check          # pre-PR gate: full test suite + ticket-corpus validation
make test           # Go unit tests + all shell integration suites
make test-<suite>   # run one shell suite, e.g. make test-init
make unit-test      # Go unit tests with a coverage report
make validate       # validate the tickets/ corpus
```

`make check` (an alias for `make test` + `make validate`) is the gate: it must
exit 0 before you open a merge request.
Tests always run against `build/erg` (rebuilt from source), never the
committed `tickets/erg` bootstrap binary. CI likewise always compiles from
source and never relies on the committed binary.

The committed `tickets/erg` bootstrap binary is refreshed explicitly via
`make update-bootstrap-binary` (typically after Go-code changes or when
releasing); `make test` must never modify it.

## Branches and merge requests

- Work on a branch; `main` is for merges only.
- One change per commit; the message explains *why this change and not
  another*.
- Open one merge request per ticket. Put a `**Ticket:**
  tickets/NNNN-...erg` line in the body so the ticket auto-closes on merge.
- Keep the diff inside the change-scope allowlist in `AGENTS.md` section 3.

## Adding a subcommand

A subcommand touches five places. Two automated tests
(`TestDispatchRegistrySync` in `src/go/erg_test.go` and the Makefile
`_test-lint` target) catch the easy-to-forget ones, so follow all five:

1. **Command file**: create `src/go/<cmd>.go` with `summary<Cmd>`,
   `help<Cmd>`, and `cmd<Cmd>(args []string) int`. Use an existing
   small command (`src/go/init.go`) as the template.
2. **Registry**: add a `commandEntry` to the `commands` slice in
   `src/go/helptext.go` (this drives `--help` and `--help --all`).
3. **Dispatch**: add a `case "<cmd>":` to the switch in `src/go/main.go`
   calling `cmd<Cmd>(rest)`.
   (Steps 2 and 3 must stay in sync; `TestDispatchRegistrySync`
   fails if one is missing.)
4. **Test**: create `tests/test_<cmd>.sh` (see `tests/README.md` for the
   fixture conventions).
5. **Test registration**: add `<cmd>` to `TEST_SUITES` in the `Makefile`.
   (The `_test-lint` target fails if a `tests/test_*.sh` file has no
   matching `TEST_SUITES` entry.)

Then `make test`; both guard tests run as part of it.

## Where things live

- Source of truth: `src/go/` (the binary is a built cache, see README
  "Binary policy").
- Format spec: `erg spec` (or `tickets/spec-erg-v1.md`). Design rationale: `pep-erg-v1.md`.
- Test policy and fixture strategy: `tests/README.md`.

## Committed helper scripts (the forge layer)

`erg` core is offline and forge-blind. Forge integration lives in committed
helper scripts named `erg-<forge>` (e.g. `tickets/erg-github`) -- the hyphen is
the link convention, like git's `git-foo` helpers; they are run directly, not
dispatched by the binary. Such a script must be: POSIX `sh` (`#!/bin/sh`,
`set -eu`, no bashisms -- `[[`, `local`, arrays, `pipefail`), pure ASCII,
committed executable (mode 100755), CWD-independent (resolve the repo via
`git rev-parse --show-toplevel`), and covered by a `tests/test_<name>.sh` suite
registered in `TEST_SUITES`. `erg` core must never depend on one. The
pre-commit binary-reject rule anchors `^tickets/erg$` exactly, so a text helper
like `tickets/erg-github` commits freely on any branch.
