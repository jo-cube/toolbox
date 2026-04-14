# toolbox

`toolbox` is a small Go monorepo for command-line utilities.

Each tool is released as its own binary. Most users will install the specific tool they need and run it locally.

## Tools

| Tool      | What it does                                                                    | Docs                                 |
|-----------|---------------------------------------------------------------------------------|--------------------------------------|
| `hello`   | Demo CLI that prints a friendly greeting.                                       | This README                          |
| `ksetoff` | Bootstraps or resets Kafka consumer group offsets for a topic.                  | [`docs/ksetoff.md`](docs/ksetoff.md) |
| `rdbsh`   | Inspect local RocksDB databases with an interactive shell or one-shot commands. | [`docs/rdbsh.md`](docs/rdbsh.md)     |

## Install

Install the latest release of a tool:

```sh
./scripts/install.sh ksetoff
```

Install to a custom directory:

```sh
./scripts/install.sh ksetoff "$HOME/bin"
```

Install a specific release version:

```sh
VERSION=v0.1.0 ./scripts/install.sh ksetoff
```

Install without cloning the repository:

```sh
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | sh -s -- ksetoff
```

Release binaries are published for:

- `linux/amd64`
- `linux/arm64`
- `darwin/arm64`

Release asset names follow this pattern:

```text
<tool>_linux_amd64.tar.gz
<tool>_linux_arm64.tar.gz
<tool>_darwin_arm64.tar.gz
```

## Quick Usage

`hello` is the simplest tool in the repo:

```sh
hello
hello --version
```

For everything else, use the dedicated tool docs:

- [`docs/ksetoff.md`](docs/ksetoff.md)
- [`docs/rdbsh.md`](docs/rdbsh.md)

## Repository Layout

```text
.
├── cmd/
│   ├── hello/
│   │   └── main.go
│   ├── ksetoff/
│   │   └── main.go
│   └── rdbsh/
│       └── main.go
├── docs/
│   ├── ksetoff.md
│   └── rdbsh.md
├── internal/
│   ├── buildinfo/
│   ├── hello/
│   ├── ksetoff/
│   └── rdbsh/
│       └── rocksdb/
├── scripts/
│   └── install.sh
├── .github/workflows/
├── Makefile
└── go.mod
```

## For Contributors

Prerequisites:

- Go 1.25.4 or newer
- RocksDB development headers and libraries when building `rdbsh` from source
- macOS or Linux for the installer script

Source build notes for `rdbsh`:

- On macOS, `brew install rocksdb`
- On Ubuntu or Debian, `sudo apt-get install librocksdb-dev`
- The `Makefile` auto-detects RocksDB via `pkg-config` when available and falls back to common Homebrew paths

Build all CLIs into `./bin`:

```sh
make build
```

Run tests:

```sh
make test
```

Run from source:

```sh
make run-hello
make run-ksetoff ARGS='-h'
make run-rdbsh ARGS='--db /path/to/db'
```

Install from source:

```sh
make install-hello
make install-ksetoff
make install-rdbsh
```

GitHub Actions:

- build and test on pushes and pull requests
- cross-build pure Go binaries for Linux on `amd64` and `arm64`, plus macOS `arm64`
- build native `rdbsh` binaries for Linux on `amd64` and `arm64`, plus macOS `arm64`
- publish tarball release assets when a `v*` tag is pushed

To add another CLI later:

1. Create `cmd/<cli-name>/main.go`.
2. Put tool-specific logic in `internal/<cli-name>/`.
3. Add build and install shortcuts to `Makefile` if needed.
4. Update CI and release workflows to package the new CLI.
5. Add the tool to the table above and add dedicated docs when the tool needs more than a short usage note.
