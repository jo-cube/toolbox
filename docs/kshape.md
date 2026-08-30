# kshape

`kshape` builds a compact, mergeable summary of the Kafka records supplied to
it. Offset space is the coordinate system, and each partition is summarized at
multiple power-of-two resolutions.

It reads a formatted stream from `jkq` or a compatible `kcat` invocation. It
does not connect to Kafka.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/jo-cube/toolbox/main/scripts/install.sh | sh -s -- kshape
```

From a clone:

```sh
make install-kshape
```

Check the installed version:

```sh
kshape --version
```

## Commands

```text
kshape format
kshape build [--bucket-width n] [--precision p] > topic.kshape
kshape show <topic.kshape>
kshape inspect [--json] [--bucket-width n] <topic.kshape>
kshape render [--title text] [--metric name] <topic.kshape> > report.html
kshape merge <a.kshape> <b.kshape>... > merged.kshape
```

Artifact arguments accept `-` for stdin. A merge may read stdin at most once.

## Build from jkq

`kshape format` prints the exact binary-safe `-f` expression expected by
`kshape build`:

```text
%t\t%p\t%o\t%T\t%S\t%K\t%k\n
```

Use command substitution so the producer receives that expression unchanged:

```sh
jkq -F kafka.conf -t events --snapshot -f "$(kshape format)" |
  kshape build > events.kshape
kshape show events.kshape
kshape render events.kshape > events.html
```

`jkq` is the preferred producer. A compatible `kcat` pipeline is:

```sh
kcat -F kafka.conf -C -t events -o beginning -e -f "$(kshape format)" |
  kshape build > events.kshape
kshape show events.kshape
kshape render events.kshape > events.html
```

Use the canonical formatter without key deserializers or null-replacement
options such as kcat's `-Z`. `%K` supplies the raw key length and `%k` supplies
exactly those key bytes. The parser reads the declared length before looking for
the final record newline, so tabs, newlines, NULs, and invalid UTF-8 inside keys
are safe. A key length of `-1` means a null key; `0` means a present, empty key.

The remaining fields are topic, partition, offset, timestamp in milliseconds,
and payload length. A timestamp of `-1` is missing. A payload length of `-1`
is a tombstone; `0` is an empty non-null payload. Payload bytes are never sent
to or read by `kshape`.

Partitions may be interleaved, but offsets must be strictly increasing within
each partition. This is the default ordering provided by both tools. Do not
use jkq's `--unordered` option in a `kshape` pipeline. Duplicate or decreasing
offsets are rejected so record counts remain valid offset-occupancy counts.

## Post-transform profiles

The artifact describes records emitted into the pipeline, not an independent
view of the Kafka topic. Filtering removes records from the profile. A `jkq`
projection changes `%S` to the projected payload length, and a tombstone
transformation changes it to `-1`.

```sh
jkq -F kafka.conf -t events --snapshot \
  --drop-if 'environment != "production"' \
  --project '{"id": id, "total": $sum(items.price)}' \
  -f "$(kshape format)" |
  kshape build > production-projection.kshape
```

This is intentional: `production-projection.kshape` describes that selected,
projected stream. It does not claim to describe the source payloads that were
not emitted.

Render it in the same way as an untransformed profile:

```sh
kshape show production-projection.kshape
kshape render production-projection.kshape > production-projection.html
```

Both views describe the transformed output. They do not relabel it as a raw
Kafka topic view.

## Terminal view

`show` is the concise human and agent-readable view. It prints scale and key
repetition once, followed by one offset-density strip per partition:

```text
topic: events
partitions: 2  records: 26  keys: ~25  visible versions/key: ~1x
density: 54.2% of observed offset spans

offset density
p0  [@@@@@@@@@@@@@@@@@@@@@@@@]  24 records  100.0% dense
p1  [@                      @]  2 records  8.3% dense
```

Every ASCII character covers an equal slice of that partition's own observed
first-to-last offset span. The characters ` .:-=+*#%@` run from empty to dense.
The row also states its exact visible-record count and density, so the output
does not depend on interpreting the strip. No color, Unicode, or TTY detection
is used; redirected output is identical. A tombstone summary line appears only
when tombstones are present.

