# Probabilistic Tools

This page documents behavior shared by `hll`, `bf`, `card`, `heavy`, and `sample`.

The individual tool pages are the command references:

- [`hll`](hll.md)
- [`bf`](bf.md)
- [`card`](card.md)
- [`heavy`](heavy.md)
- [`sample`](sample.md)

## Stream Input

Most tools read newline-delimited records from stdin by default:

```sh
cat values.txt | hll count
```

They also accept file arguments:

```sh
hll count values-a.txt values-b.txt
```

For tools that use the shared stream reader, these flags are available:

- `--trim`: trim surrounding whitespace before processing an item
- `--ignore-empty`: skip empty items
- `-0`, `--nul`: read NUL-delimited items instead of newline-delimited items

Defaults are conservative:

- one line is one item
- case is preserved
- whitespace is preserved unless `--trim` is set
- empty lines are processed unless `--ignore-empty` is set

`sample` is the exception: it preserves emitted records exactly, so it does not trim, skip, or reinterpret records.

## Output

Human-readable output is the default. Where supported, `--json` emits JSON and `--tsv` emits tab-separated output.

Diagnostics are written to stderr. Machine-readable stdout is not mixed with warnings or errors.

Common exit statuses:

- `0`: success
- `1`: runtime error, invalid input data, unreadable file, or incompatible state file
- `2`: invalid command-line arguments

## Approximation

`hll`, `bf`, `card`, and default `heavy` are approximate by design.

- `hll` and `card` estimate distinct values.
- `bf` may report false positives, but should not report false negatives unless the filter is corrupted or misused.
- `heavy` approximate mode can overestimate counts and can miss low-frequency items.
- `sample --rate` is probabilistic unless `--stable` is set.

The tools use estimate-oriented field names such as `approx_unique` and `count_estimate` to avoid implying exactness.

## Hashing

Tools that need deterministic hashing use `fnv1a64-avalanche-v1`.

The hash is stable across supported platforms and does not use Go's randomized map hashing. State files record hash metadata and reject unsupported hash names.

## State Files

`hll build` and `bf build` write binary state files.

State files are:

- binary
- versioned
- architecture-independent
- checked before use
- tied to a hash name and algorithm parameters

Do not edit state files by hand. Use `hll inspect` or `bf inspect` to view metadata.

Compatibility rules:

- HLL sketches can merge only when precision, register count, version, and hash metadata match.
- Bloom filters can union only when bit count, hash count, false-positive rate, version, and hash metadata match.

## Composition

The tools intentionally do not read compressed files or parse every data format directly. Compose with existing tools:

```sh
zcat events.jsonl.gz | jq -r .user_id | hll count
```

```sh
awk '{print $7}' access.log | heavy --top 20
```
