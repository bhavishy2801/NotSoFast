"""Checks the actual static website in a browser, including its production CSP."""
import functools,http.server,json,threading
from pathlib import Path
from playwright.sync_api import sync_playwright,expect
ROOT=Path(__file__).resolve().parents[1]
config=json.loads((ROOT/'website/vercel.json').read_text(encoding='utf-8-sig'))
class Handler(http.server.SimpleHTTPRequestHandler):
    def log_message(self,*args):pass
    def end_headers(self):
        for header in config['headers'][0]['headers']:self.send_header(header['key'],header['value'])
        super().end_headers()
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Handler,directory=str(ROOT/'website')))
threading.Thread(target=server.serve_forever,daemon=True).start()
try:
    with sync_playwright() as p:
        browser=p.chromium.launch(channel='msedge',headless=True)
        page=browser.new_page(viewport={'width':1440,'height':1000});errors=[]
        page.on('pageerror',lambda e:errors.append(str(e)))
        page.goto('http://127.0.0.1:'+str(server.server_port))
        for width,height in [(1440,1000),(768,1024),(390,844),(320,568)]:
            page.set_viewport_size({'width':width,'height':height})
            for label in ['After hours','Daylight','Saved work']:
                page.get_by_role('button',name=label,exact=True).click()
                page.locator("#product-screen").evaluate("e => e.decode()")
            page.get_by_role('button',name='Whole repository',exact=True).click()
            expect(page.locator('#decision')).to_have_text('REFUTED')
            page.get_by_role('button',name='Only src/',exact=True).click()
            expect(page.locator('#decision')).to_have_text('UNKNOWN')
            page.locator('#site-theme').click()
            assert page.evaluate('document.documentElement.scrollWidth<=innerWidth'),('overflow',width)
            page.locator('summary').first.click()
            expect(page.locator('details').first).to_have_attribute('open','')
            page.locator('summary').first.click()
            page.evaluate("window.scrollTo({top:0,behavior:'instant'})")
            page.screenshot(path=str(ROOT/f'docs/website-{width}.png'),full_page=True,animations='disabled')
        assert not errors,errors
        browser.close()
    print(json.dumps({'result':'PASS','viewports':[1440,768,390,320],'checks':['production CSP','screenshot tabs','coverage example','theme switch','FAQ','no horizontal overflow','no JS errors']}))
finally:server.shutdown();server.server_close()
