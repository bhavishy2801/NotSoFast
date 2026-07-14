# Evaluation

Measured on 2026-10-05, Windows/amd64, Go 1.27.1, Git 2.55.0.windows.3, SQLite through go-sqlite3 v1.14.32. These are local fixture measurements, not a claim about every laptop or real-world repository. The final matrix contains 64 single observations: 32/256 entries × basename/content × cold/unchanged/one-object-change/75%-change × four methods. Files contain about 4 KiB of unique content. A separate 4096-entry filename case checks a larger cheap workload. This deliberately avoids an enormous factorial experiment.

## Performance conclusion

**No receipt-specific performance advantage is established.** Conventional memoization remains the evaluation cache. Content reuse can avoid substantial repeated Git subprocess work; memoization and receipts obtain the same evaluation-count savings. Cheap filename predicates can be faster with fresh validation. Receipt value demonstrated here is composable coverage, explainable UNKNOWN and recovery, not a new caching technique.

Selected final 256-entry observations, milliseconds (one trial each; no confidence interval):

| Workload | Fresh | Per-input memoization | Receipts | Whole-query memoization |
|---|---:|---:|---:|---:|
| Cold filename | 341.37 | 452.43 | 473.40 | 572.94 |
| Unchanged filename | 293.02 | 334.46 | 332.42 | 319.09 |
| Cold content | 37059.15 | 32599.30 | 32739.56 | 32675.09 |
| Unchanged content | 30382.27 | 294.49 | 308.48 | 288.14 |
| One content object changed | 32988.87 | 478.67 | 452.49 | 505.10 |
| 75% content changed | 32037.15 | 24772.85 | 24638.48 | 24357.52 |

For unchanged content, D and E both evaluated 0/256 objects; after one changed blob both evaluated 1/256, reusing 255; after heavy change both evaluated 192/256. Small latency differences between D/E are not statistically supported wins. Cold-content costs are dominated by launching Git for each blob's size/content, not an optimized in-process substring implementation. Batched blob reads are a reasonable future optimization if these costs matter; no feature was added to rescue a performance claim.

The separate 4096-entry cold filename run measured 347.52 ms fresh, 2287.15 ms memoized and 2293.74 ms with receipts. Each starts with an empty evaluation cache. This is an explicit unfavorable case: caching thousands of cheap comparisons costs more than doing them.

## Fairness and timing boundaries

Fresh and memoized methods use the same authoritative manifest path and deterministic evaluator. Per-input memoization uses the same indexed SQLite cache keys, content IDs, access identity and evaluation version as receipts. Each method starts from a restored identical cache. Whole-query memoization additionally retains completed immutable query identities on unchanged snapshots, while still checking the authoritative manifest. Full-query identity information is not withheld from the baseline.

Cold means an empty **application evaluation cache**, not a flushed OS/disk/Git cache. The execution order is fixed. Other development checks sometimes ran concurrently. These facts, single trials, subprocess overhead and the synthetic repository sizes limit causal claims. One-off cold cases and heavy mutation show unfavorable workloads. Repeat runs and workload-specific profiling are required before selecting an optimization based on latency.

Raw rows include end-to-end scan latency, manifest cost, cache lookup, measured receipt persistence tail, evaluated/reused objects, processed bytes, Go allocation increments and total state-directory size. Hashing and database operations are included in total time; they are not all individually isolated. Receipt tail is measured outside the call and includes serialization, ID hashing and insertion; scanner statistics explicitly stop before persistence. Manifest time includes identity checks, enumeration and hashing. Comparisons do not divide by zero or translate evaluation savings into tokens/money.

[CPU profile](benchmark-cpu.txt) and [allocation profile](benchmark-memory.txt) were collected during the 392.79-second matrix. The Windows Go CPU profile attributes most samples to cgocall/syscall paths and reports 922.69 sampled seconds across threads; it is **not** a reliable combined Go+Git process CPU budget. The allocation profile reports about 719.81 MB cumulatively allocated across the full run, not peak RSS. Per-case allocations/storage are in raw rows. Combined-process CPU, peak resident memory, physical cold-cache behavior and broad machine variability remain unmeasured.

