# sample

`sample` emits a subset of input records while preserving emitted records exactly.

It supports:

- random rate sampling
- deterministic stable rate sampling
- fixed-count reservoir sampling

## Install

Install the latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | sh -s -- sample
```

Install from a cloned repo:

```sh
./scripts/install.sh sample
```

Check the installed version:

```sh
sample --version
```

## Synopsis

```sh
sample --rate <p> [--stable] [--seed n] [--delimiter value --field n] [-0] [file...]
sample --count <n> [--seed n] [-0] [file...]
```

Exactly one of `--rate` or `--count` is required.

## Examples

Random 1% sample:

```sh
sample --rate 0.01 events.jsonl
```

Stable 1% sample by full record:

```sh
sample --rate 0.01 --stable events.jsonl
```

Stable 1% cohort sample by the second tab-delimited field:

```sh
sample --rate 0.01 --stable -d $'\t' -f 2 events.tsv
```

Reproducible random sample:

```sh
sample --rate 0.01 --seed 12345 events.jsonl
```

Reservoir sample of 10,000 records:

```sh
sample --count 10000 huge-file.txt
```

## Modes

### Random Rate Sampling

`--rate P` emits each input record independently with probability `P`.

`P` must be from `0` to `1`, inclusive. `--rate 0` emits nothing. `--rate 1` emits every record.

Without `--seed`, random mode uses the current time as the seed.

### Stable Rate Sampling

`--rate P --stable` hashes each record and emits it when the hash falls below the rate threshold.

The same input record, rate, and seed produce the same decision across runs.

Stable mode hashes the full record without its trailing newline or NUL delimiter. With `--delimiter` and `--field`, it hashes only the selected field while emitting the complete record. Records with the same selected value therefore receive the same sampling decision.

### Reservoir Sampling

`--count N` keeps up to `N` records from the stream without knowing the stream length in advance.

Reservoir mode stores the selected records in memory and writes them after input is consumed.
Selected records are emitted in their original input order.

## Output

`sample` writes selected records to stdout exactly as they appeared in the input.

It does not:

- trim whitespace
- skip empty records
- parse JSON
- add a missing trailing newline

With `-0` or `--nul`, records and emitted delimiters are NUL-separated instead.

## Options

- `--rate P`: sample each record with probability `P`, from `0` to `1`
- `--count N`: keep up to a positive `N` records using reservoir sampling
- `--stable`: use deterministic hash sampling with `--rate`
- `--seed N`: seed random modes or stable hashing
- `-d`, `--delimiter VALUE`: literal field delimiter for stable sampling
- `-f`, `--field N`: 1-based field used for stable sampling
- `-0`, `--nul`: read and write NUL-delimited records
- `--version`, `-V`: print version information

Invalid combinations fail:

- `--rate` with `--count`
- `--stable` with `--count`
- a non-positive `--count`
- neither `--rate` nor `--count`
- field selection without `--stable`

`--delimiter` and `--field` must be supplied together. The delimiter is literal, not a regular expression or CSV parser. Missing fields are input errors; empty fields are valid sampling keys.

## Exit Status

- `0`: success
- `1`: runtime error
- `2`: invalid command-line usage

## Contributor Notes

- CLI flags live in `cmd/sample/main.go`.
- Sampling behavior lives in `internal/sample`.
- Stable hashing uses `internal/prob.Hash64`.
- Preserve records exactly. Do not switch to the shared trimmed stream reader for this tool.
