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
sample --rate <p> [--stable] [--seed n] [file...]
sample --count <n> [--seed n] [file...]
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

Stable mode hashes the full record without a trailing newline.

### Reservoir Sampling

`--count N` keeps up to `N` records from the stream without knowing the stream length in advance.

Reservoir mode stores the selected records in memory and writes them after input is consumed.

## Output

`sample` writes selected records to stdout exactly as they appeared in the input.

It does not:

- trim whitespace
- skip empty records
- parse JSON
- add a missing trailing newline

## Options

- `--rate P`: sample each record with probability `P`, from `0` to `1`
- `--count N`: keep up to `N` records using reservoir sampling
- `--stable`: use deterministic hash sampling with `--rate`
- `--seed N`: seed random modes or stable hashing
- `--version`: print version information

Invalid combinations fail:

- `--rate` with `--count`
- `--stable` with `--count`
- neither `--rate` nor `--count`

## Exit Status

- `0`: success
- `1`: runtime error
- `2`: invalid command-line usage

## Contributor Notes

- CLI flags live in `cmd/sample/main.go`.
- Sampling behavior lives in `internal/sample`.
- Stable hashing uses `internal/prob.Hash64`.
- Preserve records exactly. Do not switch to the shared trimmed stream reader for this tool.
