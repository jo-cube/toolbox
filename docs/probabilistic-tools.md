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

Use `-` to place stdin among file arguments. Stdin may appear only once:

```sh
hll count historical.txt - < live-values.txt
```

For tools that use the shared stream reader, these flags are available:

- `--trim`: trim surrounding whitespace before processing an item
- `--ignore-empty`: skip empty items
- `-0`, `--nul`: read NUL-delimited items instead of newline-delimited items

Defaults are conservative:

- one line is one item
- LF and CRLF line endings are recognized and are not part of the item
- case is preserved
- whitespace is preserved unless `--trim` is set
- empty lines are processed unless `--ignore-empty` is set

`sample` is the exception: it preserves emitted records exactly, so it does not trim or skip records. It supports newline and NUL delimiters.

## Field Selection

`hll count`, `hll build`, `heavy`, `bf build`, `bf test`, `bf dedupe`, and `sample --stable` can use one field from a literal-delimited record:

```sh
hll count -d $'\t' -f 2 events.tsv
heavy -d $'\t' -f 2 events.tsv
bf test -d $'\t' -f 2 users.bf events.tsv
sample --rate 0.01 --stable -d $'\t' -f 2 events.tsv
```

`-d`/`--delimiter` and `-f`/`--field` must be supplied together. Fields are 1-based. Missing fields are input errors; empty fields are valid values. `--trim` and `--ignore-empty`, where supported, apply after field selection. `bf test`, `bf dedupe`, and `sample` emit complete records even though the selected field controls the decision. This is intentionally not CSV or JSON parsing; use an upstream parser when quoting or structured data matters.

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

HLL and stable sampling use `fnv1a64-avalanche-v1`. Bloom filter version 2 uses `xxhash64-v1` with 256-bit split blocks.

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

State-file commands also accept `-` for stdin, which allows inspection or combination without a temporary file:

```sh
cat users.hll | hll inspect -
cat shard.bf | bf union baseline.bf - > combined.bf
```

Compatibility rules:

- HLL sketches can merge only when precision, register count, version, and hash metadata match.
- Bloom filters can union only when split-block bitset size, version, and hash metadata match.

## Composition

The tools intentionally do not read compressed files or parse every data format directly. Compose with existing tools:

```sh
zcat events.jsonl.gz | jq -r .user_id | hll count
```

```sh
awk '{print $7}' access.log | heavy --top 20
```

## Kafka Pipelines

These tools consume byte streams; they do not manage Kafka connections, offsets, checkpoints, or delivery semantics. Commands that emit a final summary or reusable state complete only at EOF, so use a bounded snapshot for them. For example, `jkq --snapshot` or `kcat -e` can bound a topic read.

Project each Kafka record to exactly one value before the operator. Newline-delimited output is convenient for text values. Use NUL-delimited output and `-0` when values may contain newlines; encode values first if they may contain NUL bytes.

A typical window produces one artifact per partition or time range, then combines compatible artifacts:

```sh
consume-window-a | project-key | hll build > window-a.hll
consume-window-b | project-key | hll build > window-b.hll
hll merge window-a.hll window-b.hll > combined.hll
```

Use `bf union` for Bloom-filter windows. Keep the same precision for HLL inputs and the same sizing parameters for Bloom-filter inputs. `heavy` and `sample` have no mergeable state; run them on the combined input stream. `bf dedupe` is useful for bounded best-effort duplicate suppression, but Bloom false positives mean it is not appropriate when every unique record must be retained.

For Kafka offset-shape analysis, use `kshape`; its canonical formatter preserves binary keys and Kafka metadata.
