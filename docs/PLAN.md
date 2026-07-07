# Implementation plan

The user-supplied [specification](SPEC.md) is the design authority. Execute inline, in gate order.

Architecture: one Go core, local controlled Git subprocesses, SQLite receipts and memoization; CLI and HTTP call the same methods. Operator configuration fixes repositories, principals, policy and budgets. Git operation references retain publication identity independently of SQLite. Python and MCP only transport requests.

1. Core: `core/types.go`, `git.go`, `store.go`, `evidence.go`, `action.go`; integration tests over real temporary Git repositories. Test partial search UNKNOWN, missing search REFUTED, complete absence SUPPORTED and compare-and-swap creation before implementing.
2. Composition/reuse: test set union, invalid references, scopes, content domains, cancellation, object limits, snapshot changes and independent oracle agreement. CLI JSON requests and benchmark command. Record early fresh/memoized/receipt measurements before HTTP.
3. Operations: requester-scoped identity, canonical digest, Git metadata commit plus operation ref in publication transaction. Test conflict, retry, concurrent requests, termination and fail-closed recovery. Verify permissions, limits and storage ceiling.
4. Integrations: bounded authenticated HTTP handler, Python stdlib client and stdio MCP adapter, actual service integration test, Docker Compose and CI.
5. Release evidence: six executable demonstrations, fair baselines/ablations, measured results, threat model, protocol, correctness argument, audit and documentation. Record unavailable checks explicitly.

Review focus: path-component scopes; malformed/incompatible receipts before witnesses; empty domains backed by complete manifests; branch/operation partial visibility after interruption; no agent-controlled policy or repository paths. No external publishing or source repository mutation.