## Composition/reuse ablations

[Ablations](ablation-results.json) use the same service with partial searches over a four-blob snapshot followed by one added matching blob. Without cross-snapshot reuse, 5 entries were evaluated. Both reuse without composing parent receipts and full parent composition evaluated 1/reused 4. All results matched REFUTED. Full parent validation took more time in this small workload than ordinary cached reuse. This is evidence against assuming composition always speeds a query; its benefit is retaining explicit provenance and recovering incomplete coverage.

## Scripted action controls

[Action results](action-results.json) contain two trials per baseline: one invalid duplicate basename, one valid absent basename. All action-capable controls use the actual gateway's same authorization, destination validation, operation identity and conditional publication. A/B are operator-configured to require destination-only absence; C/D/E require the full convention. No agent can alter that configuration.

| Control | Incorrect admissions / 1 invalid | Incorrect blocks / 1 valid | Valid task completion / 1 |
|---|---:|---:|---:|
| A: partial-search choices | 1 | 0 | 1 |
| B: warning, same scripted choices | 1 | 0 | 1 |
| C: fresh complete validation | 0 | 0 | 1 |
| D: conventional memoization | 0 | 0 | 1 |
| E: evidence plus missing-coverage recovery | 0 | 0 | 1 |
| Reject all | 0 | 1 | 0 |

C/D/E identified the invalid task as REFUTED; reject-all did not establish the reason. E recovered UNKNOWN in both tasks. No infrastructure-error outcomes occurred in this recorded run. B is a scripted warning control and measures **no behavioral effect of language instructions**. C/D use receipt transport to retain identical publication safeguards; this table is not the isolated receipt-overhead benchmark. There were zero model calls.

The six [demonstrations](demo-results.json) and core tests cover overlapping/missing partitions, interruptions, repository permission restrictions, contention, unusual names, empty/type-restricted domains, corrupted/evicted evidence, changed snapshots and lost responses. Successful rejection alone is not task completion.

## Reproduction

```sh
NSF_BENCH=1 NSF_BENCH_OUT=../docs/benchmark-results.json go test ./core -run TestEarlyBenchmark -count=1 -timeout 20m
NSF_BENCH=1 go test ./core -run 'TestAblations|TestLargerFilenameWorkload' -count=1 -timeout 5m
python python/evaluate.py
python python/demo.py
```

PowerShell: set `$env:NSF_BENCH='1'` and `$env:NSF_BENCH_OUT='../docs/benchmark-results.json'` before the Go commands. Delete those environment variables afterward to keep ordinary tests short. Raw [early gate results](early-results.json) predate integration/hardening; [final matrix](benchmark-results.json) and [larger filename case](larger-results.json) are separate files. The first experimental fixture had carry-over across predicates; it was corrected and the stored early results rerun before conclusions were written.

## Optional model trials

`python python/model_harness.py` reports NOT_RUN without a driver. To run explicitly:

```sh
python python/model_harness.py --trials 3 --budget 8 --configuration '{"model":"your-model","temperature":0}' --driver your-model-driver
```

The provider-neutral driver reads one JSON object containing messages/tools/configuration and returns an assistant message in chat tool-call format plus optional usage and `model_calls`. It owns credentials and provider API transport; no provider SDK or paid call is required by the project. Unknown usage is null, not estimated. The harness preserves matched tasks, model configuration and tool budgets across A–E, logs complete transcripts, outcomes, transport counts and timing, and exposes only controlled search/create tools. Report language-only claims of an unsatisfied condition by reviewing those transcripts; no LLM judge is used. A driver-protocol test uses a script and is explicitly not a model experiment.

Local driver runs share the host account and are cooperative; use isolated clients without the service state mounted for enforcement. No model trial, cross-host MCP compatibility test, Docker run or power-loss experiment is included in the reported results.
