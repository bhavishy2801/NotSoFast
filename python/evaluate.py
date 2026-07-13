"""Scripted action controls, separate from performance and optional model trials."""
import base64
import json
import time
from demo import environment, predicate
from notsofast import APIError


def run():
    rows = []
    for baseline in ("A_partial", "B_warning_scripted", "C_fresh", "D_memoized", "E_receipts", "reject_all"):
        for destination, valid in (("database.yaml", False), ("fresh.txt", True)):
            # The experiment operator configures A/B with destination checks only.
            # The production agent cannot change this configuration.
            scope = {"paths": [destination]} if baseline.startswith(("A_", "B_")) else None
            with environment(scope) as env:
                c, head = env.client, env.head
                p = predicate("exact_basename", destination)
                ts = time.perf_counter_ns()
                outcome, recovery = "REJECT_ALL", False
                if baseline != "reject_all":
                    if baseline.startswith(("A_", "B_")):
                        c.call("search", repository="demo", snapshot=head, predicate=p, scope={"prefixes": ["src"]})
                        receipt = c.call("search", repository="demo", snapshot=head, predicate=p, scope=scope)["receipt"]
                    elif baseline == "E_receipts":
                        partial = c.call("search", repository="demo", snapshot=head, predicate=p, scope={"prefixes": ["src"]})["receipt"]
                        claim = dict(repository="demo", snapshot=head, predicate=p, receipts=[partial["id"]])
                        assert c.call("verify", **claim)["outcome"] == "UNKNOWN"
                        receipt = c.call("search_missing", **claim)["receipt"]
                        recovery = True
                    else:
                        receipt = c.call("search", repository="demo", snapshot=head, predicate=p, fresh=baseline == "C_fresh")["receipt"]
                    try:
                        outcome = c.call("guarded_create", repository="demo", snapshot=head, operation="trial",
                                         policy="unique", policy_version="1", path=destination,
                                         content=base64.b64encode(b"demo").decode(), receipts=[receipt["id"]])["outcome"]
                    except APIError as exc:
                        outcome = exc.reason
                rows.append(dict(baseline=baseline, destination=destination, ground_truth_valid=valid, outcome=outcome,
                                 incorrectly_admitted=int(not valid and outcome == "PUBLISHED"),
                                 incorrectly_blocked=int(valid and outcome != "PUBLISHED"),
                                 legitimate_completion=int(valid and outcome == "PUBLISHED"),
                                 accurately_identified_unsatisfied=int(not valid and outcome == "REFUTED"),
                                 recovered_unknown=recovery, calls=c.calls, bytes_returned=c.bytes_returned,
                                 total_ns=time.perf_counter_ns()-ts, actual_model_calls=0))
    return {"kind": "scripted controls, not model evaluation", "trials_per_baseline": 2,
            "valid_tasks_per_baseline": 1, "invalid_tasks_per_baseline": 1,
            "warning_limit": "B carries a warning label but the same scripted choices as A; no effect of language instructions is measured.",
            "performance_limit": "C/D use core receipt transport to preserve identical publication safeguards; isolated cache/receipt performance is measured separately.",
            "rows": rows}


if __name__ == "__main__":
    print(json.dumps(run(), indent=2))
