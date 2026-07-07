# card

`card` profiles approximate cardinality for explicit fields in CSV, JSON Lines, or delimited text.

Use it to answer questions such as:

- which fields have many distinct values?
- which fields are low-cardinality dimensions?
- which metric labels may be dangerous?
- which fields are good candidates for grouping?

`card` is a field profiler, not a query engine.

## Install

Install the latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | sh -s -- card
```

Install from a cloned repo:

```sh
./scripts/install.sh card
```

Check the installed version:

```sh
card --version
```

## Synopsis

```sh
card --csv --columns user_id,country,plan [file...]
card --delimiter ',' --columns 1,2,7 [file...]
card --json .user_id .event_type .metadata.country [file...]
```

## Modes

Exactly one input mode is required.

### CSV

CSV mode expects a header row and selects columns by name.

```sh
card --csv --columns user_id,country,plan users.csv
```

If a requested column is not present in the header, `card` fails.

### Delimited Text

Delimited mode selects fields by 1-based field number.

```sh
card --delimiter $'\t' --columns 1,3,5 data.tsv
```

Missing fields are counted in the `missing` column.

### JSON Lines

JSON mode reads one JSON object per line and selects explicit dot paths.

```sh
card --json .user_id .event_type .metadata.country events.jsonl
```

JSON paths are simple object paths. They do not support filters, expressions, array traversal, or escaping.

## Output

Default output is tabular:

```text
field              approx_unique  nulls  missing  empty  total
.user_id           918234         0      0        0      1000000
.event_type        42             0      0        0      1000000
.metadata.country  184            1203   320      0      1000000
```

JSON output:

```sh
card --output-json --json .user_id .event_type events.jsonl
```

```json
[
  {
    "field": ".user_id",
    "approx_unique": 918234,
    "total": 1000000
  }
]
```

JSON omits zero-valued `nulls`, `missing`, and `empty` fields.

## Options

- `--csv`: read CSV with a header row
- `--delimiter VALUE`: read delimited text using `VALUE` as the delimiter
- `--json`: read JSON Lines and treat leading `.path` arguments as selectors
- `--columns LIST`: comma-separated CSV column names or 1-based delimited fields
- `--precision N`: HLL precision from `4` to `20`; default is `14`
- `--output-json`: write JSON output
- `--version`: print version information

## Counting Rules

Each selected field keeps its own HyperLogLog sketch.

`approx_unique` counts present, non-null, non-empty values.

Separate counters report:

- `nulls`: JSON `null` values
- `missing`: missing JSON paths or missing delimited fields
- `empty`: empty strings
- `total`: total records observed for that field

For non-string JSON values, `card` hashes the JSON encoding of the value.

Malformed CSV or malformed JSON fails the command with a line-aware error where possible.

## Accuracy And Memory

At default precision `14`, each selected field uses `16,384` one-byte HLL registers and reports about `0.81%` relative error.

Memory grows with the number of selected fields, not with the number of input rows.

## Exit Status

- `0`: success
- `1`: runtime error or malformed input
- `2`: invalid command-line usage

## Contributor Notes

- CLI flags and output live in `cmd/card/main.go`.
- Profiling behavior lives in `internal/card`.
- HLL sketches live in `internal/hll`.
- Keep field selection explicit. `card` should not grow into a query language.
