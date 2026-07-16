"""Six scripted, model-free demos against a real Go HTTP service."""
import base64
from concurrent.futures import ThreadPoolExecutor
from contextlib import contextmanager
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import tempfile
import time
from types import SimpleNamespace

from notsofast import APIError, Client


def predicate(kind, value):
    return dict(kind=kind, value=base64.b64encode(value.encode()).decode(), version=1)


def git(directory, *args):
    env = {**os.environ, "GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_NOSYSTEM": "1",
           "GIT_AUTHOR_NAME": "Demo", "GIT_AUTHOR_EMAIL": "demo@example.invalid",
           "GIT_COMMITTER_NAME": "Demo", "GIT_COMMITTER_EMAIL": "demo@example.invalid"}
    return subprocess.check_output(["git", "-C", str(directory), *args], env=env, stderr=subprocess.PIPE).decode().strip()


@contextmanager
def environment(unique_scope=None):
    binary = Path(os.environ.get("NSF_BIN", str(Path(__file__).resolve().parents[1] / ".tools" / ("nsf.exe" if os.name == "nt" else "nsf")))).resolve()
    with tempfile.TemporaryDirectory(prefix="nsf-demo-") as directory:
        root = Path(directory)
        source = root / "source"
        source.mkdir()
        git(source, "init")
        for name, content in {"config/database.yaml": "db: local", "src/main.go": "package main", ".hidden/x": "hidden", "root.txt": "root"}.items():
            f = source / name
            f.parent.mkdir(parents=True, exist_ok=True)
            f.write_text(content)
        git(source, "add", ".")
        git(source, "commit", "-m", "fixture")
        head = git(source, "rev-parse", "HEAD")
        token = secrets.token_hex(24)
        config = root / "config.json"
        config.write_text(json.dumps({"root": str(root / "state"), "repositories": {"demo": str(source)},
                          "principals": {"alice": {"repositories": ["demo"], "write": True}, "outsider": {"repositories": []}},
                          "policies": {"unique": {"version": "1", "kind": "exact_basename", "scope": unique_scope or {}, "max_bytes": 4096},
                                       "marker": {"version": "1", "kind": "literal_bytes", "destination_prefix": "markers", "marker": base64.b64encode(b"owned: demo").decode(), "max_bytes": 4096}},
                          "tokens": {token: "alice", "outsider-token-for-demo-only": "outsider"}}))
        with socket.socket() as sock:
            sock.bind(("127.0.0.1", 0))
            port = sock.getsockname()[1]
        client = Client(f"http://127.0.0.1:{port}", token)
        processes = []

        def start():
            proc = subprocess.Popen([str(binary), "-config", str(config), "-listen", f"127.0.0.1:{port}", "serve"],
                                    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                                    creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0))
            processes.append(proc)
            deadline = time.monotonic() + 15
            while time.monotonic() < deadline:
                if proc.poll() is not None:
                    raise RuntimeError("service startup failed")
                try:
                    client.call("head", repository="demo")
                    return proc
                except APIError:
                    return proc  # Authenticated response; repository not registered yet.
                except OSError:
                    time.sleep(.05)
            raise RuntimeError("service startup timeout")

        proc = start()
        try:
            client.call("register", repository="demo", snapshot=head)
            yield SimpleNamespace(client=client, head=head, source=source, process=proc, restart=start)
        finally:
            for proc in processes:
                if proc.poll() is None:
                    proc.terminate()
                proc.wait(timeout=10)


