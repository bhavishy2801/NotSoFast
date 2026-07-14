# NotSoFast

Reusable evidence for exact absence claims over immutable Git snapshots. A Go core records search coverage, composes trusted receipts, recovers missing coverage, reuses deterministic evaluations, and conditionally creates one file on a managed branch.

**Example:** `config/database.yaml` exists. Searching `src/` does not establish that the basename is absent. Verification returns `UNKNOWN`; searching the missing domain finds the file and returns `REFUTED`. Ordinary destination-only overwrite checks would have allowed a second `database.yaml` at the root.

This enforces explicit naming/content conventions. It does not detect semantic equivalence, establish general agent safety, or protect operations that bypass the gateway.

## Quick start

Requirements: Go 1.24 or newer, a C compiler for SQLite/Go's race detector, Git supporting reference transactions (tested with 2.55.0.windows.3), and Python 3.10+. No model credentials, GPU, or paid service is needed.

From a clean checkout:

```sh
go mod download
go test -race ./...
mkdir -p .tools
go build -o .tools/nsf ./cmd/nsf
python python/test_integration.py
python python/demo.py
python python/evaluate.py
```

On Windows PowerShell, use `New-Item -ItemType Directory -Force .tools` and build to `.tools/nsf.exe`. This workspace includes a verified official compiler at `.tools/go/bin/go.exe`; it is excluded from source distribution. Set `CGO_ENABLED=1` if your environment disables it. GCC must be on PATH. The demos create temporary fixture repositories and state, launch the actual service, and clean up their own files.

All six demos are assertions over real operations: incomplete search, composition, concurrent publication, incremental reuse, forged evidence, and lost-response recovery. Saved output is in [docs/demo-results.json](docs/demo-results.json).

## Use your repository

Copy `config.example.json` to a private configuration file. Set the allowed `repositories.demo` path to an existing local repository and `root` to a dedicated service-owned directory. Select the full commit ID with `git rev-parse HEAD` in that source repository. Do not put the managed state inside an agent-writable directory for enforced use.

The trusted local CLI accepts one JSON request on stdin:

```sh
echo '{"repository":"demo","snapshot":"FULL_COMMIT_ID"}' | .tools/nsf -config config.json -user local register
echo '{"repository":"demo"}' | .tools/nsf -config config.json -user local head
```

PowerShell accepts the same single-quoted JSON; use `.tools/nsf.exe`. The CLI is a trusted operator interface: `-user` is not network authentication. Run one process against a state directory at a time. A SQLite lease rejects another service/CLI instance while one is running.

For HTTP, set `NSF_TOKEN` to a random value of at least 24 characters, then start:

```sh
.tools/nsf -config config.json -user local serve
```

The default address is `127.0.0.1:8787`. The token is mapped to the configured `local` principal. Requests use `Authorization: Bearer ...`; no browser origins are accepted. Operators may instead configure a private token-to-principal map. Never commit tokens.

```python
import os, sys
sys.path.insert(0, "python")
from notsofast import Client
c = Client(token=os.environ["NSF_TOKEN"])
print(c.call("head", repository="demo"))
```

See [protocol and examples](docs/PROTOCOL.md) for all operations. Byte fields use standard Base64. Receipt IDs are references; the service never accepts agent-authored receipt bodies.

## Policies

The sample configuration requires basename uniqueness for every creation. Files below `markers/` additionally require absence of literal `owned: demo` in regular blobs, and must contain that marker. **Every applicable destination-prefix policy is enforced.** Selecting a policy ID cannot disable another policy. Marker creations therefore need evidence for both conventions.

Policies and principal grants are frozen per process and activated as a Git policy epoch. The publication transaction verifies both the old branch head and that epoch. A stale process cannot publish after a new epoch has become active. Old successful operations remain recognizable on retries; no new mutation is performed for them.

## MCP

Run `python python/mcp_adapter.py` with `NSF_URL` and `NSF_TOKEN`. It implements newline-delimited stdio MCP 2025-11-25 initialization, tools/list, tools/call, and ping. It forwards to HTTP; it has no Git or evidence-store access requirement. Tested with the included protocol client against a live service, not with every agent host. Calls are sequential and bounded by the HTTP timeout; cancellation notifications do not interrupt an already running Python HTTP call.

For process separation, `compose.yaml` supplies a service container and an optional MCP client container without state/source mounts. Set `NSF_SOURCE` to the allowed source repository and `NSF_TOKEN`, then run `docker compose up --build nsf`. The source is mounted read-only; service state has its own named volume. `docker compose run --rm -T mcp` starts the adapter. Docker execution has not been validated on the development host because Docker is unavailable.

## Evidence and measured value

The early experiment did **not** establish a receipt-specific speed advantage. Ordinary memoization and receipts reuse the same unchanged inputs. Full fresh filename checks can be faster than either. Receipts are positioned around explicit coverage, composition, traceability and recovery, with conventional memoization underneath.

[Evaluation](docs/EVALUATION.md) links raw measurements, scripted controls, ablations and limitations. The optional provider-neutral [model harness](python/model_harness.py) takes a JSON driver command; no model experiment was run. Scripted trials are not model trials.

## Documentation

- [Architecture, trust model and guarantee matrix](docs/ARCHITECTURE.md)
- [Predicate, scope and API definitions](docs/PROTOCOL.md)
- [Recovery and retention](docs/RECOVERY.md)
- [Measurements and reproduction](docs/EVALUATION.md)
- [Implementation audit](AUDIT.md)
- [Related work](docs/RELATED_WORK.md)
- [Original requirements](docs/SPEC.md) and [implementation plan](docs/PLAN.md)
- [MIT license](LICENSE) and [dependency notices](THIRD_PARTY_NOTICES.md)

This is a trusted-developer, single-instance local tool. Protection is conditional on gateway use in a same-user local setup. No power-loss durability or machine-checked proof is claimed. See the audit for validation limits and open performance work.
