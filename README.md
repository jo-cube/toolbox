# toolbox

`toolbox` is a Go monorepo for small, independent command-line tools.

The repository is structured so each CLI lives under `cmd/<name>`, shared code lives under `internal/`, and release
tooling can package and install a specific CLI without cloning the repository.

## Repository layout

```text
.
├── cmd/
│   └── hello/
│       └── main.go
├── internal/
│   ├── buildinfo/
│   └── hello/
├── scripts/
│   └── install.sh
├── .github/workflows/
├── Makefile
└── go.mod
```

## Included demo CLI

`hello` is a standalone Go binary built from `cmd/hello`.

It prints:

```text
Hello, world!
```

It also supports:

```sh
hello --version
```

## Prerequisites

- Go 1.25.4 or newer
- macOS for the provided installer script

## Build locally

Build the demo CLI into `./bin`:

```sh
make build
```

Build with an explicit version string:

```sh
make build VERSION=v0.1.0
```

## Run locally

Run the demo CLI without installing it:

```sh
make run-hello
```

Or run Go directly:

```sh
go run ./cmd/hello
go run ./cmd/hello --version
```

## Test

Run the test suite:

```sh
make test
```

## Install locally from source

Install `hello` into `~/.local/bin`:

```sh
make install-hello
```

Install somewhere else:

```sh
make install-hello LOCAL_BIN="$HOME/bin"
```

## Install a specific CLI from a GitHub release

The repository includes `scripts/install.sh`, which downloads a prebuilt release asset for a named CLI and installs it
locally.

Example using the script from this repo:

```sh
./scripts/install.sh hello
```

Install to a custom location:

```sh
./scripts/install.sh hello "$HOME/bin"
```

Install a specific release tag:

```sh
VERSION=v0.1.0 ./scripts/install.sh hello
```

Use it without cloning by running the published script directly from GitHub:

```sh
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | sh -s -- hello
```

The installer currently supports:

- `darwin/arm64`
- `darwin/amd64`

It downloads release assets named like:

```text
hello_darwin_arm64.tar.gz
hello_darwin_amd64.tar.gz
```

## Release behavior

GitHub Actions are set up to:

- build and test on pushes and pull requests
- cross-build macOS binaries for `amd64` and `arm64`
- publish tarball release assets when a `v*` tag is pushed

The release workflow currently packages the `hello` CLI. As you add more CLIs, extend the matrix or add a simple
packaging loop.

## Add another CLI later

1. Create a new entrypoint at `cmd/<cli-name>/main.go`.
2. Put reusable code in `internal/<package>` if it is shared across tools.
3. Add build and run targets to `Makefile` if you want dedicated shortcuts.
4. Update the release workflow to package the new CLI as `<cli-name>_<goos>_<goarch>.tar.gz`.
5. Document the new CLI in this README.

This keeps each CLI independent while preserving one module, one CI pipeline, and one release/install path for the whole
toolbox.