Use `show` for a quick terminal or SSH read, `render` for interactive regional
exploration, and `inspect` for detailed artifact fields, coarser bucket reports,
or JSON consumed by scripts.

## Offline HTML report

`render` writes one self-contained HTML document to stdout. CSS, JavaScript,
and summary data are embedded; opening the file needs no Kafka connection,
network access, server, or external assets.

```sh
kshape render events.kshape > events.html
kshape render --title "Production events" --metric churn events.kshape > events.html
```

The report opens with four orienting signals: visible records, visible density,
estimated distinct non-null keys, and visible versions per key. Tombstones get
a separate summary only when present. Payload totals, timestamps, null keys,
and missing timestamps stay under **Stream details** until requested.

The offset map is the main report. Its default **Visible density** view divides
exact supplied records by aligned offset positions inside each partition's
observed bounds. Dense and sparse regions therefore use a common 0–100% scale.
Each row normally spans that partition's own first-to-last observed offset so
internal structure stays readable. Shared alignment is available when absolute
offset positions across partitions matter; hatched space then means outside a
partition's observed bounds, not a known gap.

Select a partition and use the zoom controls to inspect a smaller offset
range. The report automatically switches among aligned power-of-two summary
levels. It embeds the finest level, an adaptive whole-view level of roughly 240
regions across the widest partition, and a midpoint level when those differ.
Zoom stops at the artifact's finest bucket width rather than implying
sub-bucket detail. HLL sketches are merged while rendering and are not copied
into the HTML.

The secondary map views answer narrower questions:

- **Visible key churn**: color begins above one surviving keyed record per
  estimated distinct key. It describes visible repetition, not historical
  writes or versions already removed by compaction.
- **Tombstone share**: exact supplied tombstones divided by supplied records in
  the region. The option is omitted when none are present.
- **Average payload size**: logical payload bytes divided by visible
  non-tombstone records. It is useful for regional payload differences, not
  broker disk usage.

Key-churn and payload colors share one maximum across visible partitions;
density and tombstone share use a fixed 0–100% scale. Select or hover over a
region to reveal semantically grouped offset/time, shape, key, tombstone, and
payload details. Partition totals are collapsed by default and retain only the
comparison fields that help explain skew.

`~` marks HLL-derived key values. Headline counts use compact notation such as
`12.4M`, while selected-region and `inspect` output retain exact counters. The
report never invents broker watermarks, disk sizes, records outside the supplied
stream, or causes for offset gaps.

## Summary model

The default finest region is `1,048,576` offsets wide. `--bucket-width` on
`build` accepts another power of two. Each non-empty finest region stores:

- observed first and last offsets;
- observed record count and logical payload bytes;
- observed tombstone, null-key, and missing-timestamp counts;
- minimum and maximum present timestamps;
- a HyperLogLog sketch of non-null raw keys.

Only the finest regions are updated while scanning. Whole-partition and wider
views are derived by merging neighboring regions. `inspect` defaults to one
summary per partition. Pass a power-of-two multiple of the artifact width for
a regional view:

```sh
kshape inspect --bucket-width 8388608 events.kshape
kshape inspect --json --bucket-width 8388608 events.kshape
```

In JSON, `report_bucket_width: 0` means whole-partition summaries.

`--precision` controls each region's HLL precision from 4 through 20. The
default is 10, which uses 1,024 one-byte registers per non-empty region and has
a theoretical relative standard error of about 3.25%. Higher precision uses
more artifact and working memory.

## Metric meanings

- `observed_records` is the exact number of input frames observed.
- `observed_span` is the inclusive span from the first to last observed offset
  in the reported region.
- `observed_occupancy` is `observed_records / observed_span`. It is not a broker
  segment utilization or an authoritative low/high-watermark measurement.
