# AGENTS.md

This file is for coding agents and humans reviewing agent work in this repo.
Keep changes small, verify the behavior you touched, and report any local setup you create.

## Repo Map

- `cmd/hello`, `cmd/ksetoff`, `cmd/rdbsh`, `cmd/hll`, `cmd/bf`, `cmd/card`, `cmd/heavy`, `cmd/sample`: CLI entrypoints and flag handling.
- `internal/buildinfo`: build-time version string.
- `internal/hello`: demo command logic.
- `internal/ksetoff`: Kafka config parsing, client setup, offset planning, display, and commits.
- `internal/rdbsh`: shell commands, parsing, formatting, export behavior.
- `internal/rdbsh/rocksdb`: narrow CGo wrapper around the RocksDB C API.
- `internal/prob`: shared stream input and stable hashing helpers for probabilistic tools.
- `internal/hll`, `internal/bf`, `internal/card`, `internal/heavy`, `internal/sample`: probabilistic stream tool logic.
- `docs/`: user docs for the real tools.
- `scripts/install.sh`: release installer.
- `.github/workflows/`: CI and release packaging.

## Working Style

- Prefer small, boring, reviewable changes.
- Do not rewrite broad areas unless the human asked for that scope.
- Keep comments rare. Use names, small functions, and tests to explain ordinary behavior.
- Tests should freeze user-visible behavior and meaningful edge cases.
- Docs should be direct, accurate, and useful. Avoid hype, slogans, and feature marketing.

## Local Checks

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
On macOS, install them with:

```sh
brew install rocksdb
```

On Ubuntu or Debian, install them with:

```sh
sudo apt-get update
sudo apt-get install -y librocksdb-dev
```

## Docker Test Option

Use Docker only when a Docker runtime is available and host RocksDB headers are missing.
This is optional; do not make Docker a hard requirement for ordinary changes.

Use trusted base images. The current low-noise full-test workflow uses Docker's official Go image and installs only RocksDB headers in the container:

```sh
docker run \
  --name toolbox-rocksdb-test \
  -v "$PWD:/work:ro" \
  -w /work \
  golang:1.26-bookworm \
  bash -lc 'export DEBIAN_FRONTEND=noninteractive; apt-get update -qq >/dev/null && apt-get install -y -qq librocksdb-dev >/dev/null && /usr/local/go/bin/go test ./...'
```

Notes:

- Keep package-manager logs quiet; do not stream noisy logs into the conversation.
- Do not attach to long-running or noisy logs. Prefer one-shot commands whose exit status tells you whether the check passed.
- Mount the repo read-only when tests do not need to write to it.
- Use a named container so the human can inspect or remove it later.
- If the name already exists, choose a similarly specific new name and report it.
- If the first run downloads modules inside the container, that is expected.
- If you need repeated full test runs, you may commit the prepared container to a temporary local image to avoid reinstalling packages. Report the image name and size.
- Do not leave long-running containers behind.

If you create Docker containers or images, tell the human exactly what was created.
Do not remove them immediately unless asked; the human may want to inspect or reuse them while iterating.

Cleanup commands for the example names above:

```sh
docker rm toolbox-rocksdb-test
docker rmi golang:1.26-bookworm
```

If a temporary local image was created from a prepared container, remove dependent containers before removing the image:

```sh
docker rm <container-name>
docker rmi <temporary-image-name>
docker rmi golang:1.26-bookworm
```

If you create additional containers or temporary images, list them and include matching cleanup commands in your final message.

## Release Notes

Release assets are built by GitHub Actions and downloaded by `scripts/install.sh`.
Checksums and signatures are not currently implemented. Discuss the desired trust model before adding them.
