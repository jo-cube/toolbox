# hll

`hll` estimates distinct values in line-oriented input using HyperLogLog.

Use it when exact `sort | uniq | wc -l` is too slow or too memory-heavy.

## Examples

Count unique values from stdin:

```sh
awk '{print $1}' access.log | hll count
```

Build reusable state:

```sh
jq -r .user_id events.jsonl | hll build > users.hll
hll estimate users.hll
```

Merge compatible sketches:

```sh
hll merge monday.hll tuesday.hll > week.hll
hll inspect week.hll
```

## Commands

```sh
hll count [options] [file...]
hll build [options] [file...] > file.hll
hll estimate [--json] <file.hll>
hll merge <file.hll> <file.hll>... > merged.hll
hll inspect [--json] <file.hll>
```

Input options for `count` and `build`:

- `--trim`: trim surrounding whitespace
- `--ignore-empty`: skip empty items
- `-0`, `--nul`: read NUL-delimited items
- `--precision N`: HLL precision, from `4` to `20`; default is `14`

`hll count` prints:

```text
approx_unique=182934
relative_error=0.81%
```

## State Files

`.hll` files are binary and versioned. `hll merge` rejects files with incompatible precision, register count, version, or hash metadata.

Current hash: `fnv1a64-avalanche-v1`.

## Notes

Counts are approximate. The reported relative error is the configured HLL error estimate, not a measured error for the specific input.
