# Changelog

All notable changes to git-erg. The format loosely follows
[Keep a Changelog](https://keepachangelog.com/); dates are UTC and each entry
cites its ticket and merged PR.

## [Unreleased] -- erg-imagine-charter implementation (2026-06-02 .. 2026-06-03)

Implements `docs/erg-imagine-charter.md`: erg core stays offline and
forge-blind, a committed `erg-github` script carries the GitHub layer, dpkg-style
versioning is added for the embedded assets, and ticket autoclose is enforced by
a required CI check rather than a bot. Sixteen PRs (#239-#254), each gated
through `/verify`.

### Added
- **`erg install`** -- a new verb split out of `init`; the only verb that mutates
  outside `tickets/`, and only behind opt-in flags. `--hooks` installs a
  pre-commit hook (validate + check + reject committing the traveling
  `tickets/erg` off main), inserted before any third-party hook content and
  upgrading legacy managed-block markers in place. `--push-hook` installs a
  WARN-only pre-push hook that names closed-but-unarchived tickets and never
  blocks the push. `--inject-agents` (with `--create-agents-md` for consent)
  adds a pointer to `tickets/AGENTS.md` in the project-root AGENTS.md. All
  writes are pre-flighted (a refused run changes nothing) and atomic.
  (0205, 0208 -- #240, #243)
- **`erg spec`** and **`erg integration`** -- print the embedded `%erg 0.1`
  format spec and the setup guide on demand. (0206 -- #241)
- **`erg init -n` / `--dry-run`** (zero-side-effect preview) and **`--force`**
  (overwrite divergent files), plus a shared, documented exit-code table:
  `0` success, `1` hard error, `2` local edits preserved-and-skipped. (0207 -- #242)
- **`tickets/erg-github`** -- a committed POSIX-sh forge helper (run directly,
  not an `erg` subcommand). `verify` fails a PR that references a still-open
  ticket (no bot; fail-closed under CI, lenient locally; an audited escape hatch
  for unlinked PRs). `install` writes a pinned GitHub Actions workflow. (0209 -- #244)
- **`tickets/.erg-assets`** -- a deterministic provenance manifest (binary
  rev/date plus the SHA-256 of each embedded asset), written by `init` and
  `migrate`. (0210 -- #245)
- **dpkg-style 3-state asset compare** in `init`: unchanged / clean-upgrade
  (overwrite, with a `git restore` hint) / local-edit (preserve, exit 2), using
  the manifest as the reference stamp and a baked historical-hash fallback when
  none is present. (0211 -- #247)
- **Asset-drift warning** in `erg check`, and a post-update "run erg init" hint,
  when the binary's embedded assets move past the deployed stamp. (0212 -- #248)
- **`erg version` role field** -- `traveling` (the committed `tickets/erg`) vs
  `system` (a copy on PATH); the README documents the `.gitignore` opt-out.
  (0214 -- #250)
- **`make check`** pre-PR gate (full test suite + ticket-corpus validation) and
  **`make regen-assets`** to regenerate the dogfood copy from `src/go/assets/`.
  (0215, 0213 -- #246, #249)
- **Adherence ratchets:** `tests/test_gofmt.sh` (gofmt + `go vet`) and
  `tests/test_selfcoherence.sh` (a clean `erg init` reproduces the embedded
  assets; the deployed copy matches embedded). (0217, 0213 -- #253, #249)

### Changed
- **`erg init`** is now pure scaffolding: it no longer arms hooks (that moved to
  `erg install`) and writes only `.ergrc` + `AGENTS.md`, silently removing
  matching orphan assets it finds. (0205, 0206 -- #240, #241)
- **Autoclose model:** enforcement moved to the required `erg-github verify` CI
  check -- the close is committed in the PR by the author via erg core -- instead
  of a closing bot; autoarchive became a non-mutating pre-push warning, because a
  pre-push hook cannot get a file move into the push it gates. (0209 -- #244)
- **git-erg dogfoods the embedded defaults verbatim:** the diverged
  `tickets/AGENTS.md` was reconciled and `tickets/.ergrc` added so both match
  `src/go/assets/`, enforced by the self-coherence guard. (0213 -- #249)

### Removed
- Stale tracked `tickets/spec-erg-v1.md` (and the already-untracked
  `integration.md`) -- now embedded and served via `erg spec` / `erg
  integration`; a guard prevents their re-introduction. (0218 -- #254)

### Security
- **Scoped ASCII relaxation.** The ASCII-only `src/go` guard now allows exactly
  two code points -- U+201C and U+201D, the curly double quotes gofmt's
  doc-comment formatter emits for two-backtick and `''` pairs (Go 1.19+) -- in
  `*.go` only. The embedded assets (`*.md`, `.ergrc`) stay strictly ASCII, and
  every other non-ASCII byte (em-dash, bidi-override, homoglyph) is still
  rejected, so the Trojan-Source surface (CVE-2021-42574) is not reopened.
  Documented in `docs/threat-model.md`, with negative controls. (0167, 0217 -- #253)

### Internal
- Planning ticket 0204 seeded the 10-ticket execution set from the charter
  (#239); post-raid follow-up tickets were filed (#251); and a root-cause
  correction recorded that gofmt's quote rewriting is stock Go 1.19+ smart-quote
  behaviour, not a broken toolchain (#252).
