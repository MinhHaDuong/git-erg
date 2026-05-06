# tickets/

Local textfile-based tickets store for the project.

Use the `tickets/erg` binary to manipulate tickets.

As a fallback, agents can read/write directly using the example template:

```text
%erg v1
Title: Add retry logic for failed API requests
Created: 2026-05-04
Author: alice
Blocked-by: 0007
Tag: Exemple

--- log ---
2026-05-04T09:00Z alice created

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

More information: `erg --help`.
Detailed specifications: `spec-erg-v1.md`
