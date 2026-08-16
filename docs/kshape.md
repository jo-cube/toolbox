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
kshape inspect [--json] [--bucket-width n] <topic.kshape>
kshape render [--title text] [--metric name] <topic.kshape> > report.html
kshape merge <a.kshape> <b.kshape>... > merged.kshape
```

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
kshape render events.kshape > events.html
```

`jkq` is the preferred producer. A compatible `kcat` pipeline is:

```sh
kcat -F kafka.conf -C -t events -o beginning -e -f "$(kshape format)" |
  kshape build > events.kshape
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
kshape render production-projection.kshape > production-projection.html
```

The report does not relabel transformed data as a raw Kafka topic view.

## Offline HTML report

`render` writes one self-contained HTML document to stdout. CSS, JavaScript,
and summary data are embedded; opening the file needs no Kafka connection,
network access, server, or external assets.

```sh
kshape render events.kshape > events.html
kshape render --title "Production events" --metric occupancy events.kshape > events.html
```

The main map has one row per observed partition. By default, each row spans
that partition's own first-to-last observed offset so internal structure stays
readable when partition ranges differ greatly. Shared-axis mode aligns every
row to the artifact-wide observed offset bounds. Hatched space in that mode is
outside a partition's observed bounds and is not presented as a known gap.

Select a partition and use the zoom controls to inspect a smaller offset
range. The report automatically switches among aligned power-of-two summary
levels. It embeds the finest level, an adaptive whole-view level of roughly 240
regions across the widest partition, and a midpoint level when those differ.
HLL sketches are merged while rendering and are not copied into the HTML.

The selectable map metrics are:

- **Visible records**: exact supplied record count per aligned region.
- **Observed occupancy**: exact supplied records divided by the first-to-last
  observed offset span inside that region.
- **Logical payload bytes**: exact non-tombstone payload-length sum, not Kafka
  broker storage.
- **Tombstone share**: exact supplied tombstones divided by supplied records in
  the region.
- **Approx. distinct keys**: the HLL estimate for non-null keys.
- **Approx. records / distinct key**: keyed records divided by the approximate
  distinct-key estimate; a visible repetition indicator, not compaction work.

Count, byte, approximate-key, and rewrite intensity can be normalized within
each partition or across the visible partitions. Occupancy and tombstone share
always use a fixed 0–100% scale. Color is backed by partition labels, a
numerical region inspector, a legend, and a partition comparison table.

Hover or click a colored region to see its aligned and observed offset bounds,
exact counters, approximate key metrics, and present timestamp range. Missing
timestamps remain explicitly missing. The report never invents broker
watermarks, disk sizes, records outside the supplied stream, or causes for
offset gaps.

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
