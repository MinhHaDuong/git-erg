# Archived logging raid, 2026-11-26

Ticket 0314 shipped through PR #410. A unique archived ticket now accepts log entries with a warning naming its path. Duplicate IDs within or across the active store and archive are refused without writes. Regression tests reproduced the prior failure before the implementation, then passed; the full project gate passed on the closed-ticket head.

The full suite exposed a security-test fixture that recreated an ID after close had archived it. The controls now use distinct IDs; the duplicate-ID behavior is checked separately by Go tests. The audit of shared resolver callers is recorded in the ticket log.

The first delegated gaze setup (codex, OpenAI/gpt-6.1-sol, low) ended before review: automatic approval review could not link PR #410 to the raid's ticket request. No review worktree or notes were created by that attempt. The lead read the PR back, confirmed its ticket claim and head, and supplied that evidence; setup was then approved. The subsequent correctness and data-preservation seats returned clear. Their identities and evidence are captured in the PR attribution record.

At session start local main held unpublished commits f994087 and a604681 from an earlier attempt. The sync helper left that divergent local history untouched. Work, review, gating, and integration used isolated worktrees based on origin/main. The task merge was read back as MERGED at head 90e313465acc7341a281a788094e7ebc3f3e15fa.

Roar checks: the updated integration tip passed make check. The close-claim sweep found no mismatch. The report-only temporary scan was partial (unreadable history/entries), so no cleanup conclusion was drawn and no host temporary files were removed. Attribution backfill appended nothing and left nine historical records unresolved; prior review revisions were unavailable, and the current-record probe reported an unresolvable revision/path. No historical attribution was changed.
