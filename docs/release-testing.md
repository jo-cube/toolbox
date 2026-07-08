# Release Testing

Use this workflow after publishing a release tag when you want to test the
released binaries without installing them on the host machine.

The release tests exercise the same path an end user takes:

- download `scripts/install.sh` from GitHub
- install release assets for a specific tag
- verify archive checksums through the installer
- run the installed binaries from an isolated directory
- test real command behavior in disposable Docker containers

## Prerequisites

- Docker
- network access to Docker Hub and GitHub releases
- a release tag, passed with `VERSION`

Use a concrete tag:

```sh
VERSION=v0.3.0
```

The scripts require `VERSION` so a release check cannot accidentally validate
an older tag.

## Smoke Test

Run the fast release smoke suite:

```sh
VERSION=v0.3.0 sh scripts/release-test/smoke.sh
```

This starts an `ubuntu:24.04` container, installs all eight released tools into
`/tmp/toolbox-bin`, and runs the per-tool scripts in
`scripts/release-test/smoke/`.

It covers:

- install and `--version` for every tool
- `--help` availability for every tool
- CLI usage errors and runtime errors where they are cheap to trigger
- documented output formats, including JSON and TSV where supported
- local validation for `ksetoff`
- real RocksDB reads, writes, prefix scans, export, hex input, quoted input,
  overwrite protection, and read-only checks for `rdbsh`
- stream options such as trimming, empty-record handling, and NUL-delimited
  input for tools that support them
- state-file compatibility and corrupted state rejection for `hll` and `bf`
- functional stream checks for `card`, `heavy`, and `sample`

It does not start a Kafka broker. Use the Kafka test below for real `ksetoff`
offset commits.

## Kafka Test For ksetoff

Run this when `ksetoff`, Kafka config parsing, release packaging, or supported
Kafka versions change:

```sh
VERSION=v0.3.0 sh scripts/release-test/ksetoff-kafka.sh
```

The test uses:

- `apache/kafka:4.3.1`
- one Kafka 4 KRaft broker, no ZooKeeper
- a Docker volume shared with a short-lived Ubuntu installer container
- the released `ksetoff` binary installed by `scripts/install.sh`
- Kafka's own `kafka-consumer-groups.sh` to verify committed offsets

It creates a two-partition topic, produces records, verifies `ksetoff -dry-run`
does not create or move the group, then commits `latest`, numeric offset `2`,
`beginning`, a timestamp offset, and partition-scoped `end`.

## Performance Smoke Test

Run this after the functional smoke test to catch obvious stream regressions:

```sh
VERSION=v0.3.0 sh scripts/release-test/perf.sh
```

This starts an `ubuntu:24.04` container, installs the stream-oriented tools,
and runs the scripts in `scripts/release-test/perf/`.

The numbers are smoke-test signals, not benchmark results. They should stay low
and roughly stable on the same machine.

## Layout

```text
scripts/release-test/
  install-tool.sh       shared release installer wrapper
  lib.sh                tiny assertion helpers
  smoke.sh              Docker runner for per-tool smoke tests
  smoke/<tool>.sh       per-tool functional checks
  perf.sh               Docker runner for stream performance checks
  perf/<tool>.sh        per stream-tool performance smoke checks
  ksetoff-kafka.sh      real Kafka integration check for ksetoff
```

There are no Dockerfiles. The tests use trusted upstream images directly:

- `ubuntu:24.04`
- `apache/kafka:4.3.1`

Add Dockerfiles only if repeated release testing becomes slow enough that a
cached custom image is worth maintaining.

## Cleanup

The Docker runners use `--rm` and the Kafka test removes its broker container,
network, and volume through a shell trap.

Check for leftovers:

```sh
docker ps -a --filter name=toolbox-release --format '{{.Names}}'
docker ps -a --filter name=toolbox-ksetoff-test --format '{{.Names}}'
docker network ls --filter name=toolbox-ksetoff-test --format '{{.Name}}'
docker volume ls --filter name=toolbox-ksetoff-test --format '{{.Name}}'
```

Remove images pulled only for release testing:

```sh
docker rmi ubuntu:24.04
docker rmi apache/kafka:4.3.1
```

If you create extra containers or images, report their names and cleanup
commands in your final notes.
