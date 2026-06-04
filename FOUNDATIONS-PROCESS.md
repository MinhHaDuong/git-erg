# Foundations Coherence Process

Repeatable AI-assisted QA for git-erg's foundational coherence: do the docs
and the guards still agree on what the tool is? Agents run the procedure;
humans gate the findings. The foundations are stated across many sites and
enforced by the guard tests, and they drift as the tool changes -- a guard
lands without a sentence, or a sentence is written without a guard. This file
is the orchestration layer: when to run, how to prompt the agent, and how
findings become tickets.

## When to run

- Before tagging a release.
- After any change to a contract surface: `AGENTS.md`, `pep-erg-v1.md`, the
  help/spec text (`helptext.go`, `erg spec`), a `*-PROCESS.md`, or the guard
  tests (`tests/`, `src/go/*_test.go`).

## Scope

The six design-contract traits (agnostic, offline, standalone, stateless,
fast, small), the posture properties (security, UX), and the file-format and
CLI invariants. A contract is anything the project promises and can be held to.
Out of scope: feature wishlists and aspirational prose with no guard.

## Procedure

### Phase 1 -- Sweep

Inventory the contracts across every site that states or enforces one: the
constitution (`AGENTS.md`), the rationale (`pep-erg-v1.md`), the spec
(`erg spec`), the process docs (`*-PROCESS.md`), and the guard tests
(`tests/`, `src/go/*_test.go`).

Cold prompt for the sweep agent:

> You are auditing the foundational coherence of git-erg, a small Go CLI whose
> binary is committed to every repo that uses it. A "contract" is any property
> the project states and can be held to.
>
> Task: build one inventory of every contract. For each, record where it is
> STATED (`AGENTS.md`, `pep-erg-v1.md`, `erg spec`, a `*-PROCESS.md`) and where
> it is GUARDED (`tests/`, `src/go/*_test.go`). Cite file and line. Do not
> dedupe across sites yet -- record each statement and each guard separately.

### Phase 2 -- Verify (disjoint union)

For each contract, classify it. The healthy region is the intersection:
stated AND guarded. The two failure regions form a disjoint union, and each
side has a distinct remediation:

- **stated-without-guard** -- a promise no test holds. Remediation: a missing
  TEST. Write a falsifiable guard with a negative control.
- **guarded-without-statement** -- a test enforcing a property no doc names.
  Remediation: a missing SENTENCE. State the contract where readers look for
  it, or delete the guard if the property is not really promised.

Compute `(stated-without-guard) union (guarded-without-statement)`; the two
sides do not overlap. Then list inter-doc tensions explicitly: where two sites
state the same contract with different scope, wording, or numbers (e.g. a size
cap quoted as one value in `AGENTS.md` and another in `pep-erg-v1.md`). A
tension is not a gap -- both sites speak, but they disagree, and a release must
not ship a self-contradicting foundation.

### Phase 3 -- Communicate

Check the canonical-source-plus-pointers structure: each contract has one
home, and the other sites point to it rather than restating it (drift starts
when a property is duplicated in full across sites). Flag any contract living
ONLY in a decision record (`docs/erg-imagine-charter.md`) or `CHANGELOG.md` --
those are history, not the standing constitution; a live contract must be
lifted into `AGENTS.md`, the spec, or a process doc, with the record left as
provenance.

## Output format

The run produces:

- An inventory table: contract | stated-at | guarded-at | classification (healthy / missing-test / missing-sentence).
- A tensions list: contract | site A says | site B says | proposed resolution.
- A relocation list: contracts found only in a decision record or CHANGELOG, with the proposed canonical home.
- A findings paragraph and a next-run note.

## After the run

1. A human confirms each classification (a guarded-without-statement item may be an intentional internal invariant, not a missing contract).
2. Missing-test findings become fix tickets: `erg new "<concise title>"` with the unguarded property and a proposed negative control.
3. Missing-sentence and tension findings get tickets unless the wording fix is trivial enough to land in the same PR.
4. Relocations out of a decision record or CHANGELOG are human-gated -- the agent proposes the canonical home but does not move the contract unattended.

## What CI checks independently

These run on every push and do not need to be re-run manually:

- **Asset drift**: `tests/test_selfcoherence.sh` -- the embedded `src/go/assets/AGENTS.md` matches the deployed `tickets/AGENTS.md`.
- **Doc coherence**: `tests/test_docs.sh` -- greps that pin help-section counts, install wording, and the CONTRIBUTING checklist touch-points.
- **Six-trait guards**: `tests/test_contract.sh` -- each trait ships a guard with a negative control.
- **Suite registration**: `_test-lint` -- every test file is registered in the suite runner, so a guard cannot silently drop out.

The sweep covers judgment and inter-doc tensions; CI covers regressions on the
falsifiable subset. Both are needed.
