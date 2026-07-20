param([string]$Go = 'go', [switch]$SkipGitDownload)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location $projectRoot
try {
    if (Test-Path '.tools/go/bin/go.exe') { $Go = (Resolve-Path '.tools/go/bin/go.exe').Path }
    $env:CGO_ENABLED = '1'
    $env:GOCACHE = Join-Path $projectRoot '.tools/gocache'
    $env:GOMODCACHE = Join-Path $projectRoot '.tools/gomod'
    $releaseDir = Join-Path $projectRoot 'dist/NotSoFast'
    New-Item -ItemType Directory -Force $releaseDir,'.tools/downloads' | Out-Null
    Push-Location cmd/notsofast
    try { & windres.exe app.rc -O coff -o resource_windows.syso; if ($LASTEXITCODE) { throw 'Windows resource compilation failed' } }
    finally { Pop-Location }
    & $Go build -trimpath -ldflags '-s -w -H windowsgui -extldflags=-static' -o "$releaseDir/NotSoFast.exe" ./cmd/notsofast
    if ($LASTEXITCODE) { throw 'Desktop build failed' }
    if (!$SkipGitDownload) {
        $gitZip = Join-Path $projectRoot '.tools/downloads/mingit.zip'
        if (!(Test-Path $gitZip)) { Invoke-WebRequest 'https://github.com/git-for-windows/git/releases/download/v2.56.0.windows.1/MinGit-2.56.0-64-bit.zip' -OutFile $gitZip }
        if ((Get-FileHash $gitZip -Algorithm SHA256).Hash.ToLower() -ne '064b440ff870ed5198527e8f3a92cdf5bd2fd0fedf5e718af95e3fdaddeff718') { throw 'MinGit checksum mismatch' }
        Expand-Archive -LiteralPath $gitZip -DestinationPath "$releaseDir/git" -Force
    }
    Copy-Item "$releaseDir/NotSoFast.exe" "$releaseDir/NotSoFast-Web.exe" -Force
    $ghZip = Join-Path $projectRoot '.tools/downloads/gh.zip'
    if (!(Test-Path $ghZip)) { Invoke-WebRequest 'https://github.com/cli/cli/releases/download/v2.102.0/gh_2.102.0_windows_amd64.zip' -OutFile $ghZip }
    if ((Get-FileHash $ghZip -Algorithm SHA256).Hash.ToLower() -ne 'ae64e556ecc240b200f7eba60d550e4bb60d78e860e69dd88c449405b86067f4') { throw 'GitHub CLI checksum mismatch' }
    Expand-Archive -LiteralPath $ghZip -DestinationPath "$releaseDir/github" -Force
    Copy-Item LICENSE,THIRD_PARTY_NOTICES.md -Destination $releaseDir -Force
    Copy-Item docs/go-sqlite3-LICENSE.txt -Destination $releaseDir -Force
    Copy-Item cloud -Destination $releaseDir -Recurse -Force
    @'
NotSoFast Desktop — Windows x64

1. Extract this entire folder, then double-click NotSoFast.exe.
2. Choose a sample, a local Git folder, or a GitHub URL in Workspace settings.
   Sign in with GitHub for private repositories; authorize the one-time code in your browser.
3. Explore evidence, or prepare a file in Guarded publish and review before publishing.
4. Download the managed snapshot in Workspace settings. Your source working tree is not edited.

Open NotSoFast-Web.exe for the complete website in your default browser.
The desktop window uses Microsoft Edge app mode, with a browser fallback.
The executable embeds all frontend assets. No Go, Node, Python, server setup or terminal is needed.
Keep the git and github folders beside both executables.
App data is stored in %APPDATA%\NotSoFast. Quit using the app's Quit button.
Closing the window leaves the local service available; reopening the executable returns to it.

Theme offers light, dark and system modes with five accent collections.
Account saves your profile and draft locally. Optional cloud snapshots require your Supabase
project configuration; see the included cloud/README.md and cloud/schema.sql.

Model lab supports an existing Ollama/LM Studio server or a compatible provider URL and model.
Keys are held in memory for this app session. Experiments send only disposable fixtures.

MinGit 2.56.0 source and licensing:
https://github.com/git-for-windows/git/releases/tag/v2.56.0.windows.1
See git/LICENSE.txt and bundled dependency notices.
GitHub CLI 2.102.0: https://github.com/cli/cli/releases/tag/v2.102.0
Disconnect removes this app's account configuration, not the GitHub OAuth grant.
'@ | Set-Content "$releaseDir/START-HERE.txt" -Encoding utf8
    Compress-Archive -Path $releaseDir -DestinationPath 'dist/NotSoFast-Windows-x64.zip' -Force
    Get-FileHash "$releaseDir/NotSoFast.exe",'dist/NotSoFast-Windows-x64.zip' -Algorithm SHA256 | Format-Table -AutoSize
} finally { Pop-Location }
