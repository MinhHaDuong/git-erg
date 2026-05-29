# git-erg red-team checklist

Author: Claude (security audit, ticket 0151 → 0158)
Created: 2026-05-29

A repeatable, AI-assisted red-team procedure for the surfaces in
[`threat-model.md`](./threat-model.md). It is **cheap to re-run** — most items
delegate to the existing CI suites — so run it often.

**Cadence.** Per release (before tagging) and per change to the parser, the
update path, or path/ID resolution. **Process:** an agent runs every item and
records results in a dated run-log section below; a human reviews findings and
assigns severity; any high-severity finding becomes its own fix ticket. Do not
treat a green checklist as "done forever" — the value is in the re-run.

Run the binary under test with `make build` (`build/erg`), never the committed
`tickets/erg`, except where an item explicitly inspects the committed blob.

## Items by attack surface

### S1 — Supply chain / reproducible builds

- **S1.1 Reproducible rebuild.** `make verify`. *Expected:* `verify: PASS` —
  the binary rebuilt from the committed revision byte-matches `tickets/erg`.
- **S1.2 Tamper check.** Compare `sha256sum tickets/erg` against the `sha256:`
  digest that `tickets/erg version` self-reports. *Expected:* identical
  64-char digests (and both equal the `make verify` committed/rebuilt hash). A
  mismatch means the blob does not match its own claimed build.

### S2 — Update channel

- **S2.1 Hijack refusal + offline no-op.** Covered by `tests/test_update.sh`:
  `update` with no discoverable ticket store refuses (no cwd-repo hijack);
  `update` offline exits 0 and leaves the binary untouched. *Expected:* both
  pass. **Reference the suite — do not stand up a network sandbox by hand.**
- **S2.2 No network code.** `tests/test_update.sh` asserts the source carries
  no `net/http` / `crypto/tls` (the offline invariant; update is git-transport
  only). *Expected:* pass.

### S3 — Parser / input DoS

- **S3.1 Bounded parse.** `tests/test_security.sh` Group 6: a 10 MB body and a
  100,000-line log section each validate within a 5 s budget; a 10,000-char
  title slug truncates to ≤40 chars (no ReDoS). *Expected:* all pass, with a
  normal-file negative control.

### S4 — Path / ID injection

- **S4.1 Traversal + embedded-separator IDs.** `tests/test_security.sh`
  Groups 1, 1b: `../../etc/passwd` and `0042/../../../etc.erg` style IDs are
  refused by `close` / `log` / `rm`; a real in-store ID still works (negative
  control). *Expected:* pass.
- **S4.2 Symlink escape + write/delete confinement.** Groups 2, 3, 4:
  close/rm through an in-store symlink never touch the external target; `rm`
  refuses a valid `.erg` outside the named store (0157 FIX 1); `new`'s explicit
  DIR behaves as documented (0157 FIX 2). *Expected:* pass.
- **S4.3 Ref injection + glob ambiguity.** Groups 5, 7: a traversal payload in
  `Blocked-by:` is rejected as a malformed ref (never resolved as a path); a
  quoted `*` ID is refused as ambiguous rather than acting on an arbitrary
  ticket. *Expected:* pass.

### S5 — Secret hygiene

- **S5.1 No env-secret leak.** Set canary secrets in the environment
  (`SECRET_TOKEN`, `AWS_SECRET_ACCESS_KEY`, `GITHUB_TOKEN`), run a spread of
  commands (`version`, `list`, `list --json`, `ready`, `check`, `new`,
  `--help --all`), and grep all output plus any created ticket for the canary.
  *Expected:* the canary appears nowhere.

## Run log — 2026-05-29

Binary under test: `build/erg`, revision `286de8e`, `linux/amd64`. Committed
`tickets/erg`: revision `0f899c7`, sha256
`9a8008565dec9f1576e822bd5f04533185b894c180552aa2ffaee010f3509f64`.

| Item | Result | Evidence |
|---|---|---|
| S1.1 Reproducible rebuild | PASS | `make verify` → `verify: PASS`; committed == rebuilt == `9a80…9f64`, toolchain go1.21.13 |
| S1.2 Tamper check | PASS | `sha256sum tickets/erg` = `9a80…9f64` = the `sha256:` line from `tickets/erg version` |
| S2.1 Hijack refusal + offline no-op | PASS | `tests/test_update.sh`: "update refuses when no ticket store is found", "update offline exits 0 and leaves binary untouched" (14/14 passed) |
| S2.2 No network code | PASS | `tests/test_update.sh`: "no net/http or crypto/tls in source (offline invariant)" |
| S3.1 Bounded parse | PASS | `tests/test_security.sh` Group 6: 10 MB body, 100k-line log, 10k-char title slug truncated to 40 — all within budget (26/26 passed) |
| S4.1 Traversal + embedded-separator IDs | PASS | `tests/test_security.sh` Groups 1/1b: traversal and embedded-separator IDs refused; negative controls succeed |
| S4.2 Symlink + write/delete confinement | PASS | Groups 2/3/4: symlink escape blocked; out-of-store `rm` refused; `new` DIR as documented |
| S4.3 Ref injection + glob ambiguity | PASS | Groups 5/7: traversal `Blocked-by:` rejected (malformed ref); `*` ID refused as ambiguous |
| S5.1 No env-secret leak | PASS | Ran `version`/`list`/`list --json`/`ready`/`check`/`new`/`--help --all` with three canary env secrets set; grep of all output and the created ticket found zero canary occurrences |

**Findings:** none. All surfaces behaved as specified; both CI suites are green
(security 26/26, update 14/14) and `make verify` reports PASS. No high-severity
items, so no fix tickets are filed from this run. The signed-release-tag control
(threat model, deferred-human) remains the one outstanding item, tracked on
ticket 0151 and out of scope for an automated run.
