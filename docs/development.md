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
internal/kshape/      observed Kafka stream summaries and state files
internal/rdbsh/       RocksDB shell behavior
internal/prob/        shared stream input and stable hashing helpers
internal/hll/         HyperLogLog implementation and state files
internal/bf/          Bloom filter implementation and state files
internal/card/        structured cardinality profiler
internal/heavy/       heavy-hitter detection
internal/sample/      stream sampling
docs/                 user and contributor documentation
scripts/install.sh    release installer
scripts/smoke-local.sh local binary version/help/exit checks
scripts/release-test/ Docker-based release validation for published binaries
```

The usual split is:

- `cmd/<tool>/main.go` owns flags, help text, stdout/stderr formatting, and exit codes.
- `internal/<tool>/` owns behavior that can be tested without shelling out.
- `internal/prob/` owns ordered file/stdin traversal, raw line/NUL records, value normalization and literal field selection, and the stable hashing used by HLL and sampling.

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
make install-kshape
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
sh -n scripts/install.sh scripts/smoke-local.sh
for f in $(find scripts/release-test -type f -name '*.sh' | sort); do sh -n "$f"; done
go test ./internal/hello ./internal/ksetoff ./cmd/hello ./cmd/ksetoff
go test ./internal/prob ./internal/hll ./internal/kshape ./internal/bf ./internal/card ./internal/heavy ./internal/sample ./cmd/kshape ./cmd/hll ./cmd/bf ./cmd/card ./cmd/heavy ./cmd/sample
```

The full suite is:

```sh
make test
```

`make test` requires RocksDB development headers because `rdbsh` uses CGo and
applies the detected RocksDB compiler and linker flags.

After building, run the small local CLI smoke suite:

```sh
TOOLBOX_BIN="$PWD/bin" sh scripts/smoke-local.sh
```

It checks field-selection pipelines, complementary sampling, version aliases, help output, representative exit statuses, and the `hello` output for the locally built binaries.

## Implementation Notes

`ksetoff`:

- parses kcat/librdkafka-style config in `internal/ksetoff/config.go`
- plans offsets before committing them
- treats `-dry-run` as the safe default workflow for humans

`kshape`:

- reads only the fixed formatter expression printed by `kshape format`
- summarizes one observed topic in power-of-two offset buckets
- stores exact counters and one mergeable HLL sketch per non-empty bucket
- retains disjoint merge-guard spans so overlap checks stay order-independent
- rejects duplicate or decreasing offsets within a partition
- protects version 2 artifacts with a CRC32C checksum
- prints a plain-ASCII density summary for terminals and logs
- renders a self-contained HTML report from adaptive, aligned summary levels

`rdbsh`:

- keeps the CGo surface narrow in `internal/rdbsh/rocksdb`
- opens databases read-only unless `--writable` is set
- refuses export overwrite unless `--force` is set

Probabilistic tools:

- use the Go standard library, except for Bloom filtering's direct XXHash64 dependency
- share file/stdin traversal; sampling preserves raw records while HLL, Bloom filtering, and heavy-hitter counting normalize selected values
- read streams without loading full inputs unless the selected algorithm requires it
- estimate HLL cardinality from its register histogram using Ertl's improved raw estimator
- keep state-file compatibility constants in package code

Compatibility constants are the source of truth:

- `internal/prob.HashName`
- `internal/hll.Magic`, `internal/hll.Version`
- `internal/bf.Magic`, `internal/bf.Version`, `internal/bf.HashName`
- `internal/kshape.Magic`, `internal/kshape.Version`, `internal/kshape.CanonicalFormat`

Changing any of these can make old state files unreadable. Treat such changes as explicit file-format migrations.

## Performance Checks

Run focused benchmarks serially so packages do not compete for CPU:

```sh
go test -p 1 ./internal/card ./internal/heavy -run '^$' -bench 'Benchmark(Profile|HighCardinality|RepeatedValues)$' -benchmem -benchtime=300ms -count=3
go test ./internal/rdbsh -run '^$' -bench '^BenchmarkReadCommands$' -benchmem -benchtime=300ms -count=3
```

The RocksDB command needs the same compiler/linker flags printed by `make test`
when headers or libraries are outside the default search paths. Its integration
tests and benchmarks use `ldb` or `rocksdb_ldb` from PATH to create temporary
fixtures, and skip when neither is available. Go removes these fixtures after
checking them. They ran with Homebrew's `rocksdb_ldb` for the measurements below.

### September 2026 Review

Baseline: `faf1ca9`, before the performance changes in `card`, `heavy`, and
`rdbsh`. Measurements used an Apple M4, Go 1.26.4, darwin/arm64, and RocksDB
11.8.1. The table gives medians of three 300 ms benchmark runs. Allocation
volume is Go `B/op`, shown in decimal MB; it is not peak RSS or native RocksDB
memory. Inputs and databases are prepared outside the timed loops. RocksDB
output goes to `/dev/null`, so these measurements include write syscalls but
exclude database startup and persistent output storage.

