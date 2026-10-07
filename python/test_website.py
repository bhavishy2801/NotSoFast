"""Checks the actual static website in a browser, including its production CSP."""
import functools,http.server,json,os,threading,sys
from pathlib import Path
from urllib.request import Request, urlopen
from urllib.error import HTTPError
from playwright.sync_api import sync_playwright,expect
ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'scripts'))
from preview_website import RangeHandler
config=json.loads((ROOT/'website/vercel.json').read_text(encoding='utf-8-sig'))
root_config=json.loads((ROOT/'vercel.json').read_text(encoding='utf-8-sig'))
for base,settings in [(ROOT,root_config),(ROOT/'website',config)]:
    assert (base/settings['outputDirectory']).resolve()==(ROOT/'website').resolve()
    assert (base/settings['outputDirectory']/'index.html').is_file()
    assert settings['framework'] is None and settings['buildCommand']==''
assert root_config['headers']==config['headers']
class Handler(RangeHandler):
    def log_message(self,*args):pass
    def end_headers(self):
        for header in config['headers'][0]['headers']:self.send_header(header['key'],header['value'])
        super().end_headers()
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Handler,directory=str(ROOT/'website')))
threading.Thread(target=server.serve_forever,daemon=True).start()
try:
    video_url='http://127.0.0.1:'+str(server.server_port)+'/assets/evidence-film.mp4'
    for value in ['bytes=0-31','bytes=-32']:
        with urlopen(Request(video_url,headers={'Range':value})) as response:
            assert response.status==206 and len(response.read())==32
    try:urlopen(Request(video_url,headers={'Range':'bytes=999999999-'}))
    except HTTPError as exc:assert exc.code==416
    else:raise AssertionError('Invalid byte range accepted')
    with sync_playwright() as p:
        browser=p.chromium.launch(channel=os.environ.get('NSF_BROWSER','msedge'),headless=True)
        page=browser.new_page(viewport={'width':1440,'height':1000});errors=[]
        page.on('pageerror',lambda e:errors.append(str(e)))
        page.goto('http://127.0.0.1:'+str(server.server_port))
        for width,height in [(1440,1000),(768,1024),(390,844),(320,568)]:
            page.set_viewport_size({'width':width,'height':height})
            page.locator('[data-frame="0.98"]').click()
            expect(page.locator('#film-chapter')).to_have_text('03 / BLOCK THE DUPLICATE')
            for _ in range(60):
                if page.locator('#evidence-film').evaluate('v=>v.readyState>=2 && !v.seeking && v.currentTime>10'):break
                page.wait_for_timeout(100)
            assert page.locator('#evidence-film').evaluate('v=>v.readyState>=2 && v.currentTime>10'), 'Film did not seek to final chapter'
            assert not page.locator('#film').evaluate("e=>e.classList.contains('film-unavailable')"), 'Successful video load must clear fallback'
            assert page.locator('#evidence-film').evaluate('v=>v.videoHeight>v.videoWidth') == (width<=760), 'Wrong film framing for viewport'
            assert page.locator('#evidence-film').evaluate("v=>getComputedStyle(v).objectFit==='contain'"), 'Product video must not be cropped'
            assert page.locator('.film-media').evaluate("e=>{const a=e.getBoundingClientRect(),b=document.querySelector('.film-copy').getBoundingClientRect();return a.right<=b.left+1||a.bottom<=b.top+1;}"), 'Captions obscure the product video'
            page.locator('#film-timeline').focus()
            page.keyboard.press('Home')
            expect(page.locator('#film-timeline')).to_have_value('0')
            expect(page.locator('#film-chapter')).to_have_text('01 / CHECK THE SCOPE')
            for _ in range(40):
                if page.locator('#evidence-film').evaluate('v=>!v.seeking && v.currentTime<.2'):break
                page.wait_for_timeout(100)
            assert page.locator('#evidence-film').evaluate('v=>v.currentTime<.2'), 'Backward seeking failed'
            if width==1440:
                page.evaluate("window.dispatchEvent(new WheelEvent('wheel')); const f=document.querySelector('#film'); window.scrollTo({top:f.offsetTop+(f.offsetHeight-innerHeight)*.5,behavior:'instant'})")
                for _ in range(50):
                    if page.locator('#evidence-film').evaluate('v=>!v.seeking && v.currentTime>5 && v.currentTime<7'):break
                    page.wait_for_timeout(100)
                assert page.locator('#evidence-film').evaluate('v=>v.currentTime>5 && v.currentTime<7'), 'Scroll did not scrub the film'
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
        page.locator('[data-frame="0.5"]').click()
        expect(page.locator('#film-timeline')).to_have_value('50')
        page.evaluate('window.scrollBy(0,100)')
        page.wait_for_timeout(150)
        expect(page.locator('#film-timeline')).to_have_value('50')
        assert page.locator('.orb').evaluate("e=>getComputedStyle(e).animationName")=='none'
        assert page.locator('.scan-line').evaluate("e=>getComputedStyle(e).animationName")=='none'
        assert page.evaluate("document.fonts.check('16px \"Space Grotesk\"') && document.fonts.check('16px Manrope')")
        page.route('**/assets/evidence-film*.mp4',lambda route:route.abort())
        page.reload()
        page.locator('[data-frame="0.5"]').click()
        expect(page.locator('#film-hint')).to_contain_text('Film unavailable')
        assert page.locator('#film').evaluate("e=>e.classList.contains('film-unavailable')")
        assert not errors,errors
        browser.close()
    print(json.dumps({'result':'PASS','viewports':[1440,768,390,320],'checks':['production CSP','HTTP byte ranges','film decoding and chapter seek','scroll scrubbing','keyboard timeline','poster fallback','reduced-motion scroll disabled','screenshot tabs','coverage example','theme switch','three palettes and persistence','motion pause and persistence','reduced motion','self-hosted fonts','FAQ','no horizontal overflow','no JS errors']}))
finally:server.shutdown();server.server_close()
