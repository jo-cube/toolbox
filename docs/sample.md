# sample

`sample` emits a subset of input records while preserving emitted lines exactly.

It supports random rate sampling, deterministic stable rate sampling, and fixed-count reservoir sampling.

## Examples

Random 1% sample:

```sh
sample --rate 0.01 events.jsonl
```

Stable 1% sample by full line:

```sh
sample --rate 0.01 --stable events.jsonl
```

Reservoir sample of 10,000 records:

```sh
sample --count 10000 huge-file.txt
```

Reproducible random sample:

```sh
sample --rate 0.01 --seed 12345 events.jsonl
```

## Usage

```sh
sample (--rate <p> [--stable] | --count <n>) [file...]
```

Options:

- `--rate P`: independently sample each record with probability `P`, from `0` to `1`
- `--count N`: keep exactly up to `N` records using reservoir sampling
- `--stable`: use deterministic hash sampling with `--rate`
- `--seed N`: seed random modes or stable hashing

## Notes

`--rate` and `--count` are mutually exclusive. `--stable` applies only to `--rate`.

Stable sampling hashes the full line without the trailing newline.

