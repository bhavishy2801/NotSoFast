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

Docker is unavailable on the development host. Earlier remote CI run [37277875527](https://github.com/bhavishy2801/NotSoFast/actions/runs/37277875527) passed at commit `4b906a3`, including image build; newly added runtime/browser CI steps have not been remotely verified. No real-model inference or private-account OAuth authorization was performed. Physical cold-cache runs, power-loss behavior and broad agent-host compatibility remain unverified.

Five repeated 32-file matrices now include Windows Job CPU/process totals and committed-memory peaks, plus sampled aggregate RSS. These are not exact RSS peaks, controlled latency distributions or per-action CPU measurements. See [raw repeated results](repeated-results.json). Official Python MCP SDK interoperability and real Edge/Chrome desktop workflows were exercised locally.

## 7. Audit and documentation

[AUDIT.md](../AUDIT.md), [protocol](PROTOCOL.md), [architecture](ARCHITECTURE.md), [recovery](RECOVERY.md), [evaluation](EVALUATION.md), [related work](RELATED_WORK.md), [requirements](SPEC.md), [execution record](PLAN.md), MIT license and dependency notices.

## 8. Resume bullets

- Built a Go/Git/SQLite evidence gateway with exact-scope verification, receipt composition, incremental reuse and authenticated CLI/HTTP/Python/MCP access; validated six model-free end-to-end demonstrations.
- Implemented conditional Git publication and durable operation recovery with policy epochs; exercised race detection, crash/retry scenarios and a 64-case performance comparison against fresh validation and conventional memoization.


## Desktop and browser delivery — October 6

Added an embedded animated local UI, Windows portable executable and browser launcher, secure session reconnect, recent workspaces, GitHub URL imports with official CLI OAuth device flow, safe file preview/download, command palette, evidence composition, guarded publication/recovery, ZIP export, local/provider model setup and isolated model trials. The complete user guide is now in [README](../README.md).

GitHub imports use bounded default-branch history; helper-process cancellation and rejected-import cleanup were reviewed. The frontend is embedded and has no runtime frontend dependencies. Public GitHub import was tested against a real repository. Packaged browser checks use a restricted executable PATH to exercise bundled runtimes.

### Account, theme and installer extension

Added five accent palettes with light/dark/system modes; local profiles, draft persistence, bookmarks and insights; optional Supabase PKCE accounts and explicit portable saves; SQL ownership policies and a configuration guide. Live hosted OAuth/RLS remains pending project configuration. No repository evidence or credentials are uploaded in cloud profile snapshots.

The native Windows installer includes bundled runtimes, Start menu/optional desktop shortcuts, upgrades and an uninstaller that retains app data. Install/upgrade/launch/uninstall and junction rejection passed. The final race suite passed core (135.099s), desktop (32.046s) and protocol (1.930s); vet passed. Browser evidence is in `browser-followup-results.json`. The package is not code-signed.
