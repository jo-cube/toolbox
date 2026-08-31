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
bf build --expected-items <n> --false-positive-rate <p> [--delimiter value --field n] [--no-size-limit] [file...] > filter.bf
bf test [--invert] [--delimiter value --field n] [--no-size-limit] <filter.bf> [file...]
bf dedupe --expected-items <n> --false-positive-rate <p> [--delimiter value --field n] [--no-size-limit] [file...]
bf inspect [--json] [--no-size-limit] <filter.bf>
bf union [--no-size-limit] <filter.bf> <filter.bf>... > combined.bf
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

If the input contains more than `--expected-items` values, `bf build` writes a warning to stderr after consuming the stream. The binary filter remains the only stdout output.

For literal-delimited records, build from one field without discarding the rest of the input schema:

```sh
bf build --expected-items 1000000 --false-positive-rate 0.001 \
  -d $'\t' -f 2 events.tsv > users.bf
```

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

`bf test` preserves the selected input delimiter: newline by default and NUL with `-0` or `--nul`.

The filter and candidate stream may each use `-` for stdin, but not at the same time.

Field selection tests one field and emits each complete matching record:

```sh
bf test -d $'\t' -f 2 users.bf candidates.tsv
```

### `bf dedupe`

Emits the first probably unseen occurrence of each input value without writing a state file:

```sh
cat events.txt | bf dedupe --expected-items 1000000 --false-positive-rate 0.0001
```

It uses bounded memory, but a Bloom false positive can discard a value that has not appeared before. Use exact tools when dropping a unique value is unacceptable.

The same field flags deduplicate by one field while preserving complete records:

```sh
bf dedupe --expected-items 1000000 --false-positive-rate 0.0001 \
  -d $'\t' -f 2 events.tsv
```

### `bf inspect`

Prints filter metadata.

```sh
bf inspect users.bf
```

Example:

```text
type=bloom-filter
version=2
expected_items=100
inserted_items=1
false_positive_rate=0.01
estimated_false_positive_rate=1.1346181979753662e-11
bit_count=1280
bitset_bytes=160
set_bits=8
fill_ratio=0.00625
hash=xxhash64-v1
```

### `bf union`

Combines compatible Bloom filters and writes a new filter to stdout.

```sh
bf union service-a.bf service-b.bf > combined.bf
```

All filters must have the same split-block bitset size, version, and hash metadata.

`inspect` scans the bitset without loading it into memory. `union` keeps the first filter in memory and streams each later filter into it.

## Options

Input options for `build`, `test`, and `dedupe`:

- `--trim`: trim surrounding whitespace
- `--ignore-empty`: skip empty items
- `-0`, `--nul`: read NUL-delimited items
- `-d`, `--delimiter VALUE`: literal delimiter between fields
- `-f`, `--field N`: 1-based field inserted, tested, or deduplicated

Command options:

- `--expected-items N`: required by `build` and `dedupe`
- `--false-positive-rate P`: required by `build` and `dedupe`
- `--no-size-limit`: allow filter bitsets larger than 2 GiB
- `--invert`: emit definitely absent values in `test`
- `--json`: write JSON output from `inspect`
- `--version`, `-V`: print version information

## Input Model

By default, each input line is one item. The trailing newline is removed before hashing.

Defaults:

- case is preserved
- surrounding whitespace is preserved
- empty lines are inserted or tested as a value
- no structured parsing is performed

`--delimiter` and `--field` must be supplied together. The delimiter is literal and may contain multiple bytes; it is not a regular expression or a CSV parser. A missing selected field is an input error, while an empty selected field is a value. With field selection, `--trim` and `--ignore-empty` apply to the selected field and output records remain complete. `-0` or `--nul` still controls record boundaries.

## Accuracy And Sizing

Bloom filters trade memory for false-positive probability.

The filter consists of 256-bit blocks with eight 32-bit lanes. One 64-bit hash selects a block and one bit in each lane, so a lookup reads only one small region of the bitset. Sizing uses the split-block false-positive model and rounds the bitset to a whole 32-byte block.

If you insert more than `--expected-items`, the actual false-positive rate increases. `inspect` reports the set-bit count, fill ratio, and an estimated current false-positive rate derived from the inserted-item count and bitset size. If you need a lower false-positive rate, rebuild the filter with a lower `--false-positive-rate` value or a higher expected item count.

`bf` limits a filter bitset to 2 GiB by default. Sizing requests and state files outside the format and platform limits fail before allocation.

`--no-size-limit` removes the 2 GiB application safeguard for any command that builds or reads a filter. It does not remove format or platform limits. Building, testing, and the first input to `union` still require the bitset to fit in memory; the operating system may terminate the process if memory is exhausted.

## State Files

`.bf` files are binary, versioned, and architecture-independent.

Current metadata:

- magic: `BLM1`
- version: `2`
- hash: `xxhash64-v1`
- bitset format: 256-bit split blocks with eight 32-bit lanes

Version 1 filters are not supported; rebuild them with the current `bf`. The command validates the header before reading the full payload. Unsupported versions, unsupported hash names, and invalid bitset sizes fail clearly. State-file arguments accept `-` for stdin; commands with multiple state inputs accept it at most once.

## Exit Status

- `0`: success
- `1`: runtime error or invalid state file
- `2`: invalid command-line usage

## Contributor Notes

- CLI flags and output live in `cmd/bf/main.go`.
- Bloom filter behavior and binary state files live in `internal/bf`.
- Shared input handling lives in `internal/prob`.
- `internal/bf.Magic`, `internal/bf.Version`, and `internal/bf.HashName` are compatibility boundaries.
- The 100M-item serial and parallel benchmarks allocate about 201 MiB and populate the full filter; run them explicitly when measuring large-filter behavior.
