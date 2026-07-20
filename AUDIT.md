# Implementation audit — 2026-10-05

Scope: Go core, SQLite store/cache, controlled Git commands, CLI/HTTP, Python client, stdio MCP, tests and documentation. A fresh reviewer inspected the actual implementation; findings were reproduced or traced before fixes. This is an implementation review, not a penetration test, dependency vulnerability audit or machine-checked proof.

| Severity | Finding and evidence | Impact | Resolution | Remaining limitation |
|---|---|---|---|---|
| High | Selecting `q.Policy` could bypass an overlapping basename convention; `TestCannotSelectWeakerPolicy` reproduced it | Duplicate creation through a weaker marker policy | `core/policy.go` requires all matching destination policies; every receipt validated first | Operator must configure meaningful conventions; no semantic equivalence |
| High | Candidate manifest limits were not checked before publication | A valid creation could make subsequent scans exceed MaxEntries | Candidate manifest rebuilt/validated before intent/publication; `TestCandidateCannotExceedManifestLimit` | Large legal states still cost a full enumeration |
| High | Multi-reference transaction readers/crashes can observe partial publication | A lost response could look unrecorded | Durable prepublication intent pins metadata/candidate; ancestry plus operation binding reconciles branch-first state | Pending ambiguous intents fail UNRESOLVED; operator availability repair may be necessary |
| High | Identical trees/base/timestamp could alias candidate commits between operations | Intent recovery could attribute another operation's commit | Candidate message binds requester/payload digest; recovery checks message and exact parent/base | Assumes intact service-owned Git storage; no malicious-store protection |
| High | Orphan Git child after parent death could outlive a policy reload | Publication under stale grants/policy | Active policy/grant ref verified in same publication transaction; activation fails closed | Startup may block on stale/prepared Git locks; no forced lock deletion |
| High | Removed repositories initially retained an old epoch | Pending orphan publication could outlive repository revocation | Epoch includes allowed IDs; startup activates it in every existing managed repository, including removed IDs; regression tested | State directories must remain protected from external replacement |
| Medium | Registration retry could skip epoch activation after an interrupted first import | Registration returned success but creation stayed unavailable | Both idempotent import paths activate policy before returning; regression tested | Failed startup/registration still requires resolving genuine Git lock/I/O errors |
| Medium | Cache-row corruption could be trusted as a negative result; `TestCacheCorruptionCannotApprove` reproduced it | Incorrect absence conclusion | Integrity digest per evaluation row; incompatible value fails closed | Hashes do not protect against a malicious authoritative-store writer |
| Medium | Second process could open the same service state | Conflicting local configuration and unbounded work | Lifetime SQLite exclusive lease; crash releases OS lock | Git children need their own transaction checks, supplied by head/epoch verification |
| Medium | One fixed pre-scan reserve did not bound database/import expansion | Disk pressure could exceed intended admission budget | SQLite page cap, receipt cap, larger reserve, import count/expanded-byte preflight, pack cap | Strict physical disk/RSS ceilings require OS quota; external writers excluded |
| Medium | Source repository configuration could permit lazy remote object fetch | Unrequested external access | Sanitized Git environment, GIT_NO_LAZY_FETCH and empty GIT_ALLOW_PROTOCOL | Trusted installed Git executable/source remain TCB |
| Medium | Windows long paths made operation-ref transaction fail in a long test directory | Valid operation stuck unresolved | `core.longpaths=true`; failure reproduced then test passed | Unsupported source path encodings still fail explicitly |
| Medium | Early HTTP authentication rejection could reset a Windows client connection with an unread request body | Client saw a transport failure instead of the stable rejection code | Bounded deferred body drain; real Python HTTP regression rerun | Oversized or broken transports can still fail without a complete response; mutations must use operation recovery |
| Low | Receipt timing called pre-save time “total” and exposed a zero persistence field | Misleading overhead claims | Renamed boundary to scan_before_persistence_ns; external measured persistence/total in benchmarks | Fine-grained hashing/DB CPU not fully isolated |
| Low | Early benchmark carried source mutations between predicate cases | “Light” content case actually changed many inputs | Reset fixture before each predicate, reran stored measurements | Single trials/fixed order; no statistical latency claim |
| Performance | Cold blob searches launch two Git processes per object | High latency on many content objects | Reported openly; conventional memoization retained | Batched Git reads are future optimization, not an advertised capability |

## Verification and boundaries

Tests use actual Git and SQLite, including independent full evaluation, changed source imports, one-file deltas, policy binding, conflicts, cache/receipt corruption, cancellation, resource rejection, API authentication and subprocess recovery. Python tests connect to the actual compiled service. The latest saved test log and evaluation outputs are linked from [EVALUATION.md](docs/EVALUATION.md).

Process-kill attempts after commit input observed completed publication on this host; partial-ref states were also constructed deterministically to verify recovery. Do not claim every kernel/filesystem interruption was observed. No power-loss guarantee is made. No automated lock cleanup, pending-intent abort API, arbitrary mutation or external publication exists. The desktop now imports GitHub HTTPS repositories; the agent-facing core still does not accept arbitrary remote URLs.

Docker Compose and GitHub Actions definitions are supplied. Docker is absent locally. Earlier remote CI run 37277875527 passed at commit 4b906a3, including image build. The newly added runtime container/browser checks have not run remotely; local container execution remains unverified. Real-model runs are optional and were not performed. The harness requires a provider driver; its transport contract was tested with a non-model script.

The repository permission boundary is whole-repository authorization; path-level ACLs and hostile multitenancy are not implemented. A local same-user CLI/agent can bypass the gateway. Service-created files are ordinary 100644 blobs; symlink targets, submodule contents, LFS payloads and semantic equivalence are outside the authoritative universe.

No unresolved finding above is silently advertised as a stronger guarantee. Remaining limits are explicit product or validation boundaries.


## Desktop follow-up — October 6

- Session/workspace capabilities bind browser requests to the active local workspace; stale-window operations fail before dispatch. Secure-link hash changes reconnect an already-open tab.
- File previews authenticate immutable snapshots, restrict tracked paths and blob types, cap content at 1 MiB and render as text. Symlinks are not followed.
- GitHub URL parsing accepts only github.com HTTPS owner/repository paths. Official CLI OAuth credentials stay outside browser JavaScript; app-specific configuration is isolated, while OS credential storage remains an upstream CLI behavior.
- GitHub imports disable hooks, credential helpers and redirects, impose time/size limits, cancel Git helper processes and clean rejected untracked imports. Full default-branch history preserves parent connectivity. The temporary 96 MiB watchdog is sampled and can overshoot before cancellation; it is not a filesystem quota.
- Desktop metadata has a 64 MiB database-page bound; PENDING operations remain visible beyond the recent completed-event window. Imported workspaces have no aggregate retention policy.
- Model endpoints require HTTPS except loopback; redirects/proxy use are disabled, keys remain in memory, and trials use separate disposable fixtures. Scripted transport success is not evidence of real-model performance.
- Packaged Edge/Chrome workflows, real public GitHub import, race tests and independent review supplement the original core checks. Private OAuth approval and hardware power-loss testing remain unverified.

## Account and installer extension

Cloud OAuth uses PKCE S256, expiring one-use callback state, HTTPS Supabase project validation, and server-memory-only tokens. Public configuration rejects service-role/secret keys. Supplied SQL enables per-user row policies; response ownership is also checked locally. The integration contract is tested, while live OAuth and deployed RLS are not yet verified. Cloud saves are explicit and include draft text and GitHub bookmark URLs; they are not end-to-end encrypted.

The installer uses per-user privileges and manifest-based deletion. Review found a junction traversal risk; cleanup now validates path ancestors and the complete installation tree before changes. A real junction regression confirms rejection before deleting files. Same-user concurrent filesystem tampering is not treated as an isolation boundary. Uninstall retains application data, and setup logs failures to `%TEMP%/NotSoFast-Setup.log`. Binaries are unsigned.
