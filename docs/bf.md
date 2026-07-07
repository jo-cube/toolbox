# bf

`bf` builds and queries Bloom filters for approximate membership tests.

A Bloom filter can say:

- definitely not present
- probably present

False positives are possible. False negatives should not occur unless the filter file is corrupted or misused.

## Install

Install the latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | sh -s -- bf
```

Install from a cloned repo:

```sh
./scripts/install.sh bf
```

Check the installed version:

```sh
bf --version
```

## Synopsis

```sh
bf build --expected-items <n> --false-positive-rate <p> [file...] > filter.bf
bf test [--invert] <filter.bf> [file...]
bf inspect [--json] <filter.bf>
bf union <filter.bf> <filter.bf>... > combined.bf
```

## Commands

### `bf build`

Builds a Bloom filter from input values and writes the binary filter to stdout.

```sh
cat known-users.txt | bf build --expected-items 1000000 --false-positive-rate 0.001 > users.bf
```

Required sizing flags:

- `--expected-items`: expected number of inserted items
- `--false-positive-rate`: target false-positive rate, greater than `0` and less than `1`

These flags are required because Bloom filter size and hash count depend on them. The tool does not guess production sizing.

### `bf test`

Tests input values against a saved filter.

Default behavior emits values that are probably present:

```sh
cat candidates.txt | bf test users.bf
```

Invert mode emits values that are definitely absent:

```sh
cat candidates.txt | bf test --invert users.bf
```

`bf test` writes matching input values to stdout, one per line.

### `bf inspect`

Prints filter metadata.

```sh
bf inspect users.bf
```

Example:

```text
type=bloom-filter
version=1
expected_items=1000000
inserted_items=982341
false_positive_rate=0.001
bit_count=14377588
hash_count=10
hash=fnv1a64-avalanche-v1
```

### `bf union`

Combines compatible Bloom filters and writes a new filter to stdout.

```sh
bf union service-a.bf service-b.bf > combined.bf
```

All filters must have compatible bit count, hash count, false-positive rate, version, and hash metadata.

## Options

Input options for `build` and `test`:

- `--trim`: trim surrounding whitespace
- `--ignore-empty`: skip empty items
- `-0`, `--nul`: read NUL-delimited items

Command options:

- `--expected-items N`: required by `build`
- `--false-positive-rate P`: required by `build`
- `--invert`: emit definitely absent values in `test`
- `--json`: write JSON output from `inspect`

## Input Model

By default, each input line is one item. The trailing newline is removed before hashing.

Defaults:

- case is preserved
- surrounding whitespace is preserved
- empty lines are inserted or tested as a value
- no structured parsing is performed

## Accuracy And Sizing

Bloom filters trade memory for false-positive probability.

If you insert more than `--expected-items`, the actual false-positive rate increases. If you need a lower false-positive rate, rebuild the filter with a lower `--false-positive-rate` value or a higher expected item count.

## State Files

`.bf` files are binary, versioned, and architecture-independent.

Current metadata:

- magic: `BLM1`
- version: `1`
- hash: `fnv1a64-avalanche-v1`
- bitset format: packed bits

`bf` validates the header before reading the full payload. Unsupported versions, unsupported hash names, invalid bit counts, invalid hash counts, and invalid bitset sizes fail clearly.

## Exit Status

- `0`: success
- `1`: runtime error or invalid state file
- `2`: invalid command-line usage

## Contributor Notes

- CLI flags and output live in `cmd/bf/main.go`.
- Bloom filter behavior and binary state files live in `internal/bf`.
- Shared input and hashing live in `internal/prob`.
- `internal/bf.Magic`, `internal/bf.Version`, and `internal/prob.HashName` are compatibility boundaries.
