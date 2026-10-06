# Deploy the product website to Vercel

Deploy only `website/`: the product page, screenshot tour, fixed-fixture coverage example and GitHub release links. Repository checks run in the installed desktop/local browser app. The public site requires no secrets or backend.

## Publish the desktop downloads first

Quit any app running from `dist/NotSoFast`, then build from the repository root:

```powershell
./scripts/build-desktop.ps1
./scripts/build-installer.ps1 -SkipDesktopBuild
Get-FileHash dist/NotSoFast-Setup.exe,dist/NotSoFast-Windows-x64.zip -Algorithm SHA256 |
  Select-Object Hash,@{n='File';e={Split-Path $_.Path -Leaf}} |
  ConvertTo-Json | Set-Content dist/checksums.json -Encoding utf8
```

Commit and push the reviewed source to GitHub. Then sign in locally and publish the assets:

```powershell
gh auth login
gh release create v0.3.1 `
  dist/NotSoFast-Setup.exe `
  dist/NotSoFast-Windows-x64.zip `
  dist/checksums.json `
  --repo bhavishy2801/NotSoFast `
  --target YOUR_PUSHED_COMMIT_SHA `
  --title 'NotSoFast 0.3.1' `
  --notes-file docs/RELEASE-0.3.1.md
```

Replace `YOUR_PUSHED_COMMIT_SHA` with the full output of `git rev-parse HEAD` after pushing that commit. Use an unused release tag. If GitHub CLI is not on PATH, this workspace includes `.tools/gh/bin/gh.exe`; the portable app includes `dist/NotSoFast/github/bin/gh.exe`.

The website links to the repository's Releases page, avoiding an invented direct asset URL. **Publish the assets before advertising downloads.** Download the uploaded installer and verify its checksum. Binaries are unsigned; signing needs your own code-signing identity.

## Vercel dashboard deployment

1. Sign in to Vercel and select **Add New → Project**.
2. Import `bhavishy2801/NotSoFast` and grant access to that repository.
3. Set **Root Directory** to `website`.
4. Select framework **Other**, leave install/build commands empty, and use output directory `.` if prompted.
5. Leave environment variables empty. Deploy and inspect the generated HTTPS URL.
6. Test the screenshot tabs, coverage example, theme switch, FAQ and download links before advertising the site.

With Git integration connected, pushes to the configured production branch trigger deployments. The included `website/vercel.json` supplies security headers and clean URL behavior.

Both supported project roots now have explicit static configuration: `website/vercel.json` serves `.`, while the repository-root `vercel.json` serves only `website`. You can therefore leave Root Directory at the repository root if preferred. Do not set it to `desktop/web` or `dist`.

### Repair an existing 404 deployment

Push these configuration files first, then inspect **Project → Settings → Build and Deployment**. Choose framework **Other** and Root Directory `website` (or leave it empty to use the repository-root configuration). Remove stale build/install/output overrides; the checked-in configuration sets empty commands and the correct output directory. Redeploy the latest commit—changing settings does not repair an older deployment.

Open the new deployment's own URL from its detail page. Check that its output contains `index.html`, `site.css`, `site.js` and `assets/`. If that URL works but your custom domain fails, check the domain's project assignment separately. If it still fails, provide that deployment URL, the selected root directory and its build log. A generic `404 NOT_FOUND` alone cannot distinguish missing output from a stale deployment or domain issue.

## Alternative: Vercel CLI

Requires Node.js/npm and your Vercel login. From the repository root:

```powershell
npx vercel@latest login
npx vercel@latest --cwd website
```

Choose your account/team and create or link a project using the static settings above. Inspect the preview URL printed by the CLI. Then publish production:

```powershell
npx vercel@latest --cwd website --prod
```

`.vercel/` project metadata is ignored by Git. These deployment commands have not been executed by this task; your account authorization is required. Do not deploy `desktop/web`, which requires the trusted local service.

## Optional custom domain

Use **Project → Settings → Domains**, add your domain, and apply exactly the DNS records Vercel displays at your registrar. Wait for verification and HTTPS. The provided `vercel.app` domain works without purchasing a domain.

## Local preview and checks

```powershell
python -m http.server 4173 --bind 127.0.0.1 --directory website
# Open http://127.0.0.1:4173
.tools/ui-venv/Scripts/python.exe python/test_website.py
$env:NSF_DESKTOP_EXE = "$PWD/dist/NotSoFast/NotSoFast.exe"
.tools/ui-venv/Scripts/python.exe python/test_layout.py
```

The website test exercises the real page with production CSP at four viewport sizes. The app test exercises eight pages, palette/brightness controls, offline backup round-trip and scripted model transport through the UI. Scripted transport is not real-model inference.

## What remains on your side

- Sign in to Vercel, choose the account/team and project name, and connect the repository.
- Commit/push the changes and publish the installer/portable assets in a GitHub release.
- Supply a domain and registrar access only if you want a custom domain.
- Separately, configure your Supabase project and OAuth providers for desktop cloud saves; follow [cloud setup](../cloud/README.md). These values do not belong in the static website.
- Configure a real model endpoint and authorize private GitHub access if you want those integrations validated live.

## Troubleshooting and boundaries

| Symptom | Check |
| --- | --- |
| Missing downloads | Publish release assets and ensure public visitors can access them. |
| 404 or source files shown | Root Directory must be `website`; output is `.`. |
| Disconnected app UI on Vercel | Wrong folder deployed. The local operator UI is not the public website. |
| Old screenshots | Update `website/assets` and redeploy. |

A future hosted repository workspace needs persistent storage and an appropriate per-user backend/security design. Do not expose the trusted desktop server publicly by weakening its Host, Origin or session checks.

Official references: [Vercel CLI deploy](https://vercel.com/docs/cli/deploy), [deployment overview](https://vercel.com/docs/deployments/overview), [domains](https://vercel.com/docs/domains/working-with-domains/add-a-domain), [function runtimes](https://vercel.com/docs/functions/configuring-functions).
