# NotSoFast
## Reusable evidence for state-changing AI agent actions

You are the implementation engineer for NotSoFast. Build, test, benchmark, audit, and document the project described below.

This specification supersedes earlier versions.

Make routine technical decisions independently. Inspect the existing workspace and applicable instructions before changing files. Preserve unrelated work. Ask questions only when essential information or authorization is missing.

Deliver working software. Do not stop at plans, scaffolding, mock integrations, or invented benchmark results.

## 1. Purpose and product boundary

Agents sometimes turn an incomplete observation—

> “My search returned nothing.”

—into an unjustified action:

> “It does not exist, so I will create another.”

NotSoFast records exactly what was searched, combines compatible observations, and checks whether the resulting evidence establishes an action’s configured precondition.

The first release supports exact absence claims over immutable Git snapshots and one mutation: creating a file on a service-managed branch.

Its core flow is:

    Search an identified snapshot
        → issue trusted evidence
        → compose coverage
        → verify an exact claim
        → check authorization and policy
        → conditionally publish a change

An agent may propose an action. It cannot author authoritative evidence, weaken policy, or certify its own preconditions.

### What this release does not claim

It does not establish:

- Semantic absence of an equivalent implementation.
- General AI safety.
- Absence of vulnerabilities.
- Correctness of arbitrary generated code.
- Protection for operations that bypass its gateway.
- Enforcement across arbitrary external APIs.
- Research novelty without a related-work investigation.

It requires no GPU, trained model, paid infrastructure, or model API key for its core functionality and demonstrations.

## 2. Concrete use case

The snapshot contains:

    config/database.yaml

An agent searches only `src/`, finds nothing, and proposes creating:

    database.yaml

The destination is different, so ordinary overwrite protection would permit the creation.

An operator-configured policy requires:

> Before creating a file named `database.yaml`, establish that no tracked entry with that basename exists anywhere in the selected snapshot.

NotSoFast classifies the incomplete evidence as `UNKNOWN`.

After searching the missing scope, it finds `config/database.yaml`, classifies absence as `REFUTED`, and rejects the proposed action.

This enforces an exact naming convention. It does not identify all files that might semantically configure a database.

## 3. Research question and early feasibility check

Primary question:

> Can reusable, composable evidence enforce supported preconditions with practical benefits over fresh validation and conventional memoization?

Evaluate three dimensions separately:

1. **Correctness:** Does the implementation enforce its stated conditions?
2. **Efficiency:** Does it reduce measured work, latency, or resource use in identifiable workloads?
3. **Utility:** Does composition and recovery guidance help agents complete legitimate tasks?

Do not assume that receipt management improves performance.

### Required early experiment

Before building HTTP, MCP, or model integrations:

- Implement manifests, supported predicate evaluation, composition, and incremental reuse as a core library with a CLI.
- Compare fresh evaluation, ordinary memoization, and NotSoFast.
- Test unchanged, lightly changed, and heavily changed snapshots.
- Include cheap filename searches and content searches.
- Record manifest construction, lookup, hashing, database, and receipt overhead.

Publish an early result even if it is unfavorable.

If normal memoization is equally effective, keep it as the evaluation cache and position receipts around demonstrated composition, traceability, or recovery benefits.

Do not add features merely to rescue a performance claim.

The project must remain useful even if its efficiency hypothesis is rejected.

## 4. Architecture

Use:

- Go for the core library, CLI, and HTTP service.
- Git object storage for immutable snapshots and managed publication.
- SQLite for evidence metadata and rebuildable indexes.
- A bounded pool of service-controlled workers.
- A thin Python client and MCP adapter after the core is validated.
- Docker Compose and one CI provider.

Keep one service instance for the first release.

The CLI and HTTP handlers must call the same core implementation. The MCP adapter must delegate to that implementation rather than reproduce verification logic.

Use standard libraries where appropriate and a small number of maintained dependencies.

Do not build:

- Separate microservices.
- A generic workflow engine.
- An arbitrary policy programming language.
- A dashboard.
- A vector database.
- An LLM judge.
- Kubernetes deployment infrastructure.
- Untrusted distributed worker verification.

Concurrency within one service is not horizontal scalability.

## 5. Authoritative Git snapshots

### Registration and ownership

Explicitly register an allowed local repository and import a selected commit into service-owned Git storage.

Use a dedicated managed branch.

