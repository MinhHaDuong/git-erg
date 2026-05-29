# Design audit — is git-erg "infrastructure class"?

Author: Claude (design audit)
Created: 2026-05-29
Scope: the *design*, not the implementation. Sources read: README.md,
STATE.md, AGENTS.md, pep-erg-v1.md, tickets/spec-erg-v1.md, tickets/0146,
tickets/0151.

## Verdict

- **By architectural *shape*: yes.** The design carries all four defining
  properties of supply-chain infrastructure, and they live in the *design*,
  not the implementation.
- **By present *scale and criticality*: no.** The "maximal threat — binary
  embarked in thousands of repos, supply-chain + mass-RCE" framing in
  AGENTS.md and 0151 is aspirational, not current-state. More pointedly, that
  framing quietly breaks the project's own evidence-first bar.

One line: **git-erg is shaped like infrastructure but is not yet deployed
like it — and the single design decision that *creates* the class is
"commit the binary to the repo."**

## What makes something infrastructure-class

The test is not "is it important." It is **blast radius**: does one defect or
compromise become a *systemic* event across many independent dependents who
did not consciously opt into the risk? Hallmarks:

1. Wide, often-invisible deployment (many downstream dependents).
2. Automatic execution in *trusted / privileged* contexts.
3. Distribution as an opaque artifact (a supply-chain *source*, not just a
   consumer).
4. A self-propagation channel (auto-update).

## Where the design qualifies — and it genuinely does

All design-level choices, not implementation details:

- **Committed opaque binary** (README "Binary policy"; `tickets/erg`,
  ~6.2 MB). This is *the* property — the textbook supply-chain anti-pattern:
  a blob that travels with every clone. 0151 names it exactly: "an
  unverifiable committed binary *is* the liability."
- **Auto-execution in trusted contexts.** Pre-commit hooks and CI runners —
  precisely where credentials, tokens, and write access live. Code execution
  there is high-value.
- **Parses untrusted input in those contexts.** Anyone can PR a `.erg`;
  hooks/CI parse it. Path-traversal via crafted IDs/path-refs, ReDoS,
  oversized input are real classes (enumerated in 0151).
- **Self-update channel** (`update.go`; → `git fetch` in 0148). A propagation
  path: compromise once, ship everywhere.

The design's *response* to this framing is coherent and correct: reproducible
builds as the keystone control, signed git tags (`git tag -s`), full SHA-256
recomputable with stock tools, static/stripped/zero-dep, offline-verifiable
rebuild (`make verify`). If you accept the premise, these are the right
controls. The design is internally consistent *in its answer*. The open
question is whether the *premise* is right-sized.

## Where it does not qualify (caveats the docs underplay)

- **Dev-time metadata tooling, not a runtime dependency.** Real
  infrastructure (libc, OpenSSL, a logging lib, an npm package) sits in the
  dependent's *production* path and reaches *their* users. git-erg sits in the
  dev/CI *metadata* path. Worst-case compromise yields a dev/CI box — serious,
  but a tier below xz/liblzma or a poisoned production package. The blast
  radius stops at adopters' development environments; it does not reach the
  adopters' end users.
- **The binary is optional by the #1 invariant.** "Agnostic / POSIX-first"
  makes the `.erg` file the contract and the binary "optional." An adopter can
  run the whole system on `grep`/`find`/`Read`/`Edit` with zero binary; PEP §8
  concedes a `grep`-based hook would work. The *mandatory* execution surface
  is far smaller than "binary running in every hook and CI" implies — the
  supply-chain exposure is opt-in and removable.
- **Near-zero transitive surface.** Zero third-party deps; the only network
  call is being deleted (0148). git-erg is a *potential* supply-chain
  *source*, but barely a *consumer* of one — it lacks the deep dependency tree
  that makes most infrastructure dangerous.
- **"Thousands of repos" does not exist.** Adoption today is ~the author's own
  projects (STATE's `audit: usage in idh`; README still courting early
  adopters). The threat model is sized to a deployment that has not happened.

## The sharpest finding: an internal inconsistency

AGENTS.md disciplines *features* with "verified empirical need, not pushed
from a wishlist … if you cannot name who felt the lack, it is not ready." Yet
it sizes the *security posture* to an unverified "binary embarked in thousands
of repos." By the project's own rule, **that premise is wishlist-grade.** Two
clean resolutions:

- **(a) The framing is right because a design choice manufactures the risk.**
  Then the honest target of the audit is that choice: *why commit the binary
  at all*, given POSIX-first makes it optional? Shedding infrastructure-class
  risk is a **design lever already in hand** — do not commit the binary
  (build-from-source; binary purely opt-in), or make the grep-based hook the
  only mandatory binary-free path. The project leans the *other* way (README
  defends committing it for CI/agents without Go). That is a legitimate
  trade — but it is *the* decision that creates the class and deserves to be
  named as such in the PEP, not defended on convenience alone.
- **(b) The framing is precautionary — so label it.** Designing to the higher
  bar *before* adoption is cheap insurance for a tool *intended* to travel
  widely, but it should read "infrastructure-class discipline, adopted
  precautionarily ahead of adoption," not "maximal threat, now." Notably
  0151/0152 are correctly `needs-human` and *not* in `ready` — so the project
  is not yet over-investing. The fix here is wording, not effort.

