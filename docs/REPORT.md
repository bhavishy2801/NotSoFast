# Delivery report

## 1. Built

Go core and CLI; immutable Git imports/manifests; exact path, basename and literal-byte predicates; SQLite-backed immutable receipts; coverage composition and missing-search recovery; incremental reuse using ordinary memoization; conjunctive operator policies; authenticated bounded HTTP; thin Python client and stdio MCP; conditional one-file publication with Git intent/operation records, policy epochs and retry recovery. Docker Compose and GitHub Actions definitions are included.

## 2. Run

From the project root, with Go, GCC, Git and Python installed:

```sh
go build -o .tools/nsf ./cmd/nsf
python python/demo.py
```

On Windows use `.tools/nsf.exe` as the output name. The already-built local executable is there. [README](../README.md) contains clean-start, CLI, HTTP, MCP and configuration instructions.

## 3. Verification

Executed Go vet, the full Go race-detector suite, scope fuzzing (26,884 executions in the recorded five-second-budget run), actual CLI/HTTP/Python/MCP integration, six model-free demos, scripted action controls, incremental/oracle equivalence, concurrent publication, payload conflict, process-crash/restart, partial-ref recovery, SHA-256 empty manifests, policy/repository revocation, corruption and retention-boundary tests. Raw [Go results](test-results.txt), [fuzz results](fuzz-results.txt) and [demo results](demo-results.json) are included. Benchmark tests are opt-in and were also executed separately.

## 4. Measurements and tradeoffs

The final matrix contains 64 measurements; the larger filename case and three ablations are separate. Ordinary memoization and receipts have identical evaluation-count savings in the measured unchanged/lightly/heavily changed content cases. No receipt-specific speed advantage is established. For 4096 cold filename entries, fresh validation took 347.52 ms versus 2293.74 ms with receipts. Parent composition adds validation cost in the small ablation. [Evaluation](EVALUATION.md) gives raw data, methods, denominators and limits.

The scripted invalid/valid pair produced no incorrect admission and completed the valid creation under C/D/E; A/B admitted the duplicate, while reject-all blocked the valid task. These are two fixtures per baseline, not model behavior or a universal safety result.

## 5. Guarantees and limits

Exact scoped tracked-tree absence under a trusted scanner/store; conditional single-file publication against the verified head and active configuration; requester-scoped operation identity and Git-backed retry recognition. No semantic equivalence, arbitrary external-API enforcement, hostile multitenancy, same-user filesystem isolation or power-loss durability is claimed. Pending uncertain intents remain UNRESOLVED; stale Git locks may require operator repair. [Guarantee matrix](ARCHITECTURE.md) and [recovery documentation](RECOVERY.md) state assumptions and failure behavior.

## 6. Unverified items

Docker is unavailable on the development host; container execution and remote CI have not run. No actual model provider/credentials were used; only the provider-neutral harness contract was tested. Combined-process CPU, peak RSS, physical cold-cache runs, statistical latency variability, power-loss behavior and broad MCP-host compatibility remain unverified. Profiles and Go allocation/storage measurements are supplied without mislabeling them as those missing measurements.

## 7. Audit and documentation

[AUDIT.md](../AUDIT.md), [protocol](PROTOCOL.md), [architecture](ARCHITECTURE.md), [recovery](RECOVERY.md), [evaluation](EVALUATION.md), [related work](RELATED_WORK.md), [requirements](SPEC.md), [execution record](PLAN.md), MIT license and dependency notices.

## 8. Resume bullets

- Built a Go/Git/SQLite evidence gateway with exact-scope verification, receipt composition, incremental reuse and authenticated CLI/HTTP/Python/MCP access; validated six model-free end-to-end demonstrations.
- Implemented conditional Git publication and durable operation recovery with policy epochs; exercised race detection, crash/retry scenarios and a 64-case performance comparison against fresh validation and conventional memoization.
