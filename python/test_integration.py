"""Actual executable/service integration, including a stdio MCP handshake."""
import base64
import json
import os
from pathlib import Path
import subprocess
import sys
import unittest

from demo import environment, predicate
from notsofast import APIError, Client


class Integration(unittest.TestCase):
    def test_service_client_and_mcp(self):
        with environment() as env:
            client, head = env.client, env.head
            r = client.call("search", repository="demo", snapshot=head,
                            predicate=predicate("exact_basename", "fresh.txt"))["receipt"]
            claim = dict(repository="demo", snapshot=head, predicate=r["predicate"], receipts=[r["id"]])
            self.assertEqual(client.call("verify", **claim)["outcome"], "SUPPORTED")
            with self.assertRaises(APIError) as exc:
                Client(client.url, "wrong-token").call("head", repository="demo")
            self.assertEqual(exc.exception.reason, "UNAUTHENTICATED")
            messages = [
                {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2025-11-25", "capabilities": {}, "clientInfo": {"name": "integration-test", "version": "1"}}},
                {"jsonrpc": "2.0", "method": "notifications/initialized"},
                {"jsonrpc": "2.0", "id": 2, "method": "tools/list"},
                {"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {"name": "verify", "arguments": claim}},
                {"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": {"name": "guarded_create", "arguments": {"repository": "demo", "snapshot": head, "operation": "mcp-create", "policy": "unique", "policy_version": "1", "path": "fresh.txt", "content": base64.b64encode(b"hello").decode(), "receipts": [r["id"]]}}},
            ]
            proc = subprocess.run([sys.executable, str(Path(__file__).with_name("mcp_adapter.py"))],
                                  input="\n".join(map(json.dumps, messages)) + "\n", text=True,
                                  capture_output=True, timeout=30,
                                  env={**os.environ, "NSF_URL": client.url, "NSF_TOKEN": client.token})
            self.assertEqual(proc.returncode, 0, proc.stderr)
            replies = [json.loads(line) for line in proc.stdout.splitlines()]
            self.assertEqual(len(replies), 4)
            self.assertIn("guarded_create", [t["name"] for t in replies[1]["result"]["tools"]])
            self.assertEqual(replies[2]["result"]["structuredContent"]["outcome"], "SUPPORTED")
            self.assertEqual(replies[3]["result"]["structuredContent"]["outcome"], "PUBLISHED")


if __name__ == "__main__":
    unittest.main()
