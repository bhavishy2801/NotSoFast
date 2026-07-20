# Dependencies

- `github.com/mattn/go-sqlite3 v1.14.32`: MIT, Copyright 2014 Yasuhiro Matsumoto. Full license: [docs/go-sqlite3-LICENSE.txt](docs/go-sqlite3-LICENSE.txt). The embedded SQLite engine is public domain; see [SQLite copyright statement](https://sqlite.org/copyright.html).
- Go runtime/standard library: Go BSD-style license, distributed with the Go toolchain. The local downloaded toolchain is not part of this source release.
- Git is an external executable, GPLv2. The Windows portable package bundles MinGit 2.56.0 with its license and dependency notices. Matching upstream source: https://github.com/git-for-windows/git/releases/tag/v2.56.0.windows.1. Container packaging installs the distribution's Git package with its notices.
- Python adapters use the standard library only. Container base images and CI actions retain their respective upstream licenses.

`go.sum` locks the downloaded Go module content. This notice is not a dependency vulnerability audit.

- Windows packaging bundles GitHub CLI 2.102.0 (MIT) with its upstream license: https://github.com/cli/cli/releases/tag/v2.102.0. Authentication is provided by this upstream executable.
- Test-only tools include Playwright, psutil, pywin32 and the official MCP SDK; they are not included in the app runtime.
