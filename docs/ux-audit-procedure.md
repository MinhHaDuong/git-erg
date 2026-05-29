# UX audit — AI persona dry-run procedure

Repeatable procedure for AI-assisted UX evaluation of git-erg.
Agents do the legwork; humans judge severity and decide what ships.

## Personas

Each run evaluates git-erg from four perspectives:

| Persona | Profile | Focus |
|---------|---------|-------|
| **Beginner** | First time seeing erg, no ticket-system experience | Can they create a ticket from the README alone? |
| **Expert** | Experienced dev evaluating erg for their project | Is the value prop clear in 2 minutes? Install friction? |
| **Agent** | AI coding agent (Claude, Cursor, Codex) in a sandbox | Can it discover and use tickets with Read/Edit only? |
| **Developer** | Contributor to git-erg itself | Are test/build/debug paths discoverable? |

## First-five-minutes script

For each persona, an agent follows these steps in a fresh `mktemp -d`
directory (simulating a cold clone) and reports friction at each step.

### Steps

1. **Discover**: Read only `README.md`. Can the persona figure out what
   erg is and how to start? Note any confusion, missing info, or
   jargon without definition.

2. **Install (POSIX path)**: Follow the "Start in 10 seconds" section.
   Create a ticket by hand with `mkdir` + `cat`. Verify it works with
   `grep -L '^Closed:'`. Note friction: unclear syntax, missing
   examples, format mistakes.

3. **Install (binary path)**: Copy the `erg` binary, run `erg init`.
   Read the unpacked files. Does `integration.md` explain the next
   step? Does `AGENTS.md` orient the persona?

4. **Operate**: Create a ticket (`erg new`), close it (`erg close`),
   list open work (`erg ready`). Try `erg` with no args, `erg help`,
   `erg --help --all`. Is the command discoverability sufficient?

5. **Recover**: Induce a common error — malformed ticket, close a
   non-existent ID, run an unknown command. Does the error message
   teach what went wrong and suggest the fix?

6. **Report**: Compile a friction log:
   - Step where friction occurred
   - What the persona tried
   - What happened vs. what was expected
   - Proposed severity: **P0** (blocks adoption), **P1** (significant
     annoyance), **P2** (minor polish), **P3** (nice-to-have)

## Running the procedure

```bash
# Mechanical smoke (CI, every push):
make test-ux

# Full AI persona dry-run (per release or UX-surface change):
# Launch one agent per persona, each following the steps above
# in a clean temp directory. Collect transcripts.
```

The mechanical smoke (`tests/test_ux.sh`) runs in CI on every push and
catches regressions in the install paths, time budget, orientation, and
error messages. The full persona dry-run is heavier — run it when
changing README, help text, init assets, or adding/removing commands.

## Output

Each run produces a findings report with:
- Transcript per persona (the agent's session log)
- Friction items with draft severity (agent-proposed, human-confirmed)
- Each confirmed item becomes a ticket with the transcript attached

## Cadence

- **Every push**: `make test-ux` (mechanical smoke, in CI)
- **Per UX-surface change**: full persona dry-run when modifying
  README.md, help text (`helptext.go`, per-command help), init assets,
  or adding/removing a subcommand
- **Quarterly**: full four-persona run as a standing audit, even
  without specific changes, to catch drift
