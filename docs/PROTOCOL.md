# Protocol v1

HTTP: `POST /v1/<operation>` with one JSON object and a Bearer token. Success: `{"result": ...}`. Failure: `{"error":{"reason":"CODE"}}`. Unknown fields and trailing JSON are rejected. CLI uses the identical operation and request body, returning the result directly. `-expand` or HTTP `?expand=1` includes large manifests/evaluation sets. Authenticated `GET /metrics` reports request, rejection and active-request counters.

## Manifest, filename and universe semantics

Identity includes repository ID, full commit ID, full tree ID, Git hash algorithm, manifest version 1 and SHA-256 canonical manifest digest. The canonical digest is SHA-256 of Go JSON encoding of `{Version:1, Entries:[...]}` with entries sorted lexicographically by UTF-8 path bytes. Each entry carries path, mode, Git object ID and type. NUL-delimited Git enumeration preserves tabs/newlines/spaces and Unicode; invalid UTF-8, backslashes, empty/dot/parent components, leading slash, paths longer than 4096 bytes or more than 256 components fail explicitly. Git paths are case-sensitive without Unicode normalization.

The universe includes root entries, dot-directories, recursive directory entries, regular blobs, symlink entries and gitlink entries. It excludes the source working tree, untracked files, symlink targets, submodule contents, runtime/external data and Git LFS payloads. A stored LFS pointer is just regular blob content. Missing objects or incomplete enumeration prevent a complete manifest.

## Predicates and selectors

Predicate shape: `{"kind":"exact_basename","value":"ZGF0YWJhc2UueWFtbA==","version":1}`. `value` is standard Base64 bytes, nonempty and at most 65,536 bytes.

| Kind | Domain | Match |
|---|---|---|
| exact_path | All selected entry types | Exact repository-relative path |
| exact_basename | All selected entry types | Exact final path component |
| literal_bytes | Selected regular blobs only (100644/100755) | Nonempty literal byte substring |

Scope shape: `{"paths":["root.txt"],"prefixes":["src"],"type":"regular"}`. Omitted/empty path and prefix lists select the whole manifest. Paths and prefixes form a union, then `type` intersects it. Prefix `src` selects descendants `src/...`, not the directory entry `src` itself or `src2/...`; prefix `""` selects all. An exact path can select a directory entry without its descendants. Types are regular/tree/symlink/gitlink. Literal predicates always intersect regular blobs regardless of selector. Selector semantics are version 1, bound through receipt schema/adapter version 1.

Each eligible entry yields MATCH, NO_MATCH or UNEVALUATED with a reason. Counts are summaries, not proof. Oversize/unreadable content never becomes NO_MATCH. A cancelled request fails without issuing a new receipt; existing parent receipts remain available.

## Operations

| Operation | Request fields | Result |
|---|---|---|
| register | repository, snapshot | Imports selected commit from configured local source; first import initializes managed branch, later imports pin separate snapshots without moving it |
| head | repository | snapshot: managed commit ID |
| snapshot | repository, snapshot | snapshot metadata; entry_count; explicit universe description |
| search | repository, snapshot, predicate; optional scope, parents, fresh | receipt summary and evaluation_count |
| refresh | Same as search; parents references old evidence | Newly issued receipt for target snapshot |
| receipt | id | Authorized immutable receipt summary |
| verify | repository, snapshot, predicate; optional scope, receipts | Outcome, reason, required/evaluated, gap and witness counts/lists |
| compose | Same as verify | New receipt containing compatible coverage union; no scanning |
| search_missing | Same as verify | New receipt carrying previous successes and newly evaluated gaps |
| guarded_create | repository, snapshot, operation, policy, policy_version, path, content, receipts | Durable published operation |
| operation | repository, id | Matching requester's durable operation, or explicit unavailable/unresolved error |

Search parents must match repository, predicate semantics and current access identity; their original snapshots stay unchanged. Path predicate cache keys contain relevant path/mode and predicate version; content keys contain blob ID/mode and predicate version. Access identity and evaluation version are always included. Renames are new entries; content evaluation may still be reused from the same immutable blob. Deletes disappear from the new domain; scope/type changes recompute membership. Ordinary cache reuse and coverage applicability are separate checks.

Receipts record schema/adapter versions, repository/snapshot/manifest, predicate/scope, access identity, completion, evaluated identities/results/reasons, parent references, derivation, timestamp and scanner statistics. IDs are digests of canonical stored receipt JSON excluding its ID. Store modifications or missing references cannot support claims. `scan_before_persistence_ns` deliberately excludes receipt serialization/storage; end-to-end latency must be measured outside the call. The benchmark does so.

## Outcomes and errors

SUPPORTED: exact absence established. REFUTED: valid in-domain matching witness. UNKNOWN: missing successful coverage. INVALID_EVIDENCE: malformed, inaccessible, missing, inconsistent or incompatible references. All submitted evidence is validated before a witness is used. UNKNOWN does not authorize creation.

HTTP uses 401 for missing/invalid authentication, 403 for forbidden repository/browser origin, 409 for STATE_CHANGED/OPERATION_CONFLICT/POLICY_CHANGED, 429 for BUSY, 413 for request size, 507 for resource ceilings, 503 for UNRESOLVED, 408 for cancellation, and 400 for other rejected proposals/evidence. No filesystem paths, bearer tokens, content or receipt bodies appear in service logs.

## Worked requests

Search `src/` for basename `database.yaml`:

```json
{"repository":"demo","snapshot":"FULL_COMMIT_ID","predicate":{"kind":"exact_basename","value":"ZGF0YWJhc2UueWFtbA==","version":1},"scope":{"prefixes":["src"]}}
```

Verify whole-manifest absence using returned receipt ID:

```json
{"repository":"demo","snapshot":"FULL_COMMIT_ID","predicate":{"kind":"exact_basename","value":"ZGF0YWJhc2UueWFtbA==","version":1},"receipts":["RECEIPT_ID"]}
```

Send the same body to `search_missing` for recovery. For a legitimate absent basename `fresh.txt`, obtain its evidence and propose:

```json
{"repository":"demo","snapshot":"FULL_COMMIT_ID","operation":"request-001","policy":"unique","policy_version":"1","path":"fresh.txt","content":"aGVsbG8=","receipts":["FRESH_RECEIPT_ID"]}
```

Operation identity is requester-scoped within a registered repository. The canonical digest covers the decoded Go request JSON, with receipt references sorted. Different bytes, destination, policy version, base or receipt reference multiset conflict under the same ID. Duplicate references do not increase coverage. Never retry a changed payload under an old operation ID.
