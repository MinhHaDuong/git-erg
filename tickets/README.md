# tickets/

Local file-based tickets store for the project.

Agents should ensure the `erg` binary helper is available to manipulate tickets.
To get it: check `tickets/tools/go/erg` (committed bootstrap binary, Linux x86-64 only),
or build from source: `cd tickets/tools/go && go build -o erg .`

As a fallback, agents can read/write directly using the example template:

```text
%erg v1
Title: Add retry logic for failed API requests
Created: 2026-05-04
Author: alice
Blocked-by: 0007

--- log ---
2026-05-04T09:00Z alice created
2026-05-04T14:22Z bob status open Was blocked, 0007 now merged

--- body ---
## Context
The HTTP client silently drops requests when the upstream returns 503.
We need exponential backoff with jitter, capped at 3 retries.

## Exit criteria
- [ ] `client.Fetch()` retries up to 3 times on 5xx
- [ ] Backoff is 1s, 2s, 4s + random jitter ≤ 500ms
- [ ] Unit test covers retry exhaustion path
- [ ] `make check` passes
```

Rules agents must know:
- No `Status:` header in %erg v1 (use `erg migrate` for legacy files)
- Closed/not-closed is inferred from path conventions or a non-empty `Closed:` header
- `Tags:` is optional and repeatable; accepted values are `needs-human`, `deferred`, `post-talk`, `post-conference`
- Log entries are append-only: `YYYY-MM-DDTHH:MMZ author verb detail`

In doubt, read the specification `spec-erg-v1.md`
