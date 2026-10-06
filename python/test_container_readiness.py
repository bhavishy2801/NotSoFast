"""Docker-independent regression checks for restart connection handling."""
import json
from unittest.mock import patch

import test_container as container
from notsofast import APIError


def inspection(port):
    return json.dumps([{'NetworkSettings':{'Ports':{'8787/tcp':[{'HostPort':port}]}}}])


with patch.object(container,'docker',side_effect=[inspection('40001'),inspection('40002')]), \
     patch.object(container,'Client') as client, patch.object(container.time,'sleep'):
    client.return_value.call.side_effect=[OSError('starting'),{'head':'abc'},{'candidate':'abc'}]
    assert container.wait_for_api('fixture','token','head')=={'head':'abc'}
    assert container.wait_for_api('fixture','token','operation')=={'candidate':'abc'}
    assert [call.args[0] for call in client.call_args_list]==['http://127.0.0.1:40001','http://127.0.0.1:40002']

with patch.object(container,'docker',return_value=inspection('40003')), \
     patch.object(container,'Client') as client, \
     patch.object(container.time,'monotonic',side_effect=[0,31]):
    client.return_value.call.side_effect=OSError('connection refused')
    try:container.wait_for_api('fixture','token','operation')
    except TimeoutError as exc:
        assert 'operation unavailable on port 40003' in str(exc)
        assert isinstance(exc.__cause__,OSError)
    else:raise AssertionError('Exhausted readiness must fail explicitly')

with patch.object(container,'docker',return_value=inspection('40004')), \
     patch.object(container,'Client') as client:
    client.return_value.call.side_effect=APIError('FORBIDDEN',403)
    try:container.wait_for_api('fixture','token','operation')
    except APIError as exc:assert exc.reason=='FORBIDDEN'
    else:raise AssertionError('API errors must not be retried or hidden')

print('PASS: restart port refresh, transient retry, explicit timeout, API errors preserved')
