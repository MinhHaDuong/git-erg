# git-erg — the pitch

Author: Minh Ha-Duong <minh.ha-duong@cnrs.fr>
Last modified: 2026-05-29

A reusable source for the README hero, a landing page, a conference slide, or
a social post. The job of this doc: make adopting git-erg a *sexy no-brainer*
— near-zero cost, near-zero risk, and obviously want-able.

## One-liner

> **An issue tracker that's just files in your repo — offline, agent-native,
> zero-install.**

Variants by audience:

- *For developers:* "Your backlog is `tickets/*.erg` in git. No server, no
  account, no API. `grep` is your query language."
- *For the AI-agent crowd:* "An issue tracker your coding agent can use
  offline — no API keys, no MCP wiring. It picks its own work from plain
  files."
- *For the offline/airgapped crowd:* "Track work where there is no network —
  on a plane, in CI, in a locked-down sandbox. The whole tracker is text in
  git."

## The 10-second demo

```bash
mkdir tickets
cat > tickets/0001-add-auth.erg <<'EOF'
%erg 0.1
Title: Add authentication flow
Created: 2026-05-29
Author: me

--- log ---
2026-05-29T10:00Z me created

--- body ---
Need auth before shipping the API.
EOF

ls tickets/                          # the backlog
grep -L '^Closed:' tickets/*.erg     # the open list
```

No install ran. That is the demo: a working ticket system built from a
directory and a text file, versioned in git, offline, deletable without trace.

The "with binary" encore (optional, for the asciinema):

```bash
erg ready          # open, unblocked tickets — ranked, human-readable
erg list --json    # same data, machine-readable, for your agent or CI
erg close 0001 "done in PR #42"   # atomic, validated, never corrupts the file
```

## Three reasons it's a no-brainer (cost & risk → 0)

1. **Nothing to install to start.** The plain `.erg` file is the interface;
   `cat`/`grep`/`find`/`vim` and an agent's `Read`/`Edit` are the client. The
   binary is an optional speed/validation upgrade, never a prerequisite.
2. **Nothing to lock into.** It is text in git — no database to migrate, no
   export step, no vendor, no SaaS bill. `rm -rf tickets/` and it never
   existed. Reversibility kills the hesitation that stalls adoption.
3. **Nothing new to wire up.** No network, no API keys, no MCP config, no CI
   plugin. It runs anywhere a shell runs and works with the agent you already
   use.

## Three reasons it's sexy (want-able)

1. **Agent-native, the 2026 way.** Local-first *and* agent-first: your coding
   agent reads, writes, and closes tickets with the filesystem tools it
   already has — no integration to build, works offline. That is the
   differentiator versus GitHub Issues / Jira / Linear.
2. **Taste in the details.** `erg` is the physics unit of work; `%erg 0.1` is a
   real magic-line version marker (à la `%PDF-1.7`); a closed header schema
   means there is exactly one right way to write a ticket — agents love rigid
   schemas, humans love no bikeshedding.
3. **Honest engineering.** A static, zero-dependency binary whose full source
   (`src/go/`) travels with it — nothing to trust on faith. The roadmap closes
   the loop (ticket 0151): bit-for-bit reproducible builds and signed release
   tags so the committed binary can be verified offline.

## Who it's for

- Solo devs and small teams who want a backlog without standing up infra.
- Agent-driven development where the agent should pick its own work without a
  network round-trip.
- Disconnected / airgapped / latency-sensitive environments.
- Monorepos and polyrepos that want tickets to live and version *with the
  code* they describe.

## What it is not

Not a forge. Coordination across many agents/worktrees is a forge's job
(GitHub Issues, GitLab); git-erg complements it and never mirrors its state.
It is local-first ticket *content*, not a synchronized multi-writer queue.

## The honest catch (and why it's fine)

The thing that makes it spread — a binary that travels in thousands of repos
and runs in their hooks and CI — is also what makes it a supply-chain target.
git-erg owns this rather than hiding it. `erg version` already prints the
binary's full SHA-256 (recomputable with stock tools); the planned controls
(ticket 0151) close the rest — bit-for-bit reproducible builds (rebuild from
`src/go/` and byte-compare) and signed release tags. The same property that
makes adoption a no-brainer is why those controls are non-negotiable. See
`docs/audit-infrastructure-class.md`.
