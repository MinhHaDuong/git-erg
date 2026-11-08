# Dead core.hooksPath and a post-merge amend, found by a yawn

On 2026-11-08 a yawn and molt sweep on padme found that the primary
checkout's `.git/config` set `core.hooksPath` to
`/home/haduong/git-erg/.git/hooks`. That path no longer existed after the
repository moved to `~/CNRS/code/git-erg`. Git runs no hooks under a
missing hooks path, so the erg pre-commit hook in `.git/hooks/pre-commit`
did not run on local commits for an unknown period. The yawn probe's hook
check reported `in_worktree: false` and no stale files. It does not test
whether the configured path exists. The sweep unset the setting, and the
next commit (5794f2a, PR #382) printed `ERG VALIDATION: PASS`. CI had
validated every push throughout.

The same sweep found branch `t0285-scope-pathisclosed-to-store-root`
holding a097ba9, an amend made at 14:26 +0200 on 2026-09-16, seven minutes
after PR #349 merged its predecessor. Each of its additions had been dropped
in the merge arbitration (ticket 0294), folded into 4e1bf02 (the spec rule
of ticket 0291), or reworded and landed by #355. The merge probe returned
exit 1 because the amend is neither an ancestor of main nor tied to a PR.
With the author's approval the branch was deleted locally and on origin.

The sweep also committed ticket 0305 (memory v8 rollout), which codex
created on 2026-10-05 and which had stayed untracked in the primary
checkout since then.
