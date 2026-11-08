# Memory sources - v8 adoption inventory (ticket 0305)

Inventoried 2026-11-08 at git-erg `origin/main` 8c371d17. Audience of this
repository: public GitHub repository `MinhHaDuong/git-erg` (`gh repo view`,
2026-11-08). Counts were taken with shell commands that day; unknowns are
stated as unknown. Native notes are not quoted here: this repository is
public and the notes were never reviewed for publication.

## Sources, consumers and dispositions

| Source | Writer / consumer | Disposition |
|---|---|---|
| `~/.claude/projects/-home-haduong-CNRS-code-git-erg/memory/` (66 notes: by filename prefix 56 feedback, 5 project, 4 reference, 1 other, 52/5/3/6 by frontmatter type; plus a 70-line `MEMORY.md`, modified 2026-11-08) | Claude Code native auto-memory; the index is written by Claude Code sessions | Kept unchanged and still active. Not migrated, not quoted: the author triages it through smelt. Its index was loaded in a session started in a worktree (this PR's load probe, below). |
| `~/.agents/projects/-home-haduong-CNRS-code-git-erg/memory/` (65 notes plus `MEMORY.md`) | Export tracked in the maintainer's harness repository (last commit 8dee0831) | Differs from the native store by one newer native note and the index (`diff -rq`). Left untouched. |
| `~/.claude/projects/-home-haduong-CNRS-code-git-erg--claude-worktrees-raid-255-256-278-279` (15 files, no `memory/` directory) | Claude Code project directory of an old raid worktree | Holds no memory. Not read. Observed in one session (this PR's load probe, Claude Code 2.1.319, 2026-11-08): a session started in a worktree files its transcript under its own project directory (`...--claude-worktrees-t0305-1041531-29961`, no `memory/`) but loads the repository root's native index (`AutoMem` attachment naming `-home-haduong-CNRS-code-git-erg/memory/MEMORY.md`). How Claude Code chooses is not established. |
| Session transcripts (5 top-level `*.jsonl` in the native project directory, plus subdirectories) | Claude Code session history | Not memory sources; not read, not imported. |
| [journal/2026/](journal/2026/) (2 entries, 2026-11-04 and 2026-11-08) | Roar captures, already v8-shaped (commit 8510a9b) | Kept in place, indexed. |
| [STATE.md](../STATE.md), [AGENTS.md](../AGENTS.md) | Canonical status and policy | Linked as references, not copied. They govern over memory. |
| `tickets/` (2 open and 287 closed `.erg` files) and `git log` | Ticket store and history | Their own record. Not memory sources; not copied. |
| `~/.codex` | Codex | No `memories/` directory. `memories_1.sqlite` (2026-10-05) holds 0 rows in `stage1_outputs` and `jobs`; 11 Codex session rollouts under `~/.codex/sessions/2026/09/` carry a `cwd` in this checkout (grep of the `"cwd"` field), none since; none was read. |
| `~/.pi/agent` | Pi | No session directory keyed to git-erg (`ls ~/.pi/agent/sessions`). No memory store found. |

## Audience

No content was moved: the index and this inventory are new text, and the two
journal entries already lived in this public repository. No native note text
was imported, so no source needed encryption and no `.age` entry exists.

## Runtime limits

- The v8 index is reached only by agents that load this repository's
  AGENTS.md; no hook injects it. The section sits before the final
  `git-erg local tickets: see tickets/AGENTS.md` line, erg's pointer text
  (`agentsBody`, `src/go/install.go`) written here without the managed-block
  markers, so `erg install --inject-agents` does not manage it. The shipped
  `tickets/AGENTS.md` is untouched.
- Claude Code native auto-memory remains a separate writer of a separate
  index. This adoption adds no writer: `memory/MEMORY.md` changes only
  through reviewed roar or smelt PRs.
- No timer, TTL or automatic promotion is installed. No empty directories:
  `topics/` and `smelts/` appear when the first page exists.
- `MEMORY.md` is capped at 40 lines.

## Later review

About four weeks after merge, run the harness census
(`scripts/census/repo-memory-reads.py`) for Claude Code and Codex on this
project and report the read counts to the author.

## Rollback

Revert the adoption merge commit, or remove the `## Project memory` section
of AGENTS.md and `memory/MEMORY.md` and `memory/SOURCES.md`. The journal
stays. Native stores and the harness export were not modified.
