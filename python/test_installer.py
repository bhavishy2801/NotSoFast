"""Real install/update/uninstall smoke test. Requires Windows and a built installer.
Uses a dedicated installation folder and leaves application data untouched.
Refuses to replace an existing registered NotSoFast installation.
"""
import json, os, subprocess, tempfile, time, urllib.request, winreg
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
KEY=r'Software\Microsoft\Windows\CurrentVersion\Uninstall\NotSoFast'
try:
    with winreg.OpenKey(winreg.HKEY_CURRENT_USER,KEY):
        raise RuntimeError('Existing installation detected; use a clean Windows account for installer testing')
except FileNotFoundError:
    pass
with tempfile.TemporaryDirectory(prefix='setup-',dir=ROOT/'.tools') as temp:
    target=Path(temp)/'app'; data=Path(temp)/'saved'; data.mkdir()
    sentinel=data/'keep.txt';sentinel.write_text('keep saved work')
    setup=ROOT/'dist/NotSoFast-Setup.exe'
    for iteration in range(2):
        result=subprocess.run([str(setup),'/silent','/dir='+str(target)],timeout=120)
        assert result.returncode==0,('setup failed',iteration,result.returncode)
        assert (target/'NotSoFast.exe').is_file()
        assert (target/'github/bin/gh.exe').is_file()
        assert (target/'git/cmd/git.exe').is_file()
    with winreg.OpenKey(winreg.HKEY_CURRENT_USER,KEY) as key:
        assert winreg.QueryValueEx(key,'InstallLocation')[0]==str(target)
    process=subprocess.Popen([str(target/'NotSoFast.exe'),'-data',str(data),'-no-open'],stdout=subprocess.DEVNULL)
    try:
        for _ in range(150):
            if (data/'session.json').exists():break
            time.sleep(.1)
        session=json.loads((data/'session.json').read_text())
        request=urllib.request.Request(session['url']+'/api/quit',data=b'{}',headers={'X-NSF-Session':session['token']})
        with urllib.request.urlopen(request,timeout=10) as response:assert response.status==200
        assert process.wait(timeout=20)==0
    finally:
        if process.poll() is None:process.terminate();process.wait(timeout=10)
    outside=Path(temp)/'outside';outside.mkdir();(outside/'keep.txt').write_text('outside')
    junction=target/'junction-test'
    subprocess.run(['cmd.exe','/c','mklink','/J',str(junction),str(outside)],check=True,stdout=subprocess.DEVNULL)
    blocked=subprocess.run([str(target/'Uninstall.exe'),'/silent'],timeout=60)
    assert blocked.returncode!=0,'uninstaller accepted a junction'
    assert (outside/'keep.txt').read_text()=='outside'
    assert (target/'NotSoFast.exe').exists(),'failed safety check modified installation'
    os.rmdir(junction)
    result=subprocess.run([str(target/'Uninstall.exe'),'/silent'],timeout=60)
    assert result.returncode==0,('uninstall failed',result.returncode)
    for _ in range(100):
        if not target.exists():break
        time.sleep(.1)
    assert not target.exists(),'installer files remain'
    assert sentinel.read_text()=='keep saved work'
    assert (data/'desktop.db').exists(),'app data was removed'
    try:
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER,KEY):raise AssertionError('uninstall registration remains')
    except FileNotFoundError:pass
print(json.dumps({'result':'PASS','checks':['install','bundled runtimes','upgrade','registered uninstall','installed app launch','junction rejected before deletion','uninstall','saved data retained']}))
