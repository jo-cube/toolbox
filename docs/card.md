# card

`card` profiles approximate cardinality for explicit CSV columns, JSON Lines paths, or delimited fields.

It is a small profiler, not a query engine.

## Examples

CSV with a header row:

```sh
card --csv --columns user_id,country,plan users.csv
```

JSON Lines:

```sh
card --json .user_id .event_type .metadata.country events.jsonl
```

Delimited text with 1-based field numbers:

```sh
card --delimiter ',' --columns 1,2,7 data.txt
```

## Usage

```sh
card (--csv --columns a,b | --delimiter ',' --columns 1,2 | --json .path...) [file...]
```

Options:

- `--precision N`: HLL precision, from `4` to `20`; default is `14`
- `--output-json`: write JSON output

Default output:

```text
field              approx_unique  nulls  missing  empty  total
.user_id           918234         0      0        0      1000000
.event_type        42             0      0        0      1000000
.metadata.country  184            1203   320      0      1000000
```

## Notes

`card` counts non-empty, non-null, present values in its HLL sketches. It reports empty, null, missing, and total counts separately.

JSON paths are simple dot paths. They do not support filters, expressions, array traversal, or escaping.

