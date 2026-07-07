# Development

This page is for contributors working from a clone of the repository.

End-user docs live in the tool pages under `docs/`. Agent-specific workflow notes live in [`../AGENTS.md`](../AGENTS.md).

## Prerequisites

- Go 1.26.4 or newer
- RocksDB development headers and libraries when building or testing `rdbsh`
- macOS or Linux for `scripts/install.sh`

Install RocksDB headers on macOS:

```sh
brew install rocksdb
```

Install RocksDB headers on Ubuntu or Debian:

```sh
sudo apt-get update
sudo apt-get install -y librocksdb-dev
```

## Repository Layout

```text
cmd/                  CLI entrypoints and flag/output handling
internal/buildinfo/   build-time version string
internal/hello/       minimal reference CLI behavior
internal/ksetoff/     Kafka config parsing, offset planning, and commits
internal/rdbsh/       RocksDB shell behavior
internal/prob/        shared stream input and stable hashing helpers
internal/hll/         HyperLogLog implementation and state files
internal/bf/          Bloom filter implementation and state files
internal/card/        structured cardinality profiler
internal/heavy/       heavy-hitter detection
internal/sample/      stream sampling
docs/                 user and contributor documentation
scripts/install.sh    release installer
```

The usual split is:

- `cmd/<tool>/main.go` owns flags, help text, stdout/stderr formatting, and exit codes.
- `internal/<tool>/` owns behavior that can be tested without shelling out.
- `internal/prob/` owns shared line/NUL input handling and stable hashing for probabilistic tools.

Use `hello` as the minimal reference for that shape.

## Build

Build all CLIs into `./bin`:

```sh
make build
```

Install from source:

```sh
make install-hello
make install-ksetoff
make install-rdbsh
make install-hll
make install-bf
make install-card
make install-heavy
make install-sample
```

## Tests

Run cheap checks first:

```sh
gofmt -l .
sh -n scripts/install.sh
go test ./internal/hello ./internal/ksetoff ./cmd/hello ./cmd/ksetoff
go test ./internal/prob ./internal/hll ./internal/bf ./internal/card ./internal/heavy ./internal/sample ./cmd/hll ./cmd/bf ./cmd/card ./cmd/heavy ./cmd/sample
```

The full suite is:

```sh
go test ./...
```

`go test ./...` requires RocksDB development headers because `rdbsh` uses CGo.

## Implementation Notes

`ksetoff`:

- parses kcat/librdkafka-style config in `internal/ksetoff/config.go`
- plans offsets before committing them
- treats `-dry-run` as the safe default workflow for humans

`rdbsh`:

- keeps the CGo surface narrow in `internal/rdbsh/rocksdb`
- opens databases read-only unless `--writable` is set
- refuses export overwrite unless `--force` is set

Probabilistic tools:

- use the Go standard library only
- read streams without loading full inputs unless the selected algorithm requires it
- keep state-file compatibility constants in package code

Compatibility constants are the source of truth:

- `internal/prob.HashName`
- `internal/hll.Magic`, `internal/hll.Version`
- `internal/bf.Magic`, `internal/bf.Version`

Changing any of these can make old state files unreadable. Treat such changes as explicit file-format migrations.

## Release

GitHub Actions:

- build and test on pushes and pull requests
- cross-build pure Go binaries for Linux on `amd64` and `arm64`, plus macOS `arm64`
- build native `rdbsh` binaries for Linux on `amd64` and `arm64`, plus macOS `arm64`
- publish tarball release assets and matching SHA256 checksum files when a `v*` tag is pushed

Release assets are downloaded by `scripts/install.sh`.
The installer verifies SHA256 checksums before extracting archives.
