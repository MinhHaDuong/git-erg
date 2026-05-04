# git-erg
An agent-friendly local ticket system for development in disconnected environment.

Author: Minh Ha-Duong <minh.ha-duong@cnrs.fr>
Last modified: 2026-05-04
Status: Working draft

- **File-based**: plain text `.erg` files committed to git
- **Offline-first**: no network, no API, no database
- **Zero runtime dependencies**: single Go binary, shell tests
- **Agent-friendly by design**: the spec is the interface, the binary is the guardrail and token-saving utilities

## Specification

Normative specification:  `tickets/spec-erg-v1.md`.
Reference implementation: `tickets/tools/go/erg`.
Design rationale: `pep-erg-v1.md`.

## Install into a project

```bash
make install DEST=/path/to/your/project
```

This builds the binary, copies source and rules into the target project,
creates `tickets/`, appends the pre-commit hook (composing with any existing
hook), adds the binary to `.gitignore`, and skips sample tickets so you
start fresh at `0001`.

You can also run the script directly: `./bin/install.sh /path/to/project`

## Binary policy

`tickets/tools/go/erg` is a committed Linux x86-64 bootstrap binary for environments where Go may be unavailable (CI runners, agents). It will **not** run on macOS or ARM.

Source is authoritative. CI builds always compile from source and do not rely on the committed binary: all tests and development builds must use `build/erg`, rebuilt from source via `make build`.

The bootstrap binary is updated explicitly via `make update-bootstrap-binary`
(typically after changes to the Go code or when releasing) and must never be
modified by `make test`.

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

## Inspect the dependency DAG

Edges live directly in `Blocked-by:` headers, so any tool that reads
text can render the graph. Headers are preamble-only, so awk through
the first `--- log ---` to skip body matches. The recipes below match
local refs (4-digit IDs) only; drop the `[0-9]{4}$` anchor to also
include `gh#N` and `gh:owner/repo#N` cross-repo refs.

```bash
# Adjacency list: blocker → blocked
awk '/^--- log ---/{nextfile} /^Blocked-by:[[:space:]]+[0-9]{4}$/{print FILENAME, $2}' tickets/*.erg \
  | sed -E 's|tickets/([0-9]{4})[^ ]*|\1|' \
  | awk '{print $2" -> "$1}'

# Topological order (requires GNU coreutils `tsort`)
awk '/^--- log ---/{nextfile} /^Blocked-by:[[:space:]]+[0-9]{4}$/{print FILENAME, $2}' tickets/*.erg \
  | sed -E 's|tickets/([0-9]{4})[^ ]*|\1|' \
  | awk '{print $2, $1}' | tsort
```
## License

MIT
