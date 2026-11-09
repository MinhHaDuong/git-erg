# ID claims made required (0307), and a hook cut after a structural review

On 2026-11-09 the author reviewed the opt-in claims shipped by 0306 (#386) and ruled that claims are
not optional. Raid a59e5b worked ticket 0307 and landed PR #393 as 9cb6034. The same session landed
#389 (the deferred "pre-create next-ticket" idea dropped), #390 (picking a number by hand under
claims), #391 (claims propagated to CHANGELOG, PEP, threat model) and #392 (the author's PEP revision
and ticket 0307).

Observations:

- Author decisions, in sequence: "the PEP currently reads claims optional; it is not"; then, asked
  yes/no, "fail, no legacy cruft, repair code is the migration code". The shipped design had made
  `.claims/` an on/off switch so a binary update would not break existing stores; the author treated
  that switch as cruft. `erg check` now fails a store without `.claims/`, `erg new` refuses there, and
  `erg migrate --claims` prechecks every blocker and writes nothing until the store is ready.
- Second opinions on the "pre-create 00XX-is-next-ticket.erg" idea: an Opus analysis and a gpt-6.1-sol
  review, briefed without the first view, both concluded drop (~90%). The second found what the first
  missed: a placeholder file would make `erg new` skip its own number, since the allocator counts
  filenames.
- A Sonnet review of the by-hand bullet (#390) found the first draft's "fetch first" wrong (a fetch
  does not change the working tree the `ls` reads) and the claim recipe missing.
- The cross-family Blind-Spot seat (gpt-6.1-sol) on 0307 found that random nonces at migration make
  two clones that migrate the same base conflict on every claim. Migration claims now derive their
  nonce from number, slug and Created; `erg new` keeps a random one.
- Ticket 0308 landed on main while #393 was in review, unclaimed; the coder claimed it on the branch
  after rebasing. The window exists until a store's `.claims/` reaches its main.
- After a long coding stream (about 56 minutes, five gaze rounds) the author asked whether the result
  was a spaghetti monster. Measured against the merge base: production Go +657/-259, claims.go
  349 -> 627 lines. A gpt-6.1-sol structural review returned ACCEPTABLE-WITH-CUTS; the pre-merge-commit
  hook had grown to about 60 lines of shell choosing which binary to trust. Proposed instead: the merge
  hook runs no binary, only an index check (unclaimed tickets and shared numbers) in sh and awk. A
  Fable advisor agreed (ADOPT-WITH-CHANGE), naming the loss: a clean merge leaving a dangling
  `Blocked-by:` or a cycle, caught at the next ticket commit and in CI. After the cuts: merge hook 60 ->
  31 lines, production Go -26, tests -43. Gaze reviewed the new head with no findings; the gate
  approved round 1 (2m03s); merge 13s.
- Misfires: `erg new tickets "Title"` (arguments reversed) silently created a directory named after the
  title at the repo root holding `0001-tickets.erg`; it was removed by hand. Gaze's round-1 closing
  script failed with "has no request" and recorded nothing; later rounds recorded normally. The
  gate's cleanup removed its worktree past a pre-removal refusal over `build/` output, as on
  2026-11-08. The raid did not record writer attribution lines before landing (raid_land: "not
  derivable"). The PR body's short close claim `**Ticket:** 0307` was refused by raid_land and
  rewritten to the canonical path.

Outcome: claims are required in code, docs and PEP; git-erg's own store is migrated (291 claims).
No adopter store is migrated yet; the upgrade recipe is `erg sync`, `erg init`,
`erg migrate --claims`, commit, then `erg install --hooks`.

References: PRs #389-#393; tickets/closed/0307-require-id-claims-in-every-store-prechec.erg;
memory/evidence/attribution/2026-11-09-review-attribution-pr393.md.
