# git-erg threat model

Author: Claude (security audit, ticket 0151 → 0158)
Created: 2026-05-29
Scope: the attack surfaces of the `erg` CLI and the `.erg` data it manages.

This is the consolidated threat model for git-erg. The *design-level*
justification for treating git-erg as infrastructure-class lives in
[`audit-infrastructure-class.md`](./audit-infrastructure-class.md) and is not
restated here — read that doc for the "why." This doc enumerates **what** we
defend, **against whom**, **where** the surfaces are, and **which controls**
are in place. The repeatable verification procedure is
[`red-team-checklist.md`](./red-team-checklist.md).

## Assets

- **`tickets/` data integrity.** The `.erg` files are the project's backlog and
  decision log — its long-term memory. Corruption, undetected tampering, or a
  malicious mutation that escapes the store is a direct loss.
- **Hook / CI execution context.** `erg` runs in pre-commit hooks and CI
  runners — exactly where credentials, tokens, and write access live. Code
  execution there is high value; the execution *context*, not just the data, is
  an asset to protect.

## Threat actors

- **Compromised supply chain** — someone who can land a commit (or a poisoned
  rebuild) swaps the committed `tickets/erg` blob for a malicious one. Every
  clone that trusts the blob then executes attacker code in its hooks/CI.
- **Malicious PR author** — anyone can open a PR adding or editing a `.erg`
  file; hooks and CI parse it. A crafted ticket can attempt path traversal,
  resource exhaustion, or ref injection without ever being merged.
- **Compromised update channel** — an attacker who can intercept or impersonate
  the update source tries to get `erg update` to install a hostile binary, or
  to silently downgrade a patched one.

## Attack surfaces

1. **Supply chain / reproducible builds (keystone).** The committed
   `tickets/erg` is an opaque ~2 MB blob that travels with every clone. The
   defense is reproducibility: anyone must be able to rebuild it bit-for-bit
   from `src/go/` and confirm the committed blob matches. An unverifiable
   committed binary *is* the liability; reproducibility is what earns the right
   to ship one.
2. **Update channel.** `erg update` (git transport, 0148) must not be coercible
   into installing an attacker binary — no untrusted origin, no silent
   downgrade, no install from a hijacked working-directory repo.
3. **Parser / mass code-execution.** `erg` parses untrusted `.erg` input in
   privileged contexts. The parser must not be exploitable for RCE, and must
   not hang or OOM on oversized / deeply-nested / pathological input (DoS,
   ReDoS).
4. **Path / ID injection.** Ticket IDs and path-refs feed filesystem paths
   (`resolveDir`, `resolveTicketByID`, and the `new` / `rm` / `archive` rails).
   A crafted ID or ref (`../`, an embedded separator, an absolute path, a
   symlink) must not let a mutation read or write outside the store.
5. **Secret hygiene.** `erg` must never read, write, or log secrets, and must
   never copy environment variables or keys into ticket text or command output.

## Severity multiplier

git-erg is **infrastructure-class by deliberate design choice**, not by current
scale. The author's decision to keep the committed binary — so that boxes
without a Go toolchain still get the full CLI (see
[`audit-infrastructure-class.md`](./audit-infrastructure-class.md) §Decision) —
is precisely the choice that confers supply-chain-source status: an opaque blob
that travels with every clone and self-updates. That structural property, not a
deployment that exists today, is what raises the severity floor: a defect or a
poisoned blob is a *systemic* event across every adopter that trusts it, so
"low" findings are rarely low.

Be honest about adoption: it is small today (≈ the author's own projects).
These controls are not insurance against some future large deployment — they
are the mandatory cost of the design choice to ship a committed binary. That
choice was made; the controls come with it, regardless of how many repos have
adopted git-erg today.

## Controls

| Control | Status | Reference |
|---|---|---|
| Static, stripped, zero-dependency build (`CGO_ENABLED=0 -trimpath`, no third-party deps) | shipped | ticket 0147 |
| `erg version` names the algorithm (SHA-256) and prints the full 64-char digest + a `verify:` recompute hint | shipped | ticket 0155 |
| `make verify` rebuilds the committed binary from its embedded revision and byte-diffs it | shipped, CI-tested | ticket 0156; `tests/test_verify.sh`, run by `make test` in CI |
| Path-traversal / ID-injection / symlink / input-DoS hardening, each with a negative control | CI-tested | ticket 0157; `tests/test_security.sh` |
| `rm` FILE-form and `new` explicit-DIR confinement to the named store | shipped | ticket 0157 |
| Update integrity via git transport — no `net/http`, no `crypto/tls` in source; refuses cwd-repo hijack; offline no-op | shipped | ticket 0148; `tests/test_update.sh` |
| Signed release tags (`git tag -s`, verified with `git verify-tag`) as the trustable publication | shipped | ticket 0151; tag `2026-05-30`, key `4A46C91E03B83B23` (YubiKey-backed) |

`shipped` = present in the binary today. `CI-tested` = exercised by a test that
runs on every push. `deferred-human` = requires a human action (a maintainer
key) before it can ship.

## Standing QA process

Security here is a **standing process**, not a one-shot audit. Re-run the
[red-team checklist](./red-team-checklist.md):

- **Per release.** Before tagging, run the full checklist and record results.
- **Per change to a high-risk surface.** Any change to the parser, the update
  path, or path/ID resolution (`erg.go`, `ref.go`, `main.go`, `update.go`,
  `new.go`, `rm.go`, `archive.go`) triggers a re-run of the relevant items.

A human reviews the recorded findings and sets severity. High-severity findings
become their own fix tickets. Threat-model changes themselves are human-gated.
