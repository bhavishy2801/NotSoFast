# Implementation plan

The user-supplied [specification](SPEC.md) is the design authority. Execute inline, in gate order.

Architecture: one Go core, local controlled Git subprocesses, SQLite receipts and memoization; CLI and HTTP call the same methods. Operator configuration fixes repositories, principals, policy and budgets. Git operation references retain publication identity independently of SQLite. Python and MCP only transport requests.

1. Core: `core/types.go`, `git.go`, `store.go`, `evidence.go`, `action.go`; integration tests over real temporary Git repositories. Test partial search UNKNOWN, missing search REFUTED, complete absence SUPPORTED and compare-and-swap creation before implementing.
2. Composition/reuse: test set union, invalid references, scopes, content domains, cancellation, object limits, snapshot changes and independent oracle agreement. CLI JSON requests and benchmark command. Record early fresh/memoized/receipt measurements before HTTP.
3. Operations: requester-scoped identity, canonical digest, Git metadata commit plus operation ref in publication transaction. Test conflict, retry, concurrent requests, termination and fail-closed recovery. Verify permissions, limits and storage ceiling.
4. Integrations: bounded authenticated HTTP handler, Python stdlib client and stdio MCP adapter, actual service integration test, Docker Compose and CI.
5. Release evidence: six executable demonstrations, fair baselines/ablations, measured results, threat model, protocol, correctness argument, audit and documentation. Record unavailable checks explicitly.

Review focus: path-component scopes; malformed/incompatible receipts before witnesses; empty domains backed by complete manifests; branch/operation partial visibility after interruption; no agent-controlled policy or repository paths. No external publishing or source repository mutation.

## Execution record

- Gates 1–2: actual Git/SQLite core and CLI implemented; early comparison run before HTTP. An experimental fixture carry-over bug was fixed and measurements rerun. Memoization retained; receipt speed hypothesis not established.
- Gate 3: operation intents, conditional refs, configuration epochs, grants, limits, retention and crash/recovery checks implemented. Independent review found and drove regression fixes for overlapping policies, candidate limits, operation identity, orphan epochs, repository revocation and interrupted registration.
- Gate 4: shared dispatch, bounded authenticated HTTP, stdlib Python client and stdio MCP implemented and exercised against the real service.
- Gate 5: six demos, scripted controls, 64-cell matrix, larger filename case, ablations, profiles, audit and docs produced. CI/container definitions supplied; runtime validation unavailable on this host. Model-driver harness tested without an actual model.
- Ruling: repository-granular access is the first-release access boundary; no path ACLs are claimed. Whole configured repository access determines what gap information may be returned.
- Ruling: pending unconfirmed intents remain UNRESOLVED; automatic replay/administrative abort is excluded rather than guessing after interruption.
- Ruling: policy requirements are conjunctive across applicable destinations; caller-selected policy never overrides other rules.
- Remaining measurement limits: combined-process CPU, peak RSS, physical cold caches and statistical multi-machine variability. No power-loss or universal MCP-host compatibility claim.
