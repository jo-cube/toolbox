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
rank  count_estimate  item
1     823991          /api/search
2     712330          /api/login
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
    "count_estimate": 823991
  }
]
```

The field is named `count_estimate` even in exact mode so scripts can switch modes without changing parsers.

## Options

- `--top N`: number of results to print; default is `10`
- `--capacity N`: tracked item capacity for approximate mode; default is `max(top*10, 1000)`
- `--exact`: use exact counts with unbounded memory
- `--json`: write JSON output
- `--tsv`: write tab-separated output
- `--trim`: trim surrounding whitespace
- `--ignore-empty`: skip empty items
- `-0`, `--nul`: read NUL-delimited items
- `--version`: print version information

`--json` and `--tsv` are mutually exclusive.

## Input Model

By default, each input line is one item. The trailing newline is removed before counting.

Defaults:

- case is preserved
- surrounding whitespace is preserved
- empty lines are counted as a value
- no structured parsing is performed

Use tools such as `awk`, `cut`, or `jq` before `heavy` to select the field you want ranked.

## Approximate Mode

By default, `heavy` uses bounded-memory tracking.

Approximate mode can:

- overestimate counts
- miss low-frequency items
- produce approximate ordering near the cutoff

Increase `--capacity` when ordering quality matters. Use `--exact` when the input is small enough to keep every distinct value in memory.

## Exit Status

- `0`: success
- `1`: runtime error
- `2`: invalid command-line usage

## Contributor Notes

- CLI flags and output live in `cmd/heavy/main.go`.
- Exact and approximate behavior lives in `internal/heavy`.
- Shared input handling lives in `internal/prob`.
- Keep approximate mode bounded by default.
