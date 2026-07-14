# Related work

Reviewed primary sources on 2026-10-05. This is a scoped engineering comparison, not a novelty or exhaustive-literature claim.

| System / mechanism | Shared idea | Distinction in this implementation |
|---|---|---|
| [Git update-ref](https://git-scm.com/docs/git-update-ref) | Expected-old-value reference updates and transactions | NotSoFast uses this existing primitive; receipt checks are additional application preconditions, not a replacement for concurrency control |
| [Bazel remote caching](https://bazel.build/remote/caching) | Cached deterministic results keyed by immutable relevant inputs | NotSoFast's cache is ordinary memoization; receipts additionally expose exact scoped coverage and derivation for absence claims |
| [SLSA provenance](https://slsa.dev/spec/v1.1/provenance) | Recorded provenance and traceability | Search receipts concern a small exact predicate/domain, not build provenance or proof that arbitrary output is trustworthy |
| [SQLite atomic commit](https://sqlite.org/atomiccommit.html) | Durable local metadata transactions | Git remains publication authority; SQLite metadata alone cannot answer whether a branch update occurred |
| [MCP tools](https://modelcontextprotocol.io/specification/2025-11-25/server/tools) and [stdio transport](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports) | Agent tool transport and structured results | The adapter only forwards; MCP itself is not the policy/evidence enforcement boundary |

Inference: the reusable contribution demonstrated here is an integration of explicit coverage, set composition, gap recovery and a narrowly guarded action. The performance measurements do not justify inventing a new caching claim, and the source comparison does not establish research novelty. A trust record's integrity is distinct from the correctness of the system that issued it.
