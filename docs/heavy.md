# heavy

`heavy` finds frequent values in a stream.

Use it to inspect dominant API paths, error messages, tenants, hosts, keys, or other repeated values.

## Install

Install the latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | sh -s -- heavy
```

Install from a cloned repo:

```sh
./scripts/install.sh heavy
```

Check the installed version:

```sh
heavy --version
```

## Synopsis

```sh
heavy [options] [file...]
```

## Examples

Top API paths:

```sh
awk '{print $7}' access.log | heavy --top 20
```

Top JSON field values:

```sh
jq -r .tenant_id events.jsonl | heavy --top 50
```

Exact counts for smaller inputs:

```sh
heavy --top 20 --exact values.txt
```

## Output

Default output:

```text
rank  count_estimate  count_lower_bound  item
1     823991          823102             /api/search
2     712330          711908             /api/login
```

TSV output:

```sh
heavy --top 20 --tsv values.txt
```

JSON output:

```sh
heavy --top 20 --json values.txt
```

```json
[
  {
    "rank": 1,
    "item": "/api/search",
    "count_estimate": 823991,
    "count_lower_bound": 823102
  }
]
```

The true observed count is between `count_lower_bound` and `count_estimate`. In exact mode the two values are equal, so scripts can switch modes without changing parsers.

## Options

- `--top N`: number of results to print; default is `10`
- `--capacity N`: tracked item capacity for approximate mode; default is `max(top*10, 1000)`
- `--exact`: use exact counts with unbounded memory
- `--json`: write JSON output
- `--tsv`: write tab-separated output
- `-d`, `--delimiter VALUE`: literal field delimiter
- `-f`, `--field N`: 1-based field to rank
- `--trim`: trim surrounding whitespace
- `--ignore-empty`: skip empty items
- `-0`, `--nul`: read NUL-delimited items
- `--version`, `-V`: print version information

`--json` and `--tsv` are mutually exclusive.

## Input Model

By default, each input line is one item. The trailing newline is removed before counting.

Defaults:

- case is preserved
- surrounding whitespace is preserved
- empty lines are counted as a value
- no structured parsing is performed

Rank a literal-delimited field directly in either counting mode:

```sh
heavy --top 20 -d $'\t' -f 2 events.tsv
heavy --top 20 --exact -d $'\t' -f 2 events.tsv
```

The delimiter and field must be supplied together. Output items are the selected values. Trimming and empty-value filtering apply after selection. Missing fields fail; empty fields count unless `--ignore-empty` is set. Use `jq` or a CSV parser upstream for structured data.

## Approximate Mode

By default, `heavy` uses bounded-memory tracking.

Approximate mode can:

- overestimate counts
- miss low-frequency items
- produce approximate ordering near the cutoff

The lower bound accounts for uncertainty introduced when the bounded tracker replaces a low-frequency item.

Increase `--capacity` when ordering quality matters. Use `--exact` when the input is small enough to keep every distinct value in memory.

Updates in approximate mode take logarithmic time in the configured capacity, including when most input values are distinct.

## Exit Status

- `0`: success
- `1`: runtime error
- `2`: invalid command-line usage

## Contributor Notes

- CLI flags and output live in `cmd/heavy/main.go`.
- Exact and approximate behavior lives in `internal/heavy`.
- Shared input handling lives in `internal/prob`.
- Keep approximate mode bounded by default.
