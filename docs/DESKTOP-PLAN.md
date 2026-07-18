# Desktop implementation and validation

## Design
User requested a polished animated frontend, double-click executable, and completion of outstanding validation. Build a local desktop workspace on the existing Go core. Embed static assets in Go; launch an Edge app window on Windows (browser fallback). Ship Windows executable and portable archive. No frontend framework/runtime or duplicate decision engine.

Views: overview with real metrics and animated evidence graph; evidence search with scope, composition and missing coverage; guided file creation with policy verification and explicit review; durable activity with operation recovery; workspace connection/settings. First-run folder picker and isolated sample repository. Persist selected workspace and activity in user app data; source working trees stay untouched. Explain where managed commits live.

Security: loopback only, exact Host/Origin, unguessable per-launch session header, restrictive CSP, bounded requests, serialized service switching, same core policy checks, no HTML interpolation of repository content. Desktop is a trusted local operator, not a multi-user security boundary. Reduced-motion, keyboard focus, labeled forms, mobile layout and meaningful errors required.

## Implementation plan
- [ ] Desktop Go handler and launcher; authenticated setup, workspace, existing dispatch, bounded durable history. Tests: unauthorized/cross-origin rejection, malformed/oversized input, demo setup, scan/publish/retry, restart.
- [ ] Embedded frontend: animated dashboard, evidence, review/publish, activity, settings; no mocked success/data. Browser test: first launch, demo, absence/refutation, successful publication, retry lookup, reload, narrow layout, reduced motion.
- [ ] Windows executable/build packaging, quiet subprocesses, portable Git if feasible, clean launch and shutdown.
- [ ] Complete feasible outstanding checks: full race/integration, repeated measurements and process resource usage, real MCP SDK compatibility, inspect remote CI. Docker requires a running engine; actual model trials require configured provider credentials; physical power-loss tests cannot be simulated as proof.
- [ ] Independent review, corrections, final verification and updated delivery report with exact artifacts and remaining external blockers.

Ruling: proceed inline under the user's explicit instruction to do all the work; no repeated design/plan permission gates. No push to shared branch or machine-wide virtualization installation. Existing source remains in this checkout.
