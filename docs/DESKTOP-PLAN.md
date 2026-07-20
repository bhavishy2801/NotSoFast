# Desktop implementation and validation

## Design
User requested a polished animated frontend, double-click executable, and completion of outstanding validation. Build a local desktop workspace on the existing Go core. Embed static assets in Go; launch an Edge app window on Windows (browser fallback). Ship Windows executable and portable archive. No frontend framework/runtime or duplicate decision engine.

Views: overview with real metrics and animated evidence graph; evidence search with scope, composition and missing coverage; guided file creation with policy verification and explicit review; durable activity with operation recovery; workspace connection/settings. First-run folder picker and isolated sample repository. Persist selected workspace and activity in user app data; source working trees stay untouched. Explain where managed commits live.

Security: loopback only, exact Host/Origin, unguessable per-launch session header, restrictive CSP, bounded requests, serialized service switching, same core policy checks, no HTML interpolation of repository content. Desktop is a trusted local operator, not a multi-user security boundary. Reduced-motion, keyboard focus, labeled forms, mobile layout and meaningful errors required.

## Implementation plan
- [x] Desktop Go handler and launcher; authenticated setup, workspace, existing dispatch, bounded durable history. Tests: unauthorized/cross-origin rejection, malformed/oversized input, demo setup, scan/publish/retry, restart.
- [x] Embedded frontend: animated dashboard, evidence, review/publish, activity, settings; no mocked success/data. Browser test: first launch, demo, absence/refutation, successful publication, retry lookup, reload, narrow layout, reduced motion.
- [x] Windows executable/build packaging, quiet subprocesses, portable Git if feasible, clean launch and shutdown.
- [x] Complete feasible outstanding checks: full race/integration, repeated measurements and process resource usage, real MCP SDK compatibility, inspect remote CI. Docker requires a running engine; actual model trials require configured provider credentials; physical power-loss tests cannot be simulated as proof.
- [x] Independent review, corrections, final verification and updated delivery report with exact artifacts and remaining external blockers.

Ruling: proceed inline under the user's explicit instruction to do all the work; no repeated design/plan permission gates. No push to shared branch or machine-wide virtualization installation. Existing source remains in this checkout.

## October 6 follow-up
User requests broken-flow fixes, a better UI, GitHub URL imports with OAuth sign-in, a working browser edition, and a comprehensive README. Continue existing scope; do not replace the core verifier.

- [x] Reproduce packaged desktop/browser flows; browser launcher, session reconnect screen, persistent actionable errors.
- [x] GitHub public URL import and official GitHub CLI browser OAuth for private repositories; isolated CLI config, bounded bare history imports, no remote writes, cancellation and clear auth status. GitHub credentials never enter page JavaScript or history. Actual account authorization remains interactive.
- [x] Improve workspace selection, searchable tracked-file explorer and safe text preview, keyboard navigation, responsive readability and empty/loading/error states.
- [x] Add regression and integration coverage, review new boundaries, run actual public GitHub import, desktop/browser tests, remote CI/container tests where available.
- [x] Rewrite README with screenshots, desktop/web/GitHub/model onboarding, architecture, limits, troubleshooting, developer instructions, benchmarks and verified status. Rebuild final portable executable and web launcher.

External validation boundaries: no local Docker engine or actual model available. Prior remote CI was inspected; new CI steps await a remote run. Private GitHub account authorization must be completed by the user in GitHub.

## Themes, accounts and installer follow-up

Implemented system/light/dark brightness with five palettes; database-backed local profile and drafts; GitHub bookmarks; deduplicated local insights; optional Supabase GitHub/Google PKCE sign-in; explicit portable cloud snapshots and preview-before-restore. Cloud sessions remain in memory, and deployed authentication/RLS still require the user's project configuration. The cloud guide states exactly what is uploaded.

Built a native .NET Windows installer using the existing system compiler after automatic approval review blocked third-party compiler installation. Added per-user setup, shortcuts, upgrade handling, uninstall registration, manifest-based removal and saved-data preservation. Independent review identified junction traversal in cleanup; the implementation now rejects reparse points before modifying the installation.

Verified full Go race suite and vet, Edge and Chrome browser workflows, profile/theme persistence, scripted OAuth boundary checks, real public GitHub import with Git integrity check, and install/upgrade/launch/uninstall including junction rejection. An initial installer upgrade retest failed without a diagnostic log; logging was added, reproduction and the complete test then passed. The build remains unsigned.
