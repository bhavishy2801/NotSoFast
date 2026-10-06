"""Browser layout and interaction regression checks against the packaged app."""
import json,os,subprocess,tempfile,time,urllib.request,http.server,threading
from pathlib import Path
from playwright.sync_api import sync_playwright,expect
ROOT=Path(__file__).resolve().parents[1]
EXE=Path(os.environ.get('NSF_DESKTOP_EXE',ROOT/'dist/NotSoFast/NotSoFast.exe')).resolve()
class ModelStub(http.server.BaseHTTPRequestHandler):
    def log_message(self,*args):pass
    def do_POST(self):
        messages=json.loads(self.rfile.read(int(self.headers['Content-Length'])))['messages']
        message={'role':'assistant','content':'done'}
        if len(messages) in (2,4):
            name='search' if len(messages)==2 else 'create'
            args={'scope':{}} if len(messages)==2 else {'receipts':[json.loads(messages[3]['content'])['receipt']['id']]}
            message['tool_calls']=[{'id':name,'type':'function','function':{'name':name,'arguments':json.dumps(args)}}]
        raw=json.dumps({'choices':[{'message':message}]}).encode()
        self.send_response(200);self.send_header('Content-Type','application/json');self.end_headers();self.wfile.write(raw)
model=http.server.ThreadingHTTPServer(('127.0.0.1',0),ModelStub)
threading.Thread(target=model.serve_forever,daemon=True).start()
with tempfile.TemporaryDirectory(prefix='layout-',dir=ROOT/'.tools') as temp:
    process=subprocess.Popen([str(EXE),'-data',temp,'-no-open'],stdout=subprocess.DEVNULL,creationflags=getattr(subprocess,'CREATE_NO_WINDOW',0))
    try:
        session=Path(temp)/'session.json'
        for _ in range(150):
            if session.exists():break
            time.sleep(.1)
        info=json.loads(session.read_text())
        with sync_playwright() as p:
            browser=p.chromium.launch(channel=os.environ.get('NSF_BROWSER','msedge'),headless=True)
            page=browser.new_page(viewport={'width':1280,'height':720})
            errors=[];page.on('pageerror',lambda e:errors.append(str(e)))
            page.goto(info['url']+'/#'+info['token'])
            expect(page.locator('#session-gate')).to_be_hidden()
            page.locator('#try-demo').click()
            expect(page.locator('#connection')).to_have_class('connection connected',timeout=45000)
            for width,height in [(1280,720),(800,500),(390,600),(320,480)]:
                page.set_viewport_size({'width':width,'height':height})
                for name in ['overview','evidence','publish','activity','models','files','account','settings']:
                    if name=='settings':page.locator('#connect-top').click()
                    elif name=='account':page.locator('[data-page="account"]').first.click()
                    else:page.locator('nav [data-page="'+name+'"]').click()
                    expect(page.locator('#busy-bar')).to_be_hidden(timeout=30000)
                    assert page.evaluate('document.documentElement.scrollWidth<=innerWidth'),('page overflow',name,width)
                page.keyboard.press('Control+k')
                expect(page.locator('#command-dialog')).to_be_visible()
                page.keyboard.press('Escape')
                expect(page.locator('#command-dialog')).to_be_hidden()
                page.locator('#appearance-open').click()
                dialog=page.locator('#appearance-dialog')
                expect(dialog).to_be_visible()
                for scheme in ['Citrus','Tidal','Iris','Bloom','Ember']:
                    page.locator('.scheme-choice').filter(has_text=scheme).click()
                for mode in ['light','dark','system']:
                    page.locator('#theme-mode').select_option(mode)
                page.locator('#appearance-save').scroll_into_view_if_needed()
                bounds=page.locator('#appearance-save').bounding_box()
                assert bounds['y']>=0 and bounds['y']+bounds['height']<=height,('save clipped',width,bounds)
                assert dialog.evaluate('(e)=>e.scrollWidth<=e.clientWidth'),('dialog horizontal clipping',width)
                grid=page.locator('#scheme-options').bounding_box();box=dialog.bounding_box()
                assert grid['x']>=box['x']+12,('theme content touches/clips modal edge',width,grid,box)
                page.screenshot(path=str(ROOT/f'docs/theme-{width}.png'),full_page=False,animations='disabled')
                page.locator('#appearance-save').click()
                expect(dialog).to_be_hidden()
            page.set_viewport_size({'width':1280,'height':800})
            page.locator('[data-page="account"]').first.click()
            expect(page.locator('#busy-bar')).to_be_hidden(timeout=30000)
            page.locator('#profile-name').fill('Backup original')
            page.locator('#profile-form button').click()
            expect(page.locator('#profile-label')).to_have_text('Backup original')
            with page.expect_download() as backup:page.locator('#profile-export').click()
            exported=Path(backup.value.path()).read_bytes()
            payload=json.loads(exported);payload['profile']['bookmarks']=['https://github.com/octocat/Hello-World'];exported=json.dumps(payload).encode()
            page.locator('#profile-name').fill('Changed profile')
            page.locator('#profile-form button').click()
            expect(page.locator('#profile-label')).to_have_text('Changed profile')
            page.locator('#profile-import-file').set_input_files({'name':'backup.json','mimeType':'application/json','buffer':exported})
            expect(page.locator('#restore-preview')).to_contain_text('Backup original')
            page.locator('#restore-confirm').click()
            expect(page.locator('#profile-label')).to_have_text('Backup original')
            expect(page.locator('#cloud-restore-dialog')).to_be_hidden()
            expect(page.locator('#bookmarks .bookmark-row')).to_have_count(1)
            page.locator('#bookmarks .icon-button').click()
            expect(page.locator('#bookmarks .bookmark-row')).to_have_count(0)
            page.locator('#profile-import-file').set_input_files({'name':'bad.json','mimeType':'application/json','buffer':b'invalid'})
            expect(page.locator('#error-message')).to_contain_text('valid NotSoFast JSON backup')
            page.locator('#dismiss-error').click()
            page.locator('nav [data-page="models"]').click()
            page.locator('#model-endpoint').fill('http://127.0.0.1:'+str(model.server_port)+'/v1')
            page.locator('#model-name').fill('scripted-browser-contract-not-inference')
            page.locator('#model-form button[type="submit"]').click()
            expect(page.locator('#busy-bar')).to_be_hidden()
            page.locator('#run-models').click()
            expect(page.locator('#model-results .model-trial')).to_have_count(2,timeout=60000)
            expect(page.locator('#busy-bar')).to_be_hidden(timeout=60000)
            expect(page.locator('#model-results')).not_to_contain_text('Inspect')
            with page.expect_download() as results:page.locator('#export-models').click()
            assert len(json.loads(Path(results.value.path()).read_text()))==2
            assert not errors,errors
            browser.close()
        print(json.dumps({'result':'PASS','viewports':[1280,800,390,320],'pages':8,'themes':5,'modes':3,'checks':['backup export/import','invalid backup rejected','scripted model UI trials/export'], 'javascript_errors':errors}))
    finally:
        if process.poll() is None:process.terminate();process.wait(timeout=10)

model.shutdown();model.server_close()
