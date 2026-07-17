"""Real-browser desktop checks. Run after building .tools/NotSoFast-dev.exe.
Test-only dependencies: playwright. Uses installed Edge and Chrome on Windows.
"""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
import urllib.request

from playwright.sync_api import sync_playwright, expect

ROOT = Path(__file__).resolve().parents[1]
EXE = Path(os.environ.get('NSF_DESKTOP_EXE', ROOT / '.tools/NotSoFast-dev.exe')).resolve()


def run(channel='msedge'):
    with tempfile.TemporaryDirectory(prefix='ui-', dir=ROOT/'.tools') as temp:
        process = subprocess.Popen([str(EXE), '-data', temp, '-no-open'], stdout=subprocess.DEVNULL,
                                   creationflags=getattr(subprocess, 'CREATE_NO_WINDOW', 0))
        try:
            session = Path(temp) / 'session.json'
            for _ in range(200):
                if session.exists():
                    break
                if process.poll() is not None:
                    raise RuntimeError('Desktop exited during startup')
                time.sleep(.1)
            info = json.loads(session.read_text())
            with sync_playwright() as p:
                browser = p.chromium.launch(channel=channel, headless=True)
                page = browser.new_page(viewport={'width':1440, 'height':1050}, device_scale_factor=1)
                errors = []
                page.on('pageerror', lambda e: errors.append(str(e)))
                page.goto(info['url'] + '/#' + info['token'])
                expect(page.locator('#busy-bar')).to_be_hidden(timeout=20000)
                expect(page.locator('#overview-title')).to_be_visible()
                assert not page.evaluate("location.hash.includes('"+info['token']+"')")
                page.locator('#try-demo').click()
                expect(page.locator('#connection')).to_have_class('connection connected', timeout=45000)
                page.locator('nav [data-page="evidence"]').click()
                page.locator('#search-scope').fill('src')
                page.locator('#search-form button').click()
                expect(page.locator('#decision-badge')).to_have_text('UNKNOWN', timeout=45000)
                page.locator('#fill-gaps').click()
                expect(page.locator('#decision-badge')).to_have_text('REFUTED', timeout=45000)
                expect(page.locator('#witnesses')).to_contain_text('config/database.yaml')
                expect(page.locator('#busy-bar')).to_be_hidden()
                page.locator('#compose').click()
                expect(page.locator('#busy-bar')).to_be_hidden(timeout=45000)
                expect(page.locator('#decision-badge')).to_have_text('REFUTED')
                page.screenshot(path=str(ROOT/'docs/desktop-evidence.png'), full_page=True, animations='disabled')
                page.locator('nav [data-page="publish"]').click()
                page.locator('#file-path').fill('database.yaml')
                page.locator('#file-content').fill('duplicate')
                page.locator('#prepare-form button').click()
                expect(page.locator('#file-content')).to_be_disabled()
                expect(page.locator('#review-badge')).to_have_text('Check did not pass', timeout=45000)
                expect(page.locator('#publish-confirm')).to_be_disabled()
                page.locator('#file-path').fill('docs/release-notes.md')
                page.locator('#file-content').fill('# Release notes\n\nVerified from the desktop.\n')
                page.locator('#prepare-form button').click()
                expect(page.locator('#review-badge')).to_have_text('Ready to publish', timeout=45000)
                page.screenshot(path=str(ROOT/'docs/desktop-review.png'), full_page=True, animations='disabled')
                page.locator('#publish-confirm').click()
                expect(page.locator('#review-badge')).to_have_text('Published', timeout=45000)
                page.locator('#connect-top').click()
                with page.expect_download() as exported:
                    page.locator('#download-snapshot').click()
                import zipfile
                with zipfile.ZipFile(exported.value.path()) as archive:
                    assert archive.read('docs/release-notes.md').decode() == '# Release notes\n\nVerified from the desktop.\n'
                expect(page.locator('#busy-bar')).to_be_hidden()
                page.locator('nav [data-page="activity"]').click()
                page.locator('#full-activity .activity-row').filter(has_text='Guarded publication').first.click()
                page.locator('#recover').click()
                expect(page.locator('#detail-body')).to_contain_text('PUBLISHED', timeout=30000)
                expect(page.locator('#busy-bar')).to_be_hidden()
                page.locator('#close-detail').click()
                page.locator('nav [data-page="overview"]').click()
                expect(page.locator('#metric-published')).to_have_text('1')
                page.screenshot(path=str(ROOT/'docs/desktop-overview.png'), full_page=True, animations='disabled')
                page.reload()
                expect(page.locator('#metric-published')).to_have_text('1', timeout=20000)
                page.locator('nav [data-page="models"]').click()
                page.locator('#model-mode').select_option('provider')
                expect(page.locator('#discover-models')).to_be_hidden()
                page.locator('#model-mode').select_option('local')
                expect(page.locator('#discover-models')).to_be_visible()
                page.locator('#model-name').fill('test-config-only-no-model-call')
                page.locator('#model-form button[type="submit"]').click()
                expect(page.locator('#model-config-status')).to_contain_text('test-config-only-no-model-call',timeout=10000)
                page.screenshot(path=str(ROOT/'docs/desktop-models.png'),full_page=True,animations='disabled')
                page.locator('nav [data-page="overview"]').click()
                page.set_viewport_size({'width':390,'height':844})
                page.emulate_media(reduced_motion='reduce')
                assert page.locator('.orbit-one').evaluate('(e)=>getComputedStyle(e).animationName') == 'none'
                assert page.evaluate('document.documentElement.scrollWidth <= innerWidth')
                page.screenshot(path=str(ROOT/'docs/desktop-mobile.png'), full_page=True, animations='disabled')
                assert not errors, errors
                browser.close()
            request=urllib.request.Request(info['url']+'/api/quit',data=b'{}',headers={'X-NSF-Session':info['token']})
            with urllib.request.urlopen(request,timeout=10) as response:
                assert response.status == 200
            assert process.wait(timeout=20)==0
            print(json.dumps({'browser':channel,'result':'PASS','checks':['first run','sample import','partial UNKNOWN','missing REFUTED','composition','duplicate blocked','review','publication','recovery','reload','narrow viewport','reduced motion','no JS errors']}))
        finally:
            if process.poll() is None:
                process.terminate()
                process.wait(timeout=10)


if __name__ == '__main__':
    run(os.environ.get('NSF_BROWSER', 'msedge'))