| Benchmark workload | Before → after (ms/op) | Before → after (MB allocated/op) |
| --- | --- | --- |
| CSV, 10k rows, all 3 columns | 0.872 → 0.741 | 0.774 → 0.294 |
| CSV, 10k rows, first of 100 columns | 10.929 → 10.274 | 26.911 → 8.991 |
| Delimited, 10k rows, first of 100 columns | 8.983 → 1.027 | 28.181 → 10.261 |
| Delimited, 10k rows, last of 100 columns | 9.126 → 8.739 | 28.181 → 10.261 |
| Delimited, 10k rows, all 100 columns | 19.136 → 16.806 | 29.822 → 11.903 |
| Heavy, 100k distinct values, capacity 1k | 19.186 → 17.358 | 1.820 → 1.820 |
| RocksDB count, 10k entries, 4 KiB values | 4.588 → 1.509 | 41.282 → 0.240 |
| RocksDB keys, 10k entries, 4 KiB values | 8.778 → 5.827 | 41.618 → 0.560 |
| RocksDB JSON export, 10k entries, 32-byte values | 14.944 → 3.274 | 2.082 → 1.446 |
| RocksDB JSON export, 10k entries, 4 KiB values | 53.053 → 41.801 | 131.659 → 82.882 |

The changes remove work without changing CLI output or state formats:

- CSV profiling enables the standard reader's record-slice reuse. Each row is
  consumed before the next read; header indexes are resolved beforehand.
- Delimited profiling orders selectors once, scans forward with `strings.Cut`,
  and stops after the last selected field. It preserves requested output order,
  duplicate selectors, empty fields, and missing-field counts without building
  a slice of every field for every row. Line reading still consumes each full row.
- RocksDB key-only commands skip value retrieval and C-to-Go copies. Scans and
  exports still read values, and iterator errors and prefix/limit checks remain.
- JSON export uses a buffered writer and one standard JSON encoder, eliminating
  per-entry encoded-byte copies and most small writes. Formatting and atomic
  file publication remain unchanged, including propagation of flush failures.
- Heavy tracking replaces the heap root in place with `heap.Fix`, avoiding a
  pop followed by a push. The existing logarithmic update algorithm, lexical
  eviction tie-break, and count bounds remain intact.

Whole CLI comparisons used identical pre-generated files, equal build flags,
startup-inclusive elapsed time, one warm-up pair, and five measured pairs with
alternating before/after execution order. All stdout and stderr matched:

| CLI workload | Before → after median |
| --- | --- |
| CSV, 200k rows, user_id/country/plan from the release performance workload | 18.06 → 15.53 ms |
| Delimited, 200k rows, first of 100 columns | 205.36 → 46.27 ms |
| Delimited, 200k rows, last of 100 columns | 208.92 → 198.68 ms |
| Heavy, 500k sequential distinct values, top 20 | 103.56 → 93.93 ms |

A `strings.SplitSeq` trial regressed late-column selection on wide rows by
about 13%; it was replaced with the smaller cursor loop. Heavy's skewed and
single-value controls showed little change, so its gains apply primarily to
replacement-heavy streams. No pooling, unsafe borrowing, custom parser, or
additional concurrency was needed.

The review retained the shared borrowed-record reader, bounded sampling,
HLL histogram estimator, split-block Bloom layout, streaming Bloom union,
and buffered/streaming `kshape` artifact and report code. Stable hash and file
compatibility contracts rule out casually swapping hashes. Further work should
start with actual profiles of large `kshape` renders (which revisit sketches
at multiple resolutions), structured JSON profiling, or large-value exports.
Exact heavy counting still intentionally retains all distinct values. This
review did not benchmark live Kafka requests or published Linux release assets.

## Release

GitHub Actions:

- build and test on pushes and pull requests
- cross-build pure Go binaries for Linux on `amd64` and `arm64`, plus macOS `arm64`
- build native `rdbsh` binaries for Linux on `amd64` and `arm64`, plus macOS `arm64`
- publish tarball release assets and matching SHA256 checksum files when a `v*` tag is pushed

Release assets are downloaded by `scripts/install.sh`.
The installer verifies SHA256 checksums before extracting archives.
It also runs the extracted `rdbsh --version` before installation so missing or incompatible RocksDB runtime libraries fail with setup guidance.

After publishing a release, validate the published binaries in Docker without
installing them on the host:

```sh
VERSION=v0.3.0 sh scripts/release-test/smoke.sh
VERSION=v0.3.0 sh scripts/release-test/ksetoff-kafka.sh
VERSION=v0.3.0 sh scripts/release-test/perf.sh
```

The release-test workflow and cleanup expectations are documented in
[`release-testing.md`](release-testing.md).
