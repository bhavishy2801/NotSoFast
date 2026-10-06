"""Checks the actual static website in a browser, including its production CSP."""
import functools,http.server,json,os,threading
from pathlib import Path
from playwright.sync_api import sync_playwright,expect
ROOT=Path(__file__).resolve().parents[1]
config=json.loads((ROOT/'website/vercel.json').read_text(encoding='utf-8-sig'))
root_config=json.loads((ROOT/'vercel.json').read_text(encoding='utf-8-sig'))
for base,settings in [(ROOT,root_config),(ROOT/'website',config)]:
    assert (base/settings['outputDirectory']).resolve()==(ROOT/'website').resolve()
    assert (base/settings['outputDirectory']/'index.html').is_file()
    assert settings['framework'] is None and settings['buildCommand']==''
assert root_config['headers']==config['headers']
class Handler(http.server.SimpleHTTPRequestHandler):
    def log_message(self,*args):pass
    def end_headers(self):
        for header in config['headers'][0]['headers']:self.send_header(header['key'],header['value'])
        super().end_headers()
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Handler,directory=str(ROOT/'website')))
threading.Thread(target=server.serve_forever,daemon=True).start()
try:
    with sync_playwright() as p:
        browser=p.chromium.launch(channel=os.environ.get('NSF_BROWSER','msedge'),headless=True)
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
            for palette in ['citrus','glacier','orchid']:
                page.locator('button[data-palette="'+palette+'"]').click()
                expect(page.locator('body')).to_have_attribute('data-palette',palette)
                assert page.evaluate('document.documentElement.scrollWidth<=innerWidth')
            page.locator('#site-motion').click()
            expect(page.locator('#site-motion')).to_have_attribute('aria-pressed','true')
            assert page.locator('.orb').evaluate("e=>getComputedStyle(e).animationPlayState")=='paused'
            page.reload()
            expect(page.locator('body')).to_have_attribute('data-palette','orchid')
            expect(page.locator('#site-motion')).to_have_attribute('aria-pressed','true')
            assert page.locator('.hero-copy').evaluate("e=>getComputedStyle(e).opacity")=='1'
            page.locator('#site-motion').click()
            page.locator('button[data-palette="citrus"]').click()
            page.evaluate('document.fonts.ready')
            page.evaluate("window.scrollTo({top:0,behavior:'instant'})")
            page.screenshot(path=str(ROOT/f'docs/website-{width}.png'),full_page=True,animations='disabled')
            page.screenshot(path=str(ROOT/f'docs/website-hero-{width}.png'),animations='disabled')
        page.emulate_media(reduced_motion='reduce')
        assert page.locator('.orb').evaluate("e=>getComputedStyle(e).animationName")=='none'
        assert page.locator('.scan-line').evaluate("e=>getComputedStyle(e).animationName")=='none'
        assert page.evaluate("document.fonts.check('16px \"Space Grotesk\"') && document.fonts.check('16px Manrope')")
        assert not errors,errors
        browser.close()
    print(json.dumps({'result':'PASS','viewports':[1440,768,390,320],'checks':['production CSP','screenshot tabs','coverage example','theme switch','three palettes and persistence','motion pause and persistence','reduced motion','self-hosted fonts','FAQ','no horizontal overflow','no JS errors']}))
finally:server.shutdown();server.server_close()
