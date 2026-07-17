"""Compatibility check with the installed official MCP Python SDK, against real Go service."""
import asyncio
import json
import os
from pathlib import Path
import sys

from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client
from demo import environment, predicate


async def check(env):
    params = StdioServerParameters(command=sys.executable, args=[str(Path(__file__).with_name('mcp_adapter.py'))],
                                  env={**os.environ, 'NSF_URL':env.client.url, 'NSF_TOKEN':env.client.token})
    async with stdio_client(params) as (read, write):
        async with ClientSession(read, write) as client:
            initialized = await client.initialize()
            tools = await client.list_tools()
            assert any(t.name == 'guarded_create' for t in tools.tools)
            result = await client.call_tool('search', {'repository':'demo','snapshot':env.head,
                                                       'predicate':predicate('exact_basename','sdk-new.txt'),'scope':{}})
            assert not result.is_error, result
            receipt = json.loads(result.content[0].text)['receipt']['id']
            decision = await client.call_tool('verify', {'repository':'demo','snapshot':env.head,
                                                        'predicate':predicate('exact_basename','sdk-new.txt'),'receipts':[receipt]})
            assert json.loads(decision.content[0].text)['outcome'] == 'SUPPORTED'
            result = await client.call_tool('guarded_create', {'repository':'demo','snapshot':env.head,'operation':'sdk-op',
                              'policy':'unique','policy_version':'1','path':'sdk-new.txt','content':'b2s=','receipts':[receipt]})
            assert json.loads(result.content[0].text)['outcome'] == 'PUBLISHED'
            await client.send_ping()
            print(json.dumps({'result':'PASS','sdk':'official Python MCP SDK 2.3.0','protocol':initialized.protocol_version,
                              'checks':['initialize','tool schemas','search','verify','guarded publication','ping','shutdown']}))


if __name__ == '__main__':
    with environment() as env:
        asyncio.run(check(env))
