kind: review-attribution
pr: 386 · merged 2026-11-08 · project: git-erg
writer: step=plan · runtime=claude · model=anthropic/claude-opus-5-5 · effort=not-stated · route=subagent · independence=not-stated
writer: step=execute · runtime=claude · model=anthropic/claude-opus-5-5 · effort=not-stated · route=subagent · independence=not-stated
reviewer: step=review · seat=correctness · runtime=claude-code · model=anthropic/sonnet · effort=not-stated · route=unconfirmed · independence=not-stated · status: ran
  finding: consider · src/go/claims_test.go:159 · adopted: no
  finding: consider · src/go/install.go:117 · adopted: no
  finding: consider · src/go/claims.go:96 · adopted: no
reviewer: step=review · seat=redteam · runtime=codex · model=openai/gpt-6.1-sol · effort=not-stated · route=unconfirmed · independence=not-stated · status: ran
  finding: consider · src/go/claims.go:149 · adopted: no
  finding: consider · src/go/claims.go:219 · adopted: no
reviewer: step=review · seat=exitcriteria · runtime=claude-code · model=anthropic/claude-opus-5-5 · effort=not-stated · route=self · independence=not-stated · status: ran
  finding: verifiable · src/go/new.go:226 · adopted: yes
reviewer: step=review · seat=docs · runtime=claude-code · model=anthropic/sonnet · effort=not-stated · route=unconfirmed · independence=not-stated · status: ran
  finding: consider · src/go/new.go:83 · adopted: no
  finding: consider · src/go/assets/integration.md:151 · adopted: no
  finding: consider · src/go/assets/spec-erg-v1.md:222 · adopted: no
reviewer: step=review · seat=wording · runtime=claude-code · model=anthropic/sonnet · effort=not-stated · route=unconfirmed · independence=not-stated · status: ran
  finding: consider · src/go/new.go:248 · adopted: no
  finding: consider · src/go/assets/integration.md:179 · adopted: no
  finding: consider · src/go/init.go:724 · adopted: no
reviewer: step=review · seat=holds · runtime=claude-code · model=anthropic/claude-opus-5-5 · effort=not-stated · route=self · independence=not-stated · status: ran
reviewer: step=review · seat=delta · runtime=claude-code · model=anthropic/sonnet · effort=not-stated · route=unconfirmed · independence=not-stated · status: ran
  finding: consider · src/go/migrate.go:656 · adopted: no
reviewer: step=review · seat=parity · runtime=claude-code · model=anthropic/claude-opus-5-5 · effort=not-stated · route=self · independence=not-stated · status: ran
  finding: consider · src/go/migrate.go:656 · adopted: no
reviewer: step=review · seat=delta · runtime=claude-code · model=anthropic/sonnet · effort=not-stated · route=unconfirmed · independence=not-stated · status: ran
  finding: consider · src/go/claims.go:322 · adopted: no
reviewer: step=review · seat=parity · runtime=claude-code · model=anthropic/claude-opus-5-5 · effort=not-stated · route=self · independence=not-stated · status: ran
