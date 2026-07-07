# toolbox

`toolbox` is a small collection of command-line tools released as separate binaries.

Each tool is meant to be installed and used directly. Some tools inspect local systems such as Kafka or RocksDB; others process streams in ordinary shell pipelines.

## Install

Install the latest release of a tool:

```sh
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | sh -s -- <tool>
```

Valid tool names: `hello`, `ksetoff`, `rdbsh`, `hll`, `bf`, `card`, `heavy`, `sample`.

For example:

```sh
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | sh -s -- ksetoff
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | sh -s -- rdbsh
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | sh -s -- hll
```

Install to a custom directory:

```sh
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | sh -s -- hll "$HOME/bin"
```

Install a specific release version:

```sh
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | VERSION=v0.1.0 sh -s -- hll
```

Release binaries are published for:

- `linux/amd64`
- `linux/arm64`
- `darwin/arm64`

## Tools

| Tool | What it does | Docs |
| --- | --- | --- |
| `hello` | Minimal reference CLI used as the simplest implementation example. | [`docs/hello.md`](docs/hello.md) |
| `ksetoff` | Set Kafka consumer group offsets for a topic without starting the consumer app. | [`docs/ksetoff.md`](docs/ksetoff.md) |
| `rdbsh` | Inspect local RocksDB databases interactively or with one-shot commands. | [`docs/rdbsh.md`](docs/rdbsh.md) |
| `hll` | Estimate unique values in large streams with HyperLogLog. | [`docs/hll.md`](docs/hll.md) |
| `bf` | Build and query Bloom filters for approximate membership tests. | [`docs/bf.md`](docs/bf.md) |
| `card` | Profile approximate cardinality for CSV, JSON Lines, and delimited fields. | [`docs/card.md`](docs/card.md) |
| `heavy` | Find frequent values in large streams with bounded memory or exact counting. | [`docs/heavy.md`](docs/heavy.md) |
| `sample` | Sample streams randomly, deterministically, or by reservoir count. | [`docs/sample.md`](docs/sample.md) |

## Quick Examples

Preview a Kafka offset reset:

```sh
ksetoff -F kafka.conf -group my-group -topic events -offset latest -dry-run
```

Inspect a RocksDB key:

```sh
rdbsh --db /tmp/store --exec "get 0x00000001"
```

Estimate unique users:

```sh
jq -r .user_id events.jsonl | hll count
```

Build and query a Bloom filter:

```sh
cat known-users.txt | bf build --expected-items 1000000 --false-positive-rate 0.001 > users.bf
cat candidates.txt | bf test users.bf
```

Profile JSON field cardinality:

```sh
card --json .user_id .tenant_id .event_type events.jsonl
```

Find frequent API paths:

```sh
awk '{print $7}' access.log | heavy --top 20
```

Take a stable sample:

```sh
sample --rate 0.01 --stable events.jsonl
```

## Behavior At A Glance

- `--version` prints the binary name and build version.
- Usage errors exit with status `2`.
- Runtime errors exit with status `1`.
- `ksetoff -dry-run` prints the offset plan and does not commit offsets.
- `rdbsh` opens databases read-only unless `--writable` is set.
- `rdbsh export <file>` refuses to overwrite an existing file unless `--force` is set.
- Probabilistic stream tools read from stdin by default and keep diagnostics on stderr.
- `hll` and `bf` state files are binary, versioned, and checked before use.

Shared behavior for `hll`, `bf`, `card`, `heavy`, and `sample` is documented in [`docs/probabilistic-tools.md`](docs/probabilistic-tools.md).

## From Source

Build all CLIs into `./bin`:

```sh
make build
```

Run a tool from source:

```sh
make run-hello
make run-ksetoff ARGS='-h'
make run-rdbsh ARGS='--db /path/to/db'
make run-hll ARGS='count values.txt'
make run-bf ARGS='inspect known.bf'
make run-card ARGS='--csv --columns user_id users.csv'
make run-heavy ARGS='--top 20 values.txt'
make run-sample ARGS='--rate 0.01 events.jsonl'
```

Contributor setup, package layout, tests, and implementation notes are in [`docs/development.md`](docs/development.md).

Agents should start with [`AGENTS.md`](AGENTS.md).