Do not alter the source working tree, source branches, source index, or source configuration. Do not push or publish externally.

Agents may select registered repositories; they may not cause arbitrary remote fetches or supply Git command options.

### Snapshot identity

Record:

- Registered repository ID.
- Git object-hash algorithm.
- Full commit ID.
- Full tree ID.
- Manifest format version.

Construct a complete canonical manifest from the Git tree.

Each entry identifies its path, type or mode, and object ID.

Use unambiguous Git output and serialization. Define filename handling explicitly. Preserve unusual names safely or fail explicitly for unsupported encodings.

Directory-prefix scopes must respect path-component boundaries.

### Universe boundaries

The authoritative universe contains tracked tree entries.

It excludes uncommitted and untracked files, runtime-generated files, external services, symlink targets, and submodule contents.

Do not follow symlinks or recurse into submodules.

A Git LFS pointer is the stored blob; its external payload is outside scope.

Missing objects or incomplete enumeration prevent claims of complete coverage.

User-facing output must describe this universe accurately.

## 6. Supported predicates and scopes

Implement versioned deterministic predicates:

### `exact_path`

Match one exact repository-relative entry path.

### `exact_basename`

Match an exact final path component using case-sensitive Git-path semantics.

### `literal_bytes`

Match a nonempty literal byte sequence within regular blob content.

Its domain explicitly consists of regular blobs. Reject empty patterns or define and test their semantics; prefer rejecting them initially.

Do not silently search symlink targets or external LFS content.

For each eligible entry, evaluation produces:

- `MATCH`
- `NO_MATCH`
- `UNEVALUATED`, with a structured reason

An oversized, unreadable, missing, or interrupted object is not a non-match.

Support scope selectors for:

- The whole manifest.
- Exact paths.
- Directory prefixes.
- An explicit object-type restriction.

Freeze the selector semantics in a versioned definition.

Do not implement semantic equivalence detection, natural-language predicates, or regular expressions in the initial release.

## 7. Evidence receipts and trust

Receipts are immutable and issued by service-controlled scanners.

Agents provide receipt references, not authoritative receipt JSON.

A receipt records or references:

- ID and schema version.
- Repository, snapshot, and manifest identity.
- Predicate and semantic version.
- Declared scope.
- Successfully evaluated entry identities.
- Matching entry identities.
- Unevaluated entries and reasons.
- Adapter and evaluation version.
- Access scope.
- Completion state.
- Parent receipts and derivation metadata.
- Timestamp.

Store large coverage sets server-side. Counts and percentages are presentation aids, not evidence.

The matching set must be complete for the entries classified as successfully evaluated.

### Trust assumptions

The service, manifest builder, and scanner belong to the trusted computing base.

Hashes establish identity and integrity; they do not prove an untrusted worker performed a search correctly.

No signature or digest may be described as proving scanner correctness.

Corrupted, incompatible, inaccessible, or missing evidence cannot support a claim.

## 8. Claims, composition, and verification

Separate:

- What was observed.
- What proposition is claimed.
- What the action policy requires.
- Whether the requester is authorized.

For snapshot `S`, define:

- `D`: required manifest entries selected by the claim scope.
- `E`: successfully evaluated entries from compatible receipts.
- `M`: matching entries from those evaluations.

The supported absence claim means:

    Every entry in D evaluates to NO_MATCH.

Support requires:

    D ⊆ E

and:

    D ∩ M = ∅

plus successful validation of evidence identity, semantics, integrity, authorization, and relevant limitations.

An empty domain may support absence only when its emptiness follows from the authoritative manifest.

### Outcomes

- `SUPPORTED`: the exact scoped absence claim is established.
- `REFUTED`: a trusted matching witness exists within the required domain.
- `UNKNOWN`: compatible evidence is insufficient.
- `INVALID_EVIDENCE`: submitted evidence is forged, malformed, incompatible, unauthorized, or inconsistent.

Validate evidence before using its conclusions.

For valid compatible evidence, a matching witness refutes absence without searching the entire domain.

Unknown is not false, and unknown must not authorize an action requiring supported absence.

### Composition rules

Require compatible repository, snapshot, manifest, predicate semantics, access scope, and trusted issuer.

Compose coverage using object identities and set operations.

Ensure:

