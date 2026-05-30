# UX Dry-Run Process

Repeatable AI-assisted QA for "easy to try and learn." Agents run this;
humans judge the output. Cadence: once per release, or after any change to
install steps, help text, or init assets.

## When to run

- A new binary is cut (release day).
- Any change to: `README.md` install section, `src/go/assets/` (init files),
  help text (`helptext.go`), or error messages.

## Two paths

Run both paths each time. They test different audiences and different gaps.

### Path A — human dev

Persona: a developer who found git-erg on GitHub and wants to try it in a
project. Has shell access, no prior knowledge of git-erg.

Cold prompt for the agent:
> You are a human developer who has heard about git-erg, a local file-based
> ticket management system. Your project is at `<WORKDIR>` (a fresh git repo).
> You know the GitHub repo is MinhHaDuong/git-erg and nothing else.
> Task: install git-erg into your project and create your first ticket.
> Document every step and every friction point (see output format below).

Setup: `mkdir <WORKDIR> && cd <WORKDIR> && git init && echo "# Project" > README.md && git add . && git commit -m init`

### Path B — agent-delegated

Persona: an AI coding agent whose user asked it to set up tickets for a project.
No prior knowledge of git-erg.

Cold prompt for the agent:
> You are an AI agent assistant helping a developer with their project at
> `<WORKDIR>` (a fresh git repo with only a README.md).
> Task: "Install the local tickets management system MinhHaDuong/git-erg,
> create an example ticket."
> That is the only instruction. Document every step and every friction point
> (see output format below).

Setup: same as Path A.

## Output format

Tell the dry-run agent to produce:

```
## Transcript
Numbered steps: what you did, what you ran, what output you saw (quote actual output).

## Friction log
Each friction point:
- **What:** what was unclear, missing, or confusing
- **Where:** which file / section / command
- **Severity:** Low / Medium / High
  (High = would cause a real user to fail or give up)
- **Suggestion:** what would have fixed it

## Summary
3–4 sentences: overall experience. Would a real user get through this?
```

## After the run

For each friction item:
1. Assess severity — High and Medium items get tickets immediately; Low items
   are batched unless they cluster around a theme.
2. File a ticket with `erg new "<concise title>"` and paste the relevant
   friction log entry + transcript excerpt into the body as evidence.

## What CI checks independently

These run on every push and do not need to be re-run manually:

- **Install sequence**: README curl → `erg init` → `erg new` → `erg validate`
  on a clean image. (ticket 0196)
- **Time-to-first-ticket**: wall-clock budget for the install sequence.
  (ticket 0196)
- **Help-completeness**: every command has a non-empty `--help` entry.
  (ticket 0068, confirmed live)

The dry-run covers judgment; CI covers regressions. Both are needed.
