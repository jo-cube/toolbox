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
```

`jkq` is the preferred producer. A compatible `kcat` pipeline is:

```sh
kcat -F kafka.conf -C -t events -o beginning -e -f "$(kshape format)" |
  kshape build > events.kshape
```

Use the canonical formatter without key deserializers or null-replacement
options. `%K` supplies the raw key length and `%k` supplies exactly those key
bytes. The parser reads the declared length before looking for the final record
newline, so tabs, newlines, NULs, and invalid UTF-8 inside keys are safe. A key
length of `-1` means a null key; `0` means a present, empty key.

The remaining fields are topic, partition, offset, timestamp in milliseconds,
and payload length. A timestamp of `-1` is missing. A payload length of `-1`
is a tombstone; `0` is an empty non-null payload. Payload bytes are never sent
to or read by `kshape`.

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

## Summary model

The default finest region is `1,048,576` offsets wide. `--bucket-width` on
`build` accepts another power of two. Each non-empty finest region stores:

- observed first and last offsets;
- visible record count and logical payload bytes;
- visible tombstone, null-key, and missing-timestamp counts;
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

- `visible_records` is the exact number of input frames observed.
- `observed_span` is the inclusive span from the first to last observed offset
  in the reported region.
- `observed_occupancy` is `visible_records / observed_span`. It is not a broker
  segment utilization or an authoritative low/high-watermark measurement.
- `logical_payload_bytes` is the sum of non-tombstone `%S` values. With `jkq`,
  this is the post-transform payload length. It is not compressed size, record
  overhead, segment size, or broker disk usage.
- `visible_tombstones` counts tombstones still present in the supplied stream,
  including tombstones produced by an upstream transformation.
- `approx_distinct_keys` is an HLL estimate over non-null keys. Null keys are
  counted separately; an empty non-null key participates in the estimate.
- `approx_visible_rewrite_factor` is non-null-key records divided by the
  approximate distinct-key count. It describes visible repetition, not all
  historical writes.

Fields without `approx` are exact with respect to the frames supplied to
`kshape`.

## Merge semantics

Artifacts must use the same topic, finest bucket width, HLL precision, format
version, and stable hash. Exact counters are added and HLL registers are
unioned.

Disjoint partition sets merge directly:

```sh
kshape merge partitions-0-7.kshape partitions-8-15.kshape > topic.kshape
```

Separately scanned ranges of one partition can also merge. If two artifacts
contain the same finest bucket, their observed first-to-last offset spans must
not overlap. This conservative rule prevents accidental double-counting; two
filtered scans whose observed spans overlap are not mergeable even when their
actual record sets happen to be disjoint.

An empty artifact is valid and can merge with a compatible artifact.

## What cannot be inferred

`kshape` sees only consumer-visible records that reached stdin. It cannot
reconstruct records already removed by compaction or retention, missed delete
markers, or prior versions of a visible key. Kafka compaction preserves record
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
big-endian. Readers validate configuration, ordering, counters, HLL state,
truncation, and trailing data. `internal/kshape.Magic`,
`internal/kshape.Version`, `internal/kshape.CanonicalFormat`, and
`internal/prob.HashName` are compatibility boundaries.
