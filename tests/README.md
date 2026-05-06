# Test policy

## Testing layers

**Go unit tests** (`src/go/*_test.go`): pure-function correctness. If the
test can be written as `f(input) → output` without spawning a process or touching the
filesystem, it belongs here. Examples: `parseErg`, `validateErg`, `detectCycles`,
`slugify`, `appendLogLine`.

**Shell integration tests** (`tests/test_*.sh`): CLI black-box behavior. Exit codes,
error messages on stderr, file-system effects, argument parsing, cross-command
interactions. Do NOT re-test logic that is already covered by Go unit tests.

## Fixture strategy

Use two patterns, depending on what the test is asserting.

### 1. Static fixtures for ticket-format validation

Use static fixtures under:

```text
tests/fixtures/valid/
tests/fixtures/invalid/
````

Use these when the ticket file itself is the object under test, especially for:

* `erg validate` / `erg check`
* parser / schema rules
* known-good and known-bad `.erg` examples

Static fixtures should be small, named after the behavior they test, and not mutated by tests.

### 2. Ephemeral directories for CLI behavior

Use temporary directories created with `mktemp -d` when the test depends on filesystem state, filenames, relationships between files, or command-side effects.

Pattern:

```sh
TDIR=$(mktemp -d)
trap 'rm -rf "$TDIR"' EXIT
```

Use this for:

* `erg next-id`
* `erg ready`
* `erg close`
* `erg migrate`
* tests that create, rewrite, move, or close tickets
* duplicate-ID and dependency-cycle scenarios

## Rule of thumb

If the test checks **whether a ticket is valid**, use static fixtures.

If the test checks **what a command does**, use an ephemeral directory.

## Constraints

* POSIX `sh` only.
* One test file per command: `tests/test_<cmd>.sh`. Tightly coupled inverse commands (`init`/`uninstall`) may share a file.
* Tests must exit non-zero on failure.
* Prefer explicit `pass()` / `fail()` labels.
* Do not mutate files under `tests/fixtures/`.

## Binary defaults

Test scripts must default `ERG` to `build/erg`, not the bootstrap
path `tickets/erg`. Override via the `ERG_BIN` environment variable
when needed. The Makefile `_test-lint` target enforces this on every
`make test` run and will fail the build if any test reintroduces the
forbidden default.

## Parallel safety

Each test file is self-contained (private `mktemp -d`, no shared mutable state), so suites are safe to run concurrently. Use `make -j test` to run all suites in parallel, or `make test-<cmd>` to run a single suite.

## Shell hygiene

All test files use `set -eu`. To add a variable that may be unset,
declare it with a default before any `trap` that references it.

