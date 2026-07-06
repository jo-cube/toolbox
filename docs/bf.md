# bf

`bf` builds and queries Bloom filters for approximate membership tests.

A Bloom filter can say "definitely not present" or "probably present". False positives are possible. False negatives should not occur unless the filter file is corrupted or misused.

## Examples

Build a filter:

```sh
cat known-users.txt | bf build --expected-items 1000000 --false-positive-rate 0.001 > known.bf
```

Emit candidates probably present in the filter:

```sh
cat candidates.txt | bf test known.bf
```

Emit candidates definitely absent from the filter:

```sh
cat candidates.txt | bf test --invert known.bf
```

Inspect metadata:

```sh
bf inspect known.bf
```

## Commands

```sh
bf build --expected-items <n> --false-positive-rate <p> [file...] > filter.bf
bf test [--invert] <filter.bf> [file...]
bf inspect [--json] <filter.bf>
bf union <filter.bf> <filter.bf>... > combined.bf
```

Input options for `build` and `test`:

- `--trim`: trim surrounding whitespace
- `--ignore-empty`: skip empty items
- `-0`, `--nul`: read NUL-delimited items

## State Files

`.bf` files are binary and versioned. `bf union` requires compatible filters with the same bit count, hash count, and false-positive rate.

Current hash: `fnv1a64-avalanche-v1`.

## Notes

`--expected-items` and `--false-positive-rate` are required for `build`. Overfilling a Bloom filter increases the actual false-positive rate.
