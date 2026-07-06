# heavy

`heavy` finds frequent values in a line-oriented stream.

By default it uses bounded-memory Space-Saving counts. Counts are estimates. Use `--exact` only when the input is small enough to hold all distinct values in memory.

## Examples

Top API paths:

```sh
awk '{print $7}' access.log | heavy --top 20
```

Top JSON field values:

```sh
jq -r .tenant_id events.jsonl | heavy --top 50
```

Exact counts for smaller input:

```sh
heavy --top 20 --exact values.txt
```

## Usage

```sh
heavy [options] [file...]
```

Options:

- `--top N`: number of results to print; default is `10`
- `--capacity N`: tracked item capacity for approximate mode
- `--exact`: use exact counts with unbounded memory
- `--json`: write JSON output
- `--tsv`: write tab-separated output
- `--trim`: trim surrounding whitespace
- `--ignore-empty`: skip empty items
- `-0`, `--nul`: read NUL-delimited items

Default output:

```text
rank  count_estimate  item
1     823991          /api/search
2     712330          /api/login
```

## Notes

Approximate mode may overestimate counts and may miss low-frequency items. Increase `--capacity` when ordering quality matters.

