# toolbox

`toolbox` is a collection of small command-line tools.

The main way to use this repository is to install a released binary for the specific tool you want and run it locally on
your machine.

## Tools

| Tool      | What it does                                                                    | Install                        | Usage docs                           |
|-----------|---------------------------------------------------------------------------------|--------------------------------|--------------------------------------|
| `hello`   | Demo CLI that prints a friendly greeting.                                       | `./scripts/install.sh hello`   | This README                          |
| `ksetoff` | Bootstraps or resets Kafka consumer group offsets for a topic.                  | `./scripts/install.sh ksetoff` | [`docs/ksetoff.md`](docs/ksetoff.md) |
| `rdbsh`   | Inspect local RocksDB databases with an interactive shell or one-shot commands. | `./scripts/install.sh rdbsh`   | [`docs/rdbsh.md`](docs/rdbsh.md)     |

## Install A Tool

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

The installer supports release assets for the current platform and architecture.

Current release coverage for all tools:

- `darwin/amd64`
- `linux/arm64`
- `linux/amd64`

Release assets are published as:

```text
<tool>_darwin_amd64.tar.gz
<tool>_linux_arm64.tar.gz
<tool>_linux_amd64.tar.gz
```

## Tool Docs

- `hello`: prints `Hello, world!`
- `ksetoff`: see [`docs/ksetoff.md`](docs/ksetoff.md) for setup, config file format, examples, and troubleshooting
- `rdbsh`: see [`docs/rdbsh.md`](docs/rdbsh.md) for interactive usage, exports, column families, and build prerequisites

## Use `hello`

```sh
hello
hello --version
```

## Use `rdbsh`

```sh
rdbsh --db /path/to/db
rdbsh --db /path/to/db --exec "count"
rdbsh --db /path/to/db --cf offsets --exec "scan 0x00 20"
```

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
- cross-build pure Go binaries for Linux on `amd64` and `arm64`, plus macOS `amd64`
- build native `rdbsh` binaries for Linux on `amd64` and `arm64`, plus macOS `amd64`
- publish tarball release assets when a `v*` tag is pushed

To add another CLI later:

1. Create `cmd/<cli-name>/main.go`.
2. Put tool-specific logic in `internal/<cli-name>/`.
3. Add build and install shortcuts to `Makefile` if needed.
4. Update CI and release workflows to package the new CLI.
5. Add the tool to the table above and add dedicated docs when the tool needs more than a short usage note.
