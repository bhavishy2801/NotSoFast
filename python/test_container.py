"""Live container contract check; requires Docker and image notsofast:test."""
import base64
import json
import os
from pathlib import Path
import secrets
import subprocess
import tempfile
import time

from demo import git, predicate
from notsofast import APIError, Client


def docker(*args):
    return subprocess.check_output(['docker', *args], text=True, stderr=subprocess.STDOUT).strip()


def run():
    name='nsf-test-'+secrets.token_hex(6)
    with tempfile.TemporaryDirectory(prefix='nsf-container-') as tmp:
        root=Path(tmp);source=root/'source';source.mkdir()
        git(source,'init');(source/'existing.txt').write_text('fixture');git(source,'add','.');git(source,'commit','-m','fixture')
        head=git(source,'rev-parse','HEAD');token=secrets.token_hex(24)
        cfg=root/'config.json';cfg.write_text(json.dumps({'root':'/state','repositories':{'demo':'/source'},
            'principals':{'local':{'repositories':['demo'],'write':True}},'tokens':{token:'local'},
            'policies':{'unique':{'kind':'exact_basename','version':'1','max_bytes':4096}}}))
        for path in [root,*root.rglob('*')]:path.chmod(0o755 if path.is_dir() else 0o644)
        docker('volume','create',name)
        try:
            docker('run','-d','--name',name,'--read-only','--cap-drop=ALL','--security-opt=no-new-privileges',
                   '--memory=768m','--pids-limit=64','--tmpfs','/tmp:size=64m','-p','127.0.0.1::8787',
                   '--mount',f'type=bind,src={source},dst=/source,readonly',
                   '--mount',f'type=bind,src={cfg},dst=/app/config.json,readonly',
                   '--mount',f'type=volume,src={name},dst=/state','notsofast:test')
            port=json.loads(docker('inspect',name))[0]['NetworkSettings']['Ports']['8787/tcp'][0]['HostPort']
            client=Client('http://127.0.0.1:'+port,token)
            for _ in range(100):
                try:client.call('head',repository='demo');break
                except APIError:break
                except OSError:time.sleep(.1)
            client.call('register',repository='demo',snapshot=head)
            p=predicate('exact_basename','fresh.txt');r=client.call('search',repository='demo',snapshot=head,predicate=p)
            q=dict(repository='demo',snapshot=head,operation='container-test',policy='unique',policy_version='1',
                   path='fresh.txt',content=base64.b64encode(b'ok').decode(),receipts=[r['receipt']['id']])
            first=client.call('guarded_create',**q);assert first['outcome']=='PUBLISHED'
            docker('restart',name)
            for _ in range(100):
                try:recovered=client.call('operation',repository='demo',id='container-test');break
                except OSError:time.sleep(.1)
            assert recovered['candidate']==first['candidate']
            assert git(source,'rev-parse','HEAD')==head and not (source/'fresh.txt').exists()
            print(json.dumps({'result':'PASS','checks':['non-root','read-only source','bounded container','search','publication','restart recovery','source unchanged']}))
        except Exception:
            print(docker('logs',name))
            raise
        finally:
            subprocess.run(['docker','rm','-f',name],check=False,stdout=subprocess.DEVNULL)
            subprocess.run(['docker','volume','rm',name],check=False,stdout=subprocess.DEVNULL)


if __name__=='__main__':run()
