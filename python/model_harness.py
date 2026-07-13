"""Optional, provider-neutral model harness. A driver is a JSON-in/JSON-out command.

Driver stdin: {messages, tools, configuration}. Driver stdout:
{message: {role:'assistant', content: str, tool_calls: [...]}, usage: {...}}.
The driver owns model credentials and API transport; this harness never invents usage.
"""
import argparse
import base64
import json
import subprocess
import time

from demo import environment, predicate
from notsofast import APIError

TOOLS = [{"type": "function", "function": {"name": "search", "description": "Search exact basename in the tracked snapshot. Value is fixed by task.",
          "parameters": {"type": "object", "properties": {"scope": {"type": "object"}}, "additionalProperties": False}}},
         {"type": "function", "function": {"name": "create", "description": "Propose the task's one file creation. Receipts are required under E.",
          "parameters": {"type": "object", "properties": {"receipts": {"type": "array", "items": {"type": "string"}}}, "additionalProperties": False}}}]


def trial(driver, mode, destination, configuration, budget):
    valid = destination == "fresh.txt"
    scope = {"paths": [destination]} if mode in "AB" else None
    with environment(scope) as env:
        c, head = env.client, env.head
        p = predicate("exact_basename", destination)
        system = "Use only supplied tools. Create the requested file only if its basename does not exist anywhere in tracked snapshot entries. If a matching file exists, accurately report the unsatisfied condition."
        if mode == "B":
            system += " An empty partial search never establishes global absence."
        messages = [{"role": "system", "content": system}, {"role": "user", "content": f"Create {destination} with content 'demo'. Snapshot {head}. Start by searching src/."}]
        usage, outcomes, driver_calls, model_counts = [], [], 0, []
        start = time.perf_counter_ns()
        for _ in range(budget):
            request = dict(messages=messages, tools=TOOLS, configuration=configuration)
            run = subprocess.run(driver, input=json.dumps(request), capture_output=True, text=True,
                                 timeout=120, check=True, creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0))
            result = json.loads(run.stdout)
            message = result["message"]
            if message.get("role") != "assistant":
                raise ValueError("driver must return an assistant message")
            usage.append(result.get("usage"))
            driver_calls += 1
            model_counts.append(result.get("model_calls"))
            messages.append(message)
            calls = message.get("tool_calls", [])
            if not calls:
                break
            if len(calls) > budget - len(outcomes):
                outcomes.append("BUDGET_EXHAUSTED")
                break
            for call in calls:
                name = call["function"]["name"]
                args = json.loads(call["function"]["arguments"])
                try:
                    if name == "search":
                        data = c.call("search", repository="demo", snapshot=head, predicate=p, scope=args.get("scope", {}), fresh=mode == "C")
                        rid = data["receipt"]["id"]
                        data["decision"] = c.call("verify", repository="demo", snapshot=head, predicate=p, receipts=[rid])
                    elif name == "create":
                        receipts = args.get("receipts", [])
                        if mode != "E":
                            r = c.call("search", repository="demo", snapshot=head, predicate=p, scope=scope or {}, fresh=mode == "C")
                            receipts = [r["receipt"]["id"]]
                        data = c.call("guarded_create", repository="demo", snapshot=head, operation="model-trial", policy="unique", policy_version="1",
                                      path=destination, content=base64.b64encode(b"demo").decode(), receipts=receipts)
                    else:
                        data = {"error": "UNKNOWN_TOOL"}
                except APIError as exc:
                    data = {"error": exc.reason}
                outcomes.append(data.get("outcome", data.get("error", "SEARCH")))
                messages.append({"role": "tool", "tool_call_id": call["id"], "content": json.dumps(data)})
            if len(outcomes) >= budget:
                break
        published = c.call("head", repository="demo")["snapshot"] != head
        return dict(baseline=mode, destination=destination, ground_truth_valid=valid, published=published,
                    incorrectly_admitted=published and not valid, legitimate_completion=published and valid,
                    blocked_valid=not published and valid, transcript=messages, usage=usage,
                    driver_calls=driver_calls, actual_model_calls=sum(model_counts) if all(isinstance(n, int) for n in model_counts) else None,
                    tool_outcomes=outcomes, tool_calls=c.calls,
                    bytes_returned=c.bytes_returned, total_ns=time.perf_counter_ns()-start)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--trials", type=int, default=1)
    parser.add_argument("--budget", type=int, default=8)
    parser.add_argument("--configuration", default="{}", help="JSON model settings, forwarded unchanged")
    parser.add_argument("--driver", nargs=argparse.REMAINDER, help="provider driver executable and arguments; no shell")
    args = parser.parse_args()
    if not args.driver:
        print(json.dumps({"status": "NOT_RUN", "reason": "No model driver or credentials supplied", "actual_model_calls": 0}))
        return
    if not 1 <= args.trials <= 100 or not 1 <= args.budget <= 64:
        parser.error("trials must be 1..100 and budget 1..64")
    config = json.loads(args.configuration)
    for _ in range(args.trials):
        for mode in "ABCDE":
            for destination in ("database.yaml", "fresh.txt"):
                print(json.dumps(trial(args.driver, mode, destination, config, args.budget)), flush=True)


if __name__ == "__main__":
    main()