## Alternative considered: vendor the source, rebuild on demand

Instead of committing the binary, vendor `src/go/` (already in the repo) and
have the box rebuild `erg` on first use, caching a gitignored `build/erg`.
This is the natural expression of the lever above. Scored against the four
conferring properties:

| Property | Committed binary | Vendor + rebuild-on-use |
|---|---|---|
| Opaque blob travels | present (the liability) | **eliminated** — only source travels |
| Self-update propagation | present (`update.go`) | **eliminated** — `git pull` + rebuild |
| Auto-exec in hooks/CI | yes | yes (unchanged) |
| Parses untrusted input | yes | yes (unchanged) |

It removes the two *distinctive* infrastructure-class properties (the
supply-chain *source* ones). What remains — auto-exec + parse untrusted
input — is shared with every linter (eslint, black, gofmt all run in
hooks/CI on untrusted code); by itself that is "normal dev tool," not
infrastructure class. Bonus: it kills the rubber-stamped-blob vector (the
git log is full of unreviewed `chore: rebuild bootstrap binary [skip ci]`
commits — source diffs are reviewable, blob diffs are not) and lets
`update.go`'s network code be deleted outright (folds in 0148).

**The one thing it trades away is the exact thing the committed binary was
introduced for** (README binary policy): *environments where Go may be
unavailable — CI runners, agents.* The whole proposal reduces to one
empirical bet: **do the boxes that run `erg` have Go?** Minimal CI images
(alpine/distroless, Node/Python images) often don't; pre-commit hooks run on
contributor laptops; agent sandboxes vary. Requiring Go is a hit to the
*agnostic / travels-everywhere* invariant.

Three second-order costs even where Go is present: a `fast` tax (a staleness
check — source hash vs `build/erg` — on every invocation, plus a ~1–3s first
build per source-change); a `stateless` tension (the cached artifact needs
atomic build-to-temp-then-rename so parallel worktrees don't race); and a
hard requirement to **pin the Go toolchain** (`go.mod` toolchain directive),
without which different Go versions defeat "reproducible." Upside of the
pin: every box then performs 0151's reproducible-build check *implicitly*, as
the default code path rather than a separate `make verify` step.

What it does **not** fix: a malicious PR editing `src/go/*.go` still executes
in CI on rebuild — identical risk in both models (it is the auto-exec
property, not the binary), except reviewable source beats an opaque blob for
catching it. With reproducible builds, committed-binary and vendor-rebuild
are the *same trust model*; vendoring just drops the cache.

### Decision (2026-05-29): do not require Go to be useful

The author's constraint is **minimal barriers to adoption — `erg` must be
useful without a Go toolchain.** That settles the bet against
"rebuild-only," and has a clean consequence for this audit:

- **The committed binary stays** — it is what gives no-Go boxes the full CLI.
  Therefore the project **remains structurally infrastructure-class by
  deliberate choice**, not by accident or wishlist.
- This **resolves the precautionary-vs-real question** (the "internal
  inconsistency" above) in favour of *real*: the maximal threat model is
  justified by a design decision the author is consciously making, so 0151's
  reproducible-build + signed-tag controls are **non-negotiable, not
  precautionary**. Keeping the blob *obligates* the controls that make the
  blob verifiable.
- **Vendor-and-rebuild becomes an additive trust layer, not a replacement.**
  The honest distribution stance is a three-tier floor:
  1. *POSIX / grep hook* (transport #1) — the floor; always useful, zero
     binary, zero Go.
  2. *Rebuild from vendored `src/go/`* — for boxes that have Go and want to
     verify or skip the blob; reproducible build makes it byte-equal to the
     committed binary.
  3. *Committed binary* — the convenience artifact for the no-Go majority the
     constraint is protecting; trusted via reproducible-build equivalence to
     tier 2 and a signed release tag, not on faith.

In short: "do not require Go" is a legitimate adoption-first choice, but it
is *the* choice that keeps git-erg in the infrastructure class. The price of
that choice is paid in full by — and only by — shipping the 0151 controls.

## Bottom line

- **Structurally infrastructure-class: yes** — the committed-binary +
  auto-exec-in-hooks/CI + untrusted-input + self-update combination is the
  defining shape, and it is a property of the design.
- **Criticality infrastructure-class: not yet**, and bounded by being
  dev-time + optional + no-runtime-path.
- **The class is conferred by exactly one design decision — committing the
  binary.** Everything else is shared with any linter. The lever to *not* be
  infrastructure-class (vendor + rebuild) exists, but using it would require
  Go on every box.
- **Decided: keep the binary, because `erg` must be useful without Go**
  (adoption-first). That is a deliberate choice to *stay* infrastructure-class,
  which is the honest, internally-consistent position — and it converts 0151's
  reproducible-build + signed-tag controls from "precautionary" to
  **obligatory**: shipping the blob is what pays for keeping it. Vendored
  source + reproducible rebuild remains the verification tier (and the opt-out
  for Go-having boxes), layered over a POSIX/grep floor that needs neither
  binary nor Go.