- Order independence.
- Idempotence.
- No coverage gain from duplicate receipts.
- No inflated coverage from overlaps.
- Missing partitions remain missing.
- Root entries and dot-directories are included when required.
- Outside-scope matches do not refute a narrower claim.
- Conflicting evaluations for identical inputs trigger an integrity error.

Provide `search_missing` to evaluate the remaining permitted domain.

Return actionable gap information without revealing paths or contents outside the requester’s authority.

## 9. Incremental evidence reuse

Historical evidence remains evidence about its original snapshot.

Never relabel an old receipt as current.

To derive evidence for snapshot B:

1. Obtain B’s authoritative manifest.
2. Determine B’s required domain.
3. Identify unchanged predicate-relevant inputs.
4. Reuse compatible evaluations.
5. Evaluate added, changed, or newly relevant entries.
6. Remove deleted entries from the new domain.
7. Preserve unresolved gaps.
8. Issue a new receipt with recorded derivation.

Define predicate dependencies explicitly.

Path predicates depend on relevant path semantics and domain membership. Literal-content evaluation depends on content identity and predicate semantics.

Treat renames as deletion and addition initially.

Handle type, scope, predicate, permission, and evaluation-version changes.

New objects must be considered even when every previously observed object is unchanged.

Cached predicate results and receipt applicability are different:

- A blob evaluation may be reusable.
- That does not establish that the current required domain is fully covered.

The derived result must agree with a fresh independent evaluation for the same supported claim.

On uncertainty, fall back to fresh evaluation or return `UNKNOWN`.

## 10. Action policy

Implement one mutation:

    guarded_create

Policies are operator-defined, versioned, and immutable during a service run.

A policy specifies:

- Permitted action.
- Destination constraints.
- Required absence predicate.
- Required scope.
- Authorization requirements.
- Content and size limits.

The agent cannot choose a weaker precondition.

Include at least:

- Basename uniqueness within a declared scope.
- One exact literal-marker policy demonstrating content-based evidence.

Present these as explicit repository conventions, not semantic understanding.

Validate the candidate tree:

- Exactly one authorized file creation.
- No overwrite.
- No extra changes.
- No invalid path or parent-path collision.
- No unintended mode or type change.

Creation changes the precondition intentionally. For example, a valid absence check followed by one creation may establish a uniqueness condition afterward. Do not incorrectly require absence to remain true after publication.

Policy changes invalidate pending requests’ previous authorization assumptions. Recheck against the active policy.

## 11. Conditional publication and durable operations

Use an append-only service-managed branch.

For each action:

1. Resolve current head S.
2. Check authorization and policy.
3. Verify evidence applicable to S.
4. Construct candidate S′ from S.
5. Validate the exact candidate change.
6. Publish only if the managed head still equals S.
7. Otherwise return `STATE_CHANGED`.

Never fall back to an unconditional update or silently transplant the candidate onto another state.

### Operation identity

Require a requester-scoped operation ID and canonical payload digest.

Reuse with different content must fail.

Store an authoritative operation record with:

- Requester and operation identity.
- Payload digest.
- Policy version.
- Base snapshot.
- Candidate commit.
- Evidence references and decision metadata.

Avoid secrets or full evidence bodies in commit messages.

### Git-backed recovery

Use an appropriately tested Git reference transaction to:

- Conditionally update the managed branch.
- Create an immutable operation reference binding the committed operation.

Document how the operation metadata is stored and resolved.

SQLite may index this information, but it must not be the sole authority for whether publication occurred.

Validate the selected Git version and transaction behavior against official documentation.

Do not assume arbitrary readers observe a globally atomic snapshot of multiple references.

On retries:

- Return an existing matching operation result.
- Reject conflicting payloads.
- Reconcile interrupted operations from authoritative Git state.
- Return an explicit unresolved outcome when necessary.
- Do not blindly replay a mutation.

Test process termination before, during, and after publication.

State process-crash guarantees separately from power-loss durability. Claim only what the implementation and configuration support.

## 12. Enforcement and security boundary

The reference agent may use only the exposed tools to mutate managed state.

It must not have unrestricted access to the managed Git directory, policy store, or authoritative evidence store.

For a cooperative local integration that does allow bypass access, clearly label enforcement as conditional on using the gateway.

Implement:

