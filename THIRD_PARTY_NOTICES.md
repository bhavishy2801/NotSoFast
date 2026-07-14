# Dependencies

- `github.com/mattn/go-sqlite3 v1.14.32`: MIT, Copyright 2014 Yasuhiro Matsumoto. Full license: [docs/go-sqlite3-LICENSE.txt](docs/go-sqlite3-LICENSE.txt). The embedded SQLite engine is public domain; see [SQLite copyright statement](https://sqlite.org/copyright.html).
- Go runtime/standard library: Go BSD-style license, distributed with the Go toolchain. The local downloaded toolchain is not part of this source release.
- Git is an external executable, GPLv2; it is not vendored into the project source. Container packaging installs the distribution's Git package with its notices.
- Python adapters use the standard library only. Container base images and CI actions retain their respective upstream licenses.

`go.sum` locks the downloaded Go module content. This notice is not a dependency vulnerability audit.
