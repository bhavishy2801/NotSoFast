"""MCP 2025-11-25 stdio adapter; never scans, verifies, or writes Git itself."""
import json
import os
import sys

from notsofast import APIError, Client, OPERATIONS

PREDICATE = {"type": "object", "properties": {"kind": {"enum": ["exact_path", "exact_basename", "literal_bytes"]},
             "value": {"type": "string", "description": "Base64-encoded nonempty literal bytes"}, "version": {"const": 1}},
             "required": ["kind", "value", "version"], "additionalProperties": False}
SCOPE = {"type": "object", "properties": {"paths": {"type": "array", "items": {"type": "string"}},
         "prefixes": {"type": "array", "items": {"type": "string"}},
         "type": {"enum": ["regular", "tree", "symlink", "gitlink"]}}, "additionalProperties": False}
FIELDS = {name: {"type": "string"} for name in ("repository", "snapshot", "id", "operation", "policy", "policy_version", "path")}
FIELDS.update(predicate=PREDICATE, scope=SCOPE, content={"type": "string", "description": "Base64 file content"},
              receipts={"type": "array", "items": {"type": "string"}}, parents={"type": "array", "items": {"type": "string"}}, fresh={"type": "boolean"})
PARAMETERS = {"register": ("repository", "snapshot"), "head": ("repository",), "snapshot": ("repository", "snapshot"),
              "search": ("repository", "snapshot", "predicate", "scope", "parents", "fresh"),
              "refresh": ("repository", "snapshot", "predicate", "scope", "parents", "fresh"),
              "receipt": ("id",), "operation": ("repository", "id"),
              "guarded_create": ("repository", "snapshot", "operation", "policy", "policy_version", "path", "content", "receipts")}
for _op in ("verify", "compose", "search_missing"):
    PARAMETERS[_op] = ("repository", "snapshot", "predicate", "scope", "receipts")


def main():
    client = Client(os.environ.get("NSF_URL", "http://127.0.0.1:8787"), os.environ.get("NSF_TOKEN", ""))
    initialized = False
    while True:
        line = sys.stdin.buffer.readline((2 << 20) + 1)
        if not line:
            return
        if len(line) > 2 << 20:
            return  # Bounded framing: terminate rather than parse an unbounded line.
        ident = None
        try:
            q = json.loads(line)
            if not isinstance(q, dict) or q.get("jsonrpc") != "2.0" or not isinstance(q.get("method"), str):
                raise ValueError("invalid JSON-RPC request")
            ident = q.get("id")
            if "id" not in q:
                if q["method"] == "notifications/initialized":
                    initialized = True
                continue
            method, params = q["method"], q.get("params", {})
            if not isinstance(params, dict):
                raise ValueError("params must be an object")
            if method == "initialize":
                result = {"protocolVersion": "2025-11-25", "capabilities": {"tools": {}},
                          "serverInfo": {"name": "notsofast", "version": "0.1.0"}}
            elif method == "ping":
                result = {}
            elif not initialized:
                raise ValueError("initialize first")
            elif method == "tools/list":
                result = {"tools": [{"name": op, "description": "NotSoFast " + op + ": exact tracked-snapshot evidence; UNKNOWN never authorizes creation.",
                           "inputSchema": {"type": "object", "properties": {k: FIELDS[k] for k in PARAMETERS[op]},
                                           "required": [k for k in PARAMETERS[op] if k not in ("scope", "parents", "fresh")], "additionalProperties": False}}
                          for op in OPERATIONS]}
            elif method == "tools/call":
                try:
                    name, args = params.get("name"), params.get("arguments", {})
                    if name not in OPERATIONS or not isinstance(args, dict):
                        raise ValueError("invalid tool call")
                    data = client.call(name, **args)
                    result = {"content": [{"type": "text", "text": json.dumps(data)}], "structuredContent": data, "isError": False}
                except (APIError, OSError) as exc:
                    result = {"content": [{"type": "text", "text": getattr(exc, "reason", "TRANSPORT_ERROR")}], "isError": True}
            else:
                print(json.dumps({"jsonrpc": "2.0", "id": ident, "error": {"code": -32601, "message": "Method not found"}}), flush=True)
                continue
            reply = {"jsonrpc": "2.0", "id": ident, "result": result}
        except json.JSONDecodeError:
            reply = {"jsonrpc": "2.0", "id": None, "error": {"code": -32700, "message": "Parse error"}}
        except (ValueError, TypeError, KeyError):
            reply = {"jsonrpc": "2.0", "id": ident, "error": {"code": -32602, "message": "Invalid request or parameters"}}
        print(json.dumps(reply, separators=(",", ":")), flush=True)


if __name__ == "__main__":
    sys.exit(main())