- Localhost binding by default.
- API authentication.
- Per-request repository and receipt authorization.
- Explicit repository registration.
- Safe subprocess argument construction.
- Controlled Git configuration.
- No arbitrary shell execution.
- No agent-supplied Git options.
- No unexpected hooks, filters, remote fetches, or path traversal.
- Request and content limits.
- Bounded queues, workers, memory, and storage.
- Timeouts and cancellation.
- Secret-free logs.
- Safe SQLite queries.

This is a trusted-developer local tool, not a hostile multi-tenant hosting service.

Pin Git objects and evidence required by active operations.

Define retention explicitly. Do not prune operation records while promising indefinite retry recognition. Use indefinite retention for the first release or implement a clearly enforced retry window.

Evicted evidence must produce a recoverable unavailable result, never implicit approval.

## 13. Integrations and presentation

After core validation, deliver:

- A CLI.
- A documented HTTP API.
- A thin Python client.
- One tested MCP adapter.

All use the same core verification and publication logic.

Suggested operations:

- Register repository.
- Inspect snapshot.
- Search snapshot.
- Inspect receipt.
- Compose receipts.
- Verify absence.
- Search missing coverage.
- Refresh evidence.
- Guarded create.
- Inspect operation.
- Run demos and benchmarks.

Use concise structured errors and stable reason codes.

Distinguish evidence failures, positive witnesses, authorization failures, branch conflicts, and uncertain operation outcomes.

Keep large manifests and receipts out of model context unless explicitly expanded.

Do not claim universal agent-host compatibility.

Provide structured logs and a small metrics endpoint. A dashboard is unnecessary.

## 14. Required demonstrations

All demonstrations must run without a model API key.

### Incomplete search

An entry exists outside the searched partition. Show `UNKNOWN`, search missing coverage, then show `REFUTED`.

### Composition

Compose complete partitions successfully. Show that missing and duplicated partitions do not establish coverage.

### Concurrent publication

Two requests verify the same initial state. One publishes; the other receives `STATE_CHANGED`, refreshes, and observes the new match.

### Incremental reuse

Modify a small part of a fixture. Demonstrate agreement with full evaluation and report total overhead, not just reused counts.

### Forged evidence

Reject fabricated evidence and unauthorized receipt references without leaking protected information.

### Lost response

Publish an operation, lose its response, restart, and recover the same operation without a second publication.

Optional real-model demonstrations must use the same controlled environment and identify exactly what was observed.

## 15. Evaluation

Separate system correctness, performance, and agent behavior.

### Independent oracle

Implement a simple full evaluator independent of the incremental derivation code.

Ground truth must include the manifest, exact policy, and expected final repository state.

### Baselines

Compare:

A. Ordinary partial search with normal destination and concurrency checks.

B. A plus an instruction warning against assuming absence.

C. Complete deterministic precondition validation before each action.

D. Conventional memoization keyed by predicate and immutable relevant inputs, without the receipt protocol.

E. NotSoFast.

Give all action-capable baselines equivalent authorization and conditional publication safeguards.

For D and E, provide equivalent opportunities to cache, share results, and exploit known object identities. Do not manufacture an advantage by withholding information from the baseline.

Include a reject-all control when interpreting safety.

### Ablations

Using the same implementation, measure:

- Receipts without cross-snapshot reuse.
- Reuse without combining independent partial searches.
- Full NotSoFast.

This helps identify which feature contributes any benefit.

### Workloads

Include:

- Small and larger repositories within a documented laptop budget.
- Cheap path predicates and more expensive content predicates.
- Warm and cold caches.
- Low and high mutation rates.
- Repeated and one-off queries.
- Overlapping and missing partitions.
- Interrupted evaluation.
- Permission restrictions.
- Branch contention.
- Workloads where full validation is simpler or faster.

Do not create an enormous full-factorial benchmark. Choose and document a manageable set of representative cases.

### Metrics

Report:

- Incorrectly admitted actions.
- Incorrectly blocked valid actions.
- Legitimate task completion.
- Recovery from unknown evidence.
- Objects evaluated.
- Bytes processed.
- Manifest, derivation, and receipt-management cost.
- Verification and total latency.
- CPU, memory, storage, and cache growth.
- Tool calls and bytes returned to the agent.
- Actual model usage when available.

Report denominators and distinguish unknown, refuted, failed, and infrastructure-error outcomes.

Evaluation-count savings are not automatically speed, token, or monetary savings.

Do not divide by a zero baseline; report not applicable.

### Model experiments

