# git-erg
An agent-friendly local ticket system for development in disconnected environments.

Author: Minh Ha-Duong <minh.ha-duong@cnrs.fr>
Last modified: 2026-05-04
Status: Working draft

- **File-based**: plain text `.erg` files committed to git
- **Offline-first**: no network, no API, no database
- **Zero runtime dependencies**: single Go binary, shell tests
- **Agent-friendly by design**: the spec is the interface, the binary is the guardrail and token-saving utilities

## Specification

- Normative specification:  `tickets/spec-erg-v1.md`.
- Reference implementation: `src/go/` (source), `tickets/erg` (bootstrap binary).
- Design rationale: `pep-erg-v1.md`.

## Install into a project

1. Create a `tickets/` dir at project's root
2. Install the `erg` binary into it (download is amd64 only, other arch need to rebuild from source)
3. Run `tickets/erg init` to unpack the files `AGENTS.md`, `spec-erg-v1.md` and `integration.md` in there.
4. Follow `tickets/integration.md` to a/ install the pre-commit validation hook and b/ tell your agent that tickets management instructions are in `tickets/AGENTS.md`.

## Quick start

```bash
# Create a ticket (or just write the file — agents do)
cat > tickets/0001-add-auth.erg <<'EOF'
%erg 0.1
Title: Add authentication flow
Created: 2026-03-27
Author: claude

--- log ---
2026-03-27T10:00Z user created

--- body ---
## Context
Need auth before shipping the API.
EOF

# List tickets
ls tickets

# Validate a single ticket
tickets/erg validate 01

# List ready tickets
tickets/erg ready
```

## Managing dependencies

Add `Blocked-by:` lines in the ticket header to encode dependencies.

Dependencies on ticket ID will be automatically cleared by `tickets/erg close ID REASON`, otherwise you have to find them and remove the line by editing the text file.

## Updating

`tickets/erg update` will lookup for a more recent build and replace the existing binary in-place.

## Binary policy

`tickets/erg` is a committed Linux x86-64 bootstrap binary for environments where Go may be unavailable (CI runners, agents). I look forward to working with macOS, ARM or Windows early adopters.

Source lives in `src/go/`. CI builds always compile from source and do not rely on the committed binary: all tests and development builds must use `build/erg`, rebuilt from source via `make build`.

The bootstrap binary is updated explicitly via `make update-bootstrap-binary`
(typically after changes to the Go code or when releasing) and must never be
modified by `make test`.

## License

MIT