def run():
    results = {}
    with environment() as env:
        c, head = env.client, env.head
        p = predicate("exact_basename", "database.yaml")
        r = c.call("search", repository="demo", snapshot=head, predicate=p, scope={"prefixes": ["src"]})["receipt"]
        claim = dict(repository="demo", snapshot=head, predicate=p, receipts=[r["id"]])
        before = c.call("verify", **claim)["outcome"]
        r2 = c.call("search_missing", **claim)["receipt"]
        claim["receipts"].append(r2["id"])
        after = c.call("verify", **claim)["outcome"]
        assert (before, after) == ("UNKNOWN", "REFUTED")
        results["incomplete_search"] = [before, after]

        p = predicate("exact_basename", "fresh.txt")
        ids = []
        for scope in ({"prefixes": ["src"]}, {"prefixes": ["config"]}, {"prefixes": [".hidden"]}, {"paths": ["src", "config", ".hidden", "root.txt"]}):
            ids.append(c.call("search", repository="demo", snapshot=head, predicate=p, scope=scope)["receipt"]["id"])
        claim = dict(repository="demo", snapshot=head, predicate=p, receipts=ids[:-1] * 2)
        assert c.call("verify", **claim)["outcome"] == "UNKNOWN"
        claim["receipts"] = ids
        composed = c.call("compose", **claim)["receipt"]
        claim["receipts"] = [composed["id"]]
        assert c.call("verify", **claim)["outcome"] == "SUPPORTED"
        results["composition"] = {"missing_and_duplicates": "UNKNOWN", "complete": "SUPPORTED"}

        request = dict(repository="demo", snapshot=head, operation="concurrent-a", policy="unique", policy_version="1",
                       path="fresh.txt", content=base64.b64encode(b"new content").decode(), receipts=[composed["id"]])
        def publish(op):
            try:
                return c.call("guarded_create", **{**request, "operation": op})
            except APIError as exc:
                return {"outcome": exc.reason}
        with ThreadPoolExecutor(max_workers=2) as pool:
            outcomes = list(pool.map(publish, ["concurrent-a", "concurrent-b"]))
        assert sorted(x["outcome"] for x in outcomes) == ["PUBLISHED", "STATE_CHANGED"]
        new_head = c.call("head", repository="demo")["snapshot"]
        refreshed = c.call("refresh", repository="demo", snapshot=new_head, predicate=p, parents=[composed["id"]])["receipt"]
        assert c.call("verify", repository="demo", snapshot=new_head, predicate=p, receipts=[refreshed["id"]])["outcome"] == "REFUTED"
        results["concurrent_publication"] = [x["outcome"] for x in outcomes] + ["REFUTED"]

        p2 = predicate("literal_bytes", "new content")
        old = c.call("search", repository="demo", snapshot=head, predicate=p2)["receipt"]
        ts = time.perf_counter_ns()
        derived = c.call("refresh", repository="demo", snapshot=new_head, predicate=p2, parents=[old["id"]])["receipt"]
        derived_ns = time.perf_counter_ns() - ts
        ts = time.perf_counter_ns()
        full = c.call("search", repository="demo", snapshot=new_head, predicate=p2, fresh=True)["receipt"]
        fresh_ns = time.perf_counter_ns() - ts
        decisions = [c.call("verify", repository="demo", snapshot=new_head, predicate=p2, receipts=[r["id"]])["outcome"] for r in (derived, full)]
        assert decisions == ["REFUTED", "REFUTED"]
        results["incremental_reuse"] = {"outcomes": decisions, "derived_total_ns": derived_ns, "fresh_total_ns": fresh_ns, "stats": derived["stats"]}

        for client, receipt in ((c, "fabricated"), (Client(c.url, "outsider-token-for-demo-only"), derived["id"])):
            try:
                client.call("receipt", id=receipt)
                raise AssertionError("forged/unauthorized evidence accepted")
            except APIError as exc:
                assert exc.reason == "INVALID_EVIDENCE"
        results["forged_evidence"] = ["INVALID_EVIDENCE", "INVALID_EVIDENCE"]

        winner = next(x for x in outcomes if x["outcome"] == "PUBLISHED")
        env.process.kill()
        env.process.wait(timeout=10)
        env.restart()
        recovered = c.call("guarded_create", **{**request, "operation": winner["id"]})
        assert recovered["candidate"] == winner["candidate"]
        assert c.call("head", repository="demo")["snapshot"] == new_head
        assert git(env.source, "rev-parse", "HEAD") == head and not git(env.source, "status", "--porcelain")
        results["lost_response"] = {"same_commit": True, "second_publication": False}
        results["transport"] = {"tool_calls": c.calls, "bytes_returned": c.bytes_returned, "actual_model_calls": 0}
    return results


if __name__ == "__main__":
    print(json.dumps(run(), indent=2))