Keep model experiments optional when credentials are unavailable, but deliver a runnable harness.

Use matched tasks, tool constraints, budgets, and model configurations. Disclose trial counts and variability.

Do not call scripted demonstrations model evaluations.

A blocked bad action alone is not task success. The agent must finish correctly or accurately identify an unsatisfied condition.

## 16. Tests and correctness argument

Test:

- Empty and incomplete manifests.
- Complete and partial scopes.
- Root entries and dot-directories.
- Unusual paths.
- Duplicate and overlapping coverage.
- Predicate and snapshot mismatches.
- Matching witnesses inside and outside scope.
- Missing or oversized objects.
- Interrupted and cancelled evaluations.
- Forged or unauthorized receipts.
- Adds, deletes, renames, and type changes.
- Incremental/full-evaluation equivalence.
- Concurrent publication.
- Conflicting operation IDs.
- Crash recovery.
- Eviction and retention.
- Policy changes.
- SDK and MCP integration against the real service.

Use property-based testing or fuzzing where it meaningfully checks set algebra, path handling, or incremental equivalence.

Run Go’s race detector.

Provide a short soundness argument under explicit assumptions:

- Complete authoritative manifest.
- Correct deterministic evaluation.
- Authentic receipts.
- Correct set operations.
- Correct policy binding.
- Conditional publication against the verified state.

Call this a correctness argument, not a machine-checked proof.

Tests showing zero failures establish results for those tests, not universal bug freedom.

## 17. Build gates

### Gate 1 — Small working core

Manifest, exact-basename evaluation, receipt, verifier, guarded creation, and conditional publication.

Complete a deterministic end-to-end demonstration.

### Gate 2 — Composition and reuse

Add partition composition, gap recovery, remaining predicates, incremental derivation, and independent equivalence tests.

Run the early comparison against full evaluation and conventional memoization.

### Gate 3 — Operational correctness

Complete durable operation handling, crash tests, authorization, limits, retention, and security checks.

### Gate 4 — Thin integrations

Add HTTP, Python, MCP, packaging, and actual integration tests.

Do not duplicate core logic.

### Gate 5 — Release evidence

Finish demonstrations, benchmarks, audit, documentation, and CI.

The gates control sequencing. They are not permission to abandon required release work.

If a result invalidates a performance hypothesis, revise the claims and report the result; do not conceal it or expand scope to distract from it.

## 18. Documentation and audit

Deliver:

- README with clean-start instructions.
- Architecture diagram.
- Protocol and API reference.
- Predicate and scope definitions.
- Trust and threat model.
- Guarantee matrix.
- Recovery and retention documentation.
- Benchmark scripts and actual results.
- Related-work comparison with verified primary sources.
- `AUDIT.md`.
- Docker Compose and CI.
- Appropriate licensing and dependency notices.

For every guarantee, state:

- What is established.
- Required assumptions.
- Verification evidence.
- Failure behavior.
- Excluded cases.

Audit the actual implementation for correctness, concurrency, durability, security, performance, and documentation accuracy.

Each finding needs severity, evidence, impact, resolution, and remaining limitation.

An architecture review is not a completed code audit.

## 19. Completion criteria

The project is complete when:

- Clean-start instructions work.
- All six model-free demos run.
- Supported predicates and policy behavior are implemented.
- Composition and incremental reuse match the independent oracle.
- Concurrent publication and retry recovery have been exercised.
- Integrations work against the actual core.
- Security and retention boundaries are implemented.
- Benchmarks include optimized memoization and unfavorable cases.
- The audit is complete.
- No advertised core feature is a stub.
- Claims match measured results and implemented guarantees.

If environment restrictions block validation, identify the exact unverified checks. Never claim they passed.

The final report must include:

1. What was built.
2. How to run it.
3. Tests actually executed.
4. Measurements and tradeoffs.
5. Guarantees and limitations.
6. Unverified items.
7. Documentation and audit locations.
8. Two resume bullets based only on implemented capabilities and verified results.

## Final project description

NotSoFast is a reusable evidence layer for supported agent actions, implemented first over immutable Git snapshots. It records exact search coverage, composes trusted evidence, verifies scoped absence claims, reuses unchanged evaluations, and conditionally publishes changes against the verified state.

Its value must be demonstrated through correct behavior, useful recovery, and fair comparisons—not through an inflated feature list or an assumed novelty claim.