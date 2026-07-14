# Recovery and retention

The authoritative state is Git, not SQLite's opinion of whether a write happened. Each candidate commit message contains only a digest binding requester and canonical request payload. Recovery verifies that binding and the candidate's single parent/base. Two distinct operations cannot accidentally share a candidate just because they created identical trees during the same timestamp second.

Before publication, the service creates an immutable `refs/notsofast/intents/<hash>` pointing to a metadata commit. Its tree contains `operation.json` and the active operator `policy.json`; its parent pins the candidate. Metadata includes requester/operation identity, payload digest, policy-set digest/version, base, candidate, receipt IDs and decisions. No secrets or full evidence bodies appear in commit messages.

Publication uses `git update-ref --stdin`: start; conditional managed-head update; immutable operation-reference creation; verification of the active policy/grant epoch; prepare; commit. The official [Git reference transaction documentation](https://git-scm.com/docs/git-update-ref) describes the conditional and transaction commands. Arbitrary readers are not promised a globally atomic view of multiple references.

On restart, operator policy/grant activation must succeed before the service opens. Old orphan Git children verify the old epoch and cannot publish after a different epoch has activated. If an old child already holds the epoch lock, activation fails rather than silently accepting new configuration. Completion under the old epoch before activation remains an old-policy operation.

The epoch includes the allowed repository ID set and is updated in all existing managed repositories, including removed ones. Removing an ID therefore revokes its pending publication authority. Idempotent registration retries also restore/check the active epoch before reporting success.

## Retry outcomes

- Operation ref present and candidate in managed history: return the original matching result.
- Operation ref absent, durable intent present, uniquely bound candidate in managed history: return the original result. This handles branch-first partial publication.
- Intent/operation present but candidate not in managed history, or integrity checks fail: `UNRESOLVED`. Do not replay the mutation or silently transplant it.
- Same requester/ID with a different payload digest: `OPERATION_CONFLICT`.
- No record and changed managed head: `STATE_CHANGED`; refresh evidence and use a new operation ID for a new proposal.

An operation that has never been issued returns NOT_FOUND. Network failure is not proof that publication failed; inspect/retry the same identity first. SQLite can be restored/rebuilt as an index/cache; deleting receipts makes evidence unavailable, but Git operation recognition remains authoritative. Successful retries still require current requester write authority, but do not perform another action or require the old evidence to remain available.

## Process crashes versus power loss

Tests exercise abrupt process exit before/after publication, killing Git after prepare, and kill attempts immediately/1 ms/20 ms after sending commit. In the observed commit-kill trials, publication had already advanced the branch. Synthetic partial-reference states separately exercise both recovery branches; the experiment does not claim to have observed every possible interrupted rename sequence.

SIGKILL/TerminateProcess can leave Git `.lock` files. Stop the service, confirm all of its Git children have exited, back up the entire state directory, and inspect refs/intents before operator repair. Never automatically delete locks based only on age. A blocked startup or UNRESOLVED response is safer than concurrent repair. Do not reset the managed branch, remove operation/intention refs, or replay a different operation to work around uncertainty.

Pending intents are retained and deliberately fail closed. There is no automatic administrative abort/replay endpoint; an operator must inspect the actual Git state and resolve availability separately. The service does not claim rollback of interrupted external readers or guaranteed progress after disk/lock failure.

SQLite uses WAL with FULL synchronization. That does not make SQLite and Git a shared transaction, nor establish power-loss durability for the whole system. No power-cut, filesystem-loss or hardware-cache test was performed. Git fsync/storage configuration is not elevated into a durability claim.

## Retention

Receipts, cached evaluations, imported snapshot pins, managed history, intents and operation records are retained indefinitely. Managed creation is append-only. Metadata commits pin candidate/base/policy objects; snapshot refs pin imports. No GC, pruning, TTL or retry expiry is scheduled. Resource exhaustion returns a limit error; increase storage capacity or archive the entire service state with its retry-history responsibility. Do not prune records while promising indefinite retry recognition.

If an operator removes a receipt, its references yield INVALID_EVIDENCE (unavailable), never implicit approval. A fresh authorized scan recovers coverage; historical operation lookup still uses Git. Preserve the complete state directory and private configuration in backups.