- `logical_payload_bytes` is the sum of non-tombstone `%S` values. With `jkq`,
  this is the post-transform payload length. It is not compressed size, record
  overhead, segment size, or broker disk usage.
- `observed_tombstones` counts tombstones present in the supplied stream,
  including tombstones produced by an upstream transformation.
- `keyed_records` is `observed_records - null_keys`.
- `approx_distinct_keys` is an HLL estimate over non-null keys. Null keys are
  counted separately; an empty non-null key participates in the estimate. The
  reported estimate is capped at `keyed_records`, an exact upper bound.
- `approx_keyed_records_per_distinct_key` is `keyed_records` divided by
  `approx_distinct_keys`. It describes repetition in the supplied stream, not
  original writes or compaction work.

Counter fields without `approx` are exact with respect to the frames supplied
to `kshape`.

## Merge semantics

Artifacts must use the same topic, finest bucket width, HLL precision, format
version, and stable hash. Exact counters are added and HLL registers are
unioned.

Disjoint partition sets merge directly:

```sh
kshape merge partitions-0-7.kshape partitions-8-15.kshape > topic.kshape
```

Separately scanned ranges of one partition can also merge. Each artifact keeps
sorted merge-guard spans for every finest bucket. Spans from different inputs
must not overlap. Disjoint spans remain distinct after a merge, so acceptance
does not depend on input order; adjacent spans are coalesced. This conservative
rule prevents accidental double-counting. Two filtered scans whose spans
overlap are not mergeable even when their actual emitted offset sets happen to
be disjoint.

The artifact does not record a transform or pipeline identity. Merge only
artifacts that belong to the same intended profile. `kshape` can verify topic,
configuration, and offset coverage, but it cannot tell whether two inputs used
the same jkq predicates or projection.

An empty artifact is valid and can merge with a compatible artifact.

## What cannot be inferred

`kshape` sees only records that reached stdin. It cannot
reconstruct records already removed by compaction or retention, missed delete
markers, or prior versions of a supplied key. Kafka compaction preserves record
offsets while removing selected records, so gaps show only that the supplied
stream did not contain records at those offsets; they do not identify a cause.

The artifact also contains no broker segment boundaries, physical byte sizes,
replica state, or low/high watermarks beyond bounds implied by observed
records. See the Apache Kafka documentation on
[log compaction](https://kafka.apache.org/43/design/design/#log_compaction) and
[topic retention settings](https://kafka.apache.org/43/configuration/topic-configs/)
for the broker-side semantics.

## Performance and artifact format

Input is processed in one streaming pass. Partitions may be arbitrarily
interleaved, and no global reordering or goroutines are added. Keys are hashed
incrementally through a fixed-size buffer and are not retained. Working memory
and artifact size grow with the number of partitions and non-empty finest
offset regions, not directly with the number of records.

`.kshape` files are deterministic for the same summary, binary, versioned, and
big-endian. The current format has magic `KSHP` and version `2`. Its header binds
the finest bucket width, HLL precision and version, and stable hash identity.
Sorted partition/region data contains exact counters, merge-guard spans, and an
HLL sketch for each non-empty finest region. A CRC32C trailer detects accidental
corruption. Readers reject unsupported versions, sketch or hash identities,
invalid ordering or counters, checksum mismatches, truncation, and trailing
data. Draft version 1 artifacts are intentionally rejected and must be rebuilt.

To bound allocation from corrupt files, readers accept at most 1,000,000
partitions, 1,000,000 non-empty regions, 10,000,000 merge-guard spans, and 1 GiB
of aggregate HLL register data. These are file-read limits, not claims about
Kafka itself.

Renderers and future comparison tools must treat the finest bucket width, HLL
version and precision, hash name, artifact version, and observed-stream meaning
as compatibility boundaries. Coarser levels may be derived only by merging
aligned neighboring finest-region summaries; approximate distinct-key sketches
are merged register-wise, while the other counters are exact sums.
