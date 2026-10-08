# UI validation — 0.3.1

Tests run against the packaged Windows application using actual browser clicks and form submissions. Results describe exercised paths, not a guarantee that every possible workflow is defect-free.

## Layout fixes

The October 8 interface refresh applies to both desktop and local browser launchers. It embeds the product site's fonts, replaces the overview orbit illustration with a three-step workflow, updates toolbar icons and improves typography, controls, lists and dialog surfaces. The browser regression now verifies the embedded fonts and checks reduced motion against the active page rather than the removed illustration.

The native theme dialog lacked an inset scrollable body, clipping selection outlines and controls. All dialogs now share bounded sizing and scrollable content while keeping their headers and actions available. Short windows hide the decorative preview. A narrow-screen rule also hid Theme and Account along with the command trigger; it now targets only that trigger.

## Verified in browsers

| Check | Result |
| --- | --- |
| Eight application pages at 1280×720, 800×500, 390×600 and 320×480 | Passed; navigation uses visible controls |
| Five palettes and three brightness modes | Passed; theme controls and Save remain accessible |
| Profile backup download, preview and restore; malformed backup rejection | Passed |
| Individual bookmark removal | Passed |
| Model setup, trial run and JSON export | Passed with a scripted HTTP model fixture |
| Sample import, evidence coverage, composition, duplicate rejection, reviewed publication, recovery and reload | Passed in Chrome |
| Theme persistence, local profile, insights, file preview, reconnect and command palette | Passed |
| Public product site at 1440, 768, 390 and 320 pixels | Passed with production Content Security Policy |
| Product screenshot tabs, coverage example, light/dark switch and FAQ | Passed |
| Uncaught browser JavaScript errors in these checks | None |

Runnable checks: `python/test_desktop.py`, `python/test_layout.py` and `python/test_website.py`. Machine-readable results are in `browser-followup-results.json`, `layout-test-results.json` and `website-test-results.json` beside this document. Theme and website screenshots are also saved here.

## Remaining live integration checks

The rebuilt installer passed install, upgrade, registered uninstall, installed-app launch, junction rejection before deletion and saved-data retention. A deterministic Windows directory-handle test verifies the bounded lock retry and rejection of an existing destination. Run `python/test_installer.py` on an account without an existing registered installation; results are in `installer-results.json`.

- Supabase OAuth and deployed database policies need your project/provider configuration. Local profile saving works independently.
- Private GitHub repository authorization requires your own browser sign-in.
- Real model inference needs an available local model or a provider endpoint/key. The scripted UI test verifies transport and interaction, not model quality.
- Vercel deployment and uploaded GitHub release downloads need your account access. The static website was tested locally; it has not been published by this task.
- Installer binaries are unsigned. A signing identity is required for signed releases.

See [deployment instructions](DEPLOYMENT.md) for exact release and Vercel commands.
