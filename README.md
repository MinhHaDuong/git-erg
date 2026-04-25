# git-erg

Agent-native local ticket system for git worktree coordination.

- **File-based**: plain text `.erg` files committed to git
- **Offline-first**: no network, no API, no database
- **Zero dependencies**: single Go binary, shell tests
- **Agent-friendly by design**: the spec is the interface, the binary is the guardrail

## Install into a project

```bash
make install DEST=/path/to/your/project
```

This builds the binary, copies source and rules into the target project,
creates `tickets/` and `tickets/archive/`, appends the pre-commit hook
(composing with any existing hook), adds the binary to `.gitignore`,
and skips sample tickets so you start fresh at `0001`.

You can also run the script directly: `./bin/install.sh /path/to/project`

## Platform compatibility

The committed `tickets/tools/go/erg` binary is a statically-linked ELF 64-bit
executable for Linux x86-64. It runs on any Linux x86-64 system (developer
machines, CI runners, Claude Code web containers) with no dynamic dependencies.
It will **not** run on macOS or ARM. Users on those platforms must build from
source: `cd tickets/tools/go && go build -o erg .`

## Quick start

```bash
# Build the validator
make build

# Create a ticket (or just write the file — agents do)
cat > tickets/0001-add-auth.erg <<'EOF'
%erg v1
Title: Add authentication flow
Status: open
Created: 2026-03-27
Author: claude

--- log ---
2026-03-27T10:00Z claude created

--- body ---
## Context
Need auth before shipping the API.
EOF

# Validate
erg validate tickets/

# List ready tickets
erg ready tickets/
```

## Format

See [rules/tickets.md](rules/tickets.md) for the complete `%erg v1` specification.

## What to gitignore

The compiled binary `tickets/tools/go/erg` should be gitignored — commit the
source, not the binary. The install script handles this automatically. If
installing manually, add this line to your `.gitignore`:

```
tickets/tools/go/erg
```

## For Claude Code users

The install script sets up skills and rules automatically. To do it manually:

Copy `claude/` into your project's `.claude/` directory to get skills:
`/ticket-new`, `/ticket-claim`, `/ticket-close`, `/ticket-release`, `/ticket-ready`

Copy `rules/tickets.md` into `.claude/rules/`.

## For other agents

Read `rules/tickets.md`. That's the complete spec. Write `.erg` files directly.
The Go binary validates on commit — your agent doesn't need it to operate.

## Design

See [docs/pep-erg-v1.md](docs/pep-erg-v1.md) for design rationale, alternatives
explored, and architectural decisions.

## License

MIT
