# hll

`hll` estimates distinct values in a stream using HyperLogLog.

Use it when exact counting with `sort | uniq | wc -l` would be too slow, too memory-heavy, or inconvenient for large inputs.

## Install

Install the latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | sh -s -- hll
```

Install from a cloned repo:

```sh
./scripts/install.sh hll
```

Check the installed version:

```sh
hll --version
```

## Synopsis

```sh
hll count [options] [file...]
hll build [options] [file...] > file.hll
hll estimate [--json] <file.hll>
hll merge <file.hll> <file.hll>... > merged.hll
hll inspect [--json] <file.hll>
```

## Commands

### `hll count`

Reads input values and prints an approximate unique count.

```sh
jq -r .user_id events.jsonl | hll count
```

Default output:

```text
approx_unique=182934
relative_error=0.81%
```

JSON output:

```sh
hll count --json values.txt
```

```json
{"approx_unique":182934,"relative_error":0.008125}
```

### `hll build`

Builds a reusable binary sketch and writes it to stdout.

```sh
jq -r .user_id events.jsonl | hll build > users.hll
```

Sketch files are binary. Redirect stdout to a file.

### `hll estimate`

Reads a saved sketch and prints the estimated cardinality.

```sh
hll estimate users.hll
```

### `hll merge`

Merges compatible sketches and writes the merged sketch to stdout.

```sh
hll merge monday.hll tuesday.hll > week.hll
```

All input sketches must use compatible precision, register count, file version, and hash metadata.

### `hll inspect`

Prints sketch metadata.

```sh
hll inspect users.hll
```

Example:

```text
type=hyperloglog
version=1
precision=14
registers=16384
hash=fnv1a64-avalanche-v1
approx_unique=182934
relative_error=0.81%
```

## Options

Input options for `count` and `build`:

- `--precision N`: HLL precision from `4` to `20`; default is `14`
- `--json`: write JSON output for `count`
- `--trim`: trim surrounding whitespace
- `--ignore-empty`: skip empty items
- `-0`, `--nul`: read NUL-delimited items

Output options:

- `--json`: available on `count`, `estimate`, and `inspect`

## Input Model

By default, each input line is one item. The trailing newline is removed before hashing.

Defaults:

- case is preserved
- surrounding whitespace is preserved
- empty lines are counted as a value
- no structured parsing is performed

Use tools such as `awk`, `cut`, or `jq` before `hll` to select the value you want counted.

## Accuracy And Memory

`hll` reports the configured relative error estimate, not a measured error for a specific dataset.

At the default precision `14`, the sketch uses `16,384` one-byte registers and reports about `0.81%` relative error.

Higher precision uses more memory and usually improves accuracy. Lower precision uses less memory and increases error.

## State Files

`.hll` files are binary, versioned, and architecture-independent.

Current metadata:

- magic: `HLL1`
- version: `1`
- hash: `fnv1a64-avalanche-v1`
- register format: one byte per register

`hll` validates the header before reading the full payload. Unsupported versions, unsupported hash names, and invalid register counts fail clearly.

## Exit Status

- `0`: success
- `1`: runtime error or invalid state file
- `2`: invalid command-line usage

## Contributor Notes

- CLI flags and output live in `cmd/hll/main.go`.
- HyperLogLog behavior and binary state files live in `internal/hll`.
- Shared input and hashing live in `internal/prob`.
- `internal/hll.Magic`, `internal/hll.Version`, and `internal/prob.HashName` are compatibility boundaries.
