package kshape

import (
	"fmt"
	"math"
	"sort"

	"github.com/jo-cube/toolbox/internal/hll"
	"github.com/jo-cube/toolbox/internal/prob"
)

const (
	DefaultBucketWidth uint64 = 1 << 20
	DefaultPrecision   uint8  = 10
)

type Summary struct {
	Topic       string
	BucketWidth uint64
	Precision   uint8
	Partitions  map[int32]*Partition
}

type Partition struct {
	Regions    map[uint64]*Region
	lastOffset int64
	hasOffset  bool
}

type OffsetSpan struct {
	FirstOffset int64
	LastOffset  int64
}

type Region struct {
	Bucket            uint64
	FirstOffset       int64
	LastOffset        int64
	Records           uint64
	PayloadBytes      uint64
	Tombstones        uint64
	NullKeys          uint64
	MissingTimestamps uint64
	MinTimestamp      int64
	MaxTimestamp      int64
	Coverage          []OffsetSpan
	Keys              *hll.Sketch
}

type Report struct {
	Type                string            `json:"type"`
	Version             uint8             `json:"version"`
	Topic               string            `json:"topic"`
	ArtifactBucketWidth uint64            `json:"artifact_bucket_width"`
	ReportBucketWidth   uint64            `json:"report_bucket_width"`
	HLLPrecision        uint8             `json:"hll_precision"`
	HLLVersion          uint8             `json:"hll_version"`
	Hash                string            `json:"hash"`
	Checksum            string            `json:"checksum"`
	HLLRelativeError    float64           `json:"hll_relative_error"`
	Partitions          []PartitionReport `json:"partitions"`
}

type PartitionReport struct {
	Partition int32          `json:"partition"`
	Regions   []RegionReport `json:"regions"`
}

type RegionReport struct {
	RegionFirstOffset   uint64  `json:"region_first_offset"`
	RegionLastOffset    uint64  `json:"region_last_offset"`
	ObservedFirstOffset int64   `json:"observed_first_offset"`
	ObservedLastOffset  int64   `json:"observed_last_offset"`
	ObservedSpan        uint64  `json:"observed_span"`
	ObservedRecords     uint64  `json:"observed_records"`
	ObservedOccupancy   float64 `json:"observed_occupancy"`
	LogicalPayloadBytes uint64  `json:"logical_payload_bytes"`
	ObservedTombstones  uint64  `json:"observed_tombstones"`
	NullKeys            uint64  `json:"null_keys"`
	KeyedRecords        uint64  `json:"keyed_records"`
	MissingTimestamps   uint64  `json:"missing_timestamps"`
	MinTimestamp        *int64  `json:"min_timestamp"`
	MaxTimestamp        *int64  `json:"max_timestamp"`
	ApproxDistinctKeys  uint64  `json:"approx_distinct_keys"`
	ApproxRecordsPerKey float64 `json:"approx_keyed_records_per_distinct_key"`
}

func New(bucketWidth uint64, precision uint8) (*Summary, error) {
	if err := validateConfig(bucketWidth, precision); err != nil {
		return nil, err
	}
	return &Summary{
		BucketWidth: bucketWidth,
		Precision:   precision,
		Partitions:  make(map[int32]*Partition),
	}, nil
}

func (s *Summary) Add(record Record) error {
	if s.Topic == "" {
		if err := validateTopic(record.Topic); err != nil {
			return err
		}
	} else if s.Topic != record.Topic {
		return fmt.Errorf("topic %q differs from %q", record.Topic, s.Topic)
	}
	if record.Partition < 0 || record.Offset < 0 {
		return fmt.Errorf("partition and offset must be non-negative")
	}
	if record.PayloadLength < -1 {
		return fmt.Errorf("payload length must be -1 or non-negative")
	}
	if record.Timestamp < -1 {
		return fmt.Errorf("timestamp must be -1 or non-negative")
	}

	partition := s.Partitions[record.Partition]
	if partition != nil && partition.hasOffset && record.Offset <= partition.lastOffset {
		return fmt.Errorf("partition %d offset %d is not greater than previous offset %d", record.Partition, record.Offset, partition.lastOffset)
	}
	if partition == nil {
		partition = &Partition{Regions: make(map[uint64]*Region)}
	}
	bucket := uint64(record.Offset) / s.BucketWidth
	region := partition.Regions[bucket]
	if region == nil {
		keys, err := hll.New(s.Precision)
		if err != nil {
			return err
		}
		region = &Region{
			Bucket:      bucket,
			FirstOffset: record.Offset,
			LastOffset:  record.Offset,
			Coverage:    []OffsetSpan{{FirstOffset: record.Offset, LastOffset: record.Offset}},
			Keys:        keys,
		}
	}
	if region.Records == math.MaxUint64 ||
		(record.PayloadLength >= 0 && uint64(record.PayloadLength) > math.MaxUint64-region.PayloadBytes) {
		return fmt.Errorf("counter overflow")
	}

	if s.Topic == "" {
		s.Topic = record.Topic
	}
	if s.Partitions[record.Partition] == nil {
		s.Partitions[record.Partition] = partition
	}
	if partition.Regions[bucket] == nil {
		partition.Regions[bucket] = region
	}
	knownTimestamps := region.Records - region.MissingTimestamps
	region.Records++
	if record.PayloadLength == -1 {
		region.Tombstones++
	} else {
		region.PayloadBytes += uint64(record.PayloadLength)
	}
	if record.NullKey {
		region.NullKeys++
	} else {
		region.Keys.AddHash(record.KeyHash)
	}
	if record.Timestamp == -1 {
		region.MissingTimestamps++
	} else if knownTimestamps == 0 {
		region.MinTimestamp = record.Timestamp
		region.MaxTimestamp = record.Timestamp
	} else {
		region.MinTimestamp = min(region.MinTimestamp, record.Timestamp)
		region.MaxTimestamp = max(region.MaxTimestamp, record.Timestamp)
	}
	region.FirstOffset = min(region.FirstOffset, record.Offset)
	region.LastOffset = max(region.LastOffset, record.Offset)
	region.Coverage[len(region.Coverage)-1].LastOffset = record.Offset
	partition.lastOffset = record.Offset
	partition.hasOffset = true
	return nil
}

func (s *Summary) Merge(other *Summary) error {
	if s == nil || other == nil {
		return fmt.Errorf("cannot merge nil summary")
	}
	if s.BucketWidth != other.BucketWidth || s.Precision != other.Precision {
		return fmt.Errorf("incompatible bucket width or HLL precision")
	}
	if s.Topic != "" && other.Topic != "" && s.Topic != other.Topic {
		return fmt.Errorf("incompatible topics %q and %q", s.Topic, other.Topic)
	}
	for partitionID, otherPartition := range other.Partitions {
		partition := s.Partitions[partitionID]
		if partition == nil {
			continue
		}
		for bucket, otherRegion := range otherPartition.Regions {
			region := partition.Regions[bucket]
			if region == nil {
				continue
			}
			if coverageOverlaps(region.Coverage, otherRegion.Coverage) {
				return fmt.Errorf("partition %d bucket %d has overlapping observed offset spans", partitionID, bucket)
			}
			if err := canMerge(region, otherRegion); err != nil {
				return fmt.Errorf("partition %d bucket %d: %w", partitionID, bucket, err)
			}
		}
	}
	if s.Topic == "" {
		s.Topic = other.Topic
	}
	for partitionID, otherPartition := range other.Partitions {
		partition := s.Partitions[partitionID]
		if partition == nil {
			s.Partitions[partitionID] = clonePartition(otherPartition)
			continue
		}
		for bucket, otherRegion := range otherPartition.Regions {
			region := partition.Regions[bucket]
			if region == nil {
				partition.Regions[bucket] = cloneRegion(otherRegion)
				continue
			}
			if err := mergeRegion(region, otherRegion); err != nil {
				return err
			}
		}
		partition.lastOffset = max(partition.lastOffset, otherPartition.lastOffset)
		partition.hasOffset = partition.hasOffset || otherPartition.hasOffset
	}
	return nil
}

func (s *Summary) Report(bucketWidth uint64) (Report, error) {
	if bucketWidth != 0 && (bucketWidth < s.BucketWidth || bucketWidth%s.BucketWidth != 0 || !powerOfTwo(bucketWidth/s.BucketWidth)) {
		return Report{}, fmt.Errorf("report bucket width must be zero or a power-of-two multiple of %d", s.BucketWidth)
	}
	sketch, _ := hll.New(s.Precision)
	report := Report{
		Type:                "kafka-retained-log-shape",
		Version:             Version,
		Topic:               s.Topic,
		ArtifactBucketWidth: s.BucketWidth,
		ReportBucketWidth:   bucketWidth,
		HLLPrecision:        s.Precision,
		HLLVersion:          hll.Version,
		Hash:                prob.HashName,
		Checksum:            ChecksumName,
		HLLRelativeError:    sketch.RelativeError(),
		Partitions:          make([]PartitionReport, 0, len(s.Partitions)),
	}
	partitionIDs := sortedPartitions(s.Partitions)
	for _, partitionID := range partitionIDs {
		partition := s.Partitions[partitionID]
		buckets := sortedRegions(partition.Regions)
		factor := uint64(0)
		if bucketWidth != 0 {
			factor = bucketWidth / s.BucketWidth
		}
		capacity := 0
		if factor == 0 {
			capacity = 1
		} else if factor == 1 {
			capacity = len(buckets)
		}
		partitionReport := PartitionReport{
			Partition: partitionID,
			Regions:   make([]RegionReport, 0, capacity),
		}
		if factor == 1 {
			for _, bucket := range buckets {
				partitionReport.Regions = append(partitionReport.Regions, makeRegionReport(partition.Regions[bucket], bucket, bucketWidth))
			}
		} else {
			var group uint64
			var region *Region
			for _, bucket := range buckets {
				nextGroup := uint64(0)
				if factor != 0 {
					nextGroup = bucket / factor
				}
				if region != nil && nextGroup != group {
					partitionReport.Regions = append(partitionReport.Regions, makeRegionReport(region, group, bucketWidth))
					region = nil
				}
				if region == nil {
					group, region = nextGroup, cloneRegion(partition.Regions[bucket])
				} else if err := mergeMetrics(region, partition.Regions[bucket]); err != nil {
					return Report{}, err
				}
			}
			if region != nil {
				partitionReport.Regions = append(partitionReport.Regions, makeRegionReport(region, group, bucketWidth))
			}
		}
		report.Partitions = append(report.Partitions, partitionReport)
	}
	return report, nil
}

func makeRegionReport(region *Region, group, bucketWidth uint64) RegionReport {
	first, last := uint64(region.FirstOffset), uint64(region.LastOffset)
	if bucketWidth != 0 {
		first = group * bucketWidth
		last = min(first+bucketWidth-1, uint64(math.MaxInt64))
	}
	span := uint64(region.LastOffset-region.FirstOffset) + 1
	keyedRecords := region.Records - region.NullKeys
	distinctKeys := min(region.Keys.Estimate(), keyedRecords)
	report := RegionReport{
		RegionFirstOffset:   first,
		RegionLastOffset:    last,
		ObservedFirstOffset: region.FirstOffset,
		ObservedLastOffset:  region.LastOffset,
		ObservedSpan:        span,
		ObservedRecords:     region.Records,
		ObservedOccupancy:   float64(region.Records) / float64(span),
		LogicalPayloadBytes: region.PayloadBytes,
		ObservedTombstones:  region.Tombstones,
		NullKeys:            region.NullKeys,
		KeyedRecords:        keyedRecords,
		MissingTimestamps:   region.MissingTimestamps,
		ApproxDistinctKeys:  distinctKeys,
	}
	if region.MissingTimestamps != region.Records {
		minTimestamp, maxTimestamp := region.MinTimestamp, region.MaxTimestamp
		report.MinTimestamp = &minTimestamp
		report.MaxTimestamp = &maxTimestamp
	}
	if distinctKeys != 0 {
		report.ApproxRecordsPerKey = float64(keyedRecords) / float64(distinctKeys)
	}
	return report
}

func validateConfig(bucketWidth uint64, precision uint8) error {
	if !powerOfTwo(bucketWidth) || bucketWidth > 1<<63 {
		return fmt.Errorf("bucket width must be a power of two between 1 and %d", uint64(1)<<63)
	}
	if _, err := hll.New(precision); err != nil {
		return err
	}
	return nil
}

func powerOfTwo(value uint64) bool {
	return value != 0 && value&(value-1) == 0
}

func canMerge(a, b *Region) error {
	if a == nil || b == nil || a.Keys == nil || b.Keys == nil ||
		a.Keys.Precision != b.Keys.Precision || len(a.Keys.Registers) != len(b.Keys.Registers) {
		return fmt.Errorf("incompatible HLL sketches")
	}
	for _, pair := range [][2]uint64{
		{a.Records, b.Records},
		{a.PayloadBytes, b.PayloadBytes},
		{a.Tombstones, b.Tombstones},
		{a.NullKeys, b.NullKeys},
		{a.MissingTimestamps, b.MissingTimestamps},
	} {
		if pair[1] > math.MaxUint64-pair[0] {
			return fmt.Errorf("counter overflow")
		}
	}
	return nil
}

func mergeRegion(dst, src *Region) error {
	coverage := mergeCoverage(dst.Coverage, src.Coverage)
	if err := mergeMetrics(dst, src); err != nil {
		return err
	}
	dst.Coverage = coverage
	return nil
}

func mergeMetrics(dst, src *Region) error {
	if err := canMerge(dst, src); err != nil {
		return err
	}
	if err := dst.Keys.Merge(src.Keys); err != nil {
		return err
	}
	dstKnown, srcKnown := dst.Records-dst.MissingTimestamps, src.Records-src.MissingTimestamps
	dst.FirstOffset = min(dst.FirstOffset, src.FirstOffset)
	dst.LastOffset = max(dst.LastOffset, src.LastOffset)
	dst.Records += src.Records
	dst.PayloadBytes += src.PayloadBytes
	dst.Tombstones += src.Tombstones
	dst.NullKeys += src.NullKeys
	dst.MissingTimestamps += src.MissingTimestamps
	if dstKnown == 0 && srcKnown != 0 {
		dst.MinTimestamp, dst.MaxTimestamp = src.MinTimestamp, src.MaxTimestamp
	} else if srcKnown != 0 {
		dst.MinTimestamp = min(dst.MinTimestamp, src.MinTimestamp)
		dst.MaxTimestamp = max(dst.MaxTimestamp, src.MaxTimestamp)
	}
	return nil
}

func cloneRegion(region *Region) *Region {
	clone := *region
	clone.Coverage = append([]OffsetSpan(nil), region.Coverage...)
	clone.Keys, _ = hll.New(region.Keys.Precision)
	_ = clone.Keys.Merge(region.Keys)
	return &clone
}

func clonePartition(partition *Partition) *Partition {
	clone := &Partition{
		Regions:    make(map[uint64]*Region, len(partition.Regions)),
		lastOffset: partition.lastOffset,
		hasOffset:  partition.hasOffset,
	}
	for bucket, region := range partition.Regions {
		clone.Regions[bucket] = cloneRegion(region)
	}
	return clone
}

func coverageOverlaps(a, b []OffsetSpan) bool {
	for i, j := 0, 0; i < len(a) && j < len(b); {
		if a[i].LastOffset < b[j].FirstOffset {
			i++
		} else if b[j].LastOffset < a[i].FirstOffset {
			j++
		} else {
			return true
		}
	}
	return false
}

func mergeCoverage(a, b []OffsetSpan) []OffsetSpan {
	spans := make([]OffsetSpan, 0, len(a)+len(b))
	for len(a) != 0 || len(b) != 0 {
		var next OffsetSpan
		if len(b) == 0 || len(a) != 0 && a[0].FirstOffset < b[0].FirstOffset {
			next, a = a[0], a[1:]
		} else {
			next, b = b[0], b[1:]
		}
		if len(spans) != 0 && next.FirstOffset == spans[len(spans)-1].LastOffset+1 {
			spans[len(spans)-1].LastOffset = next.LastOffset
		} else {
			spans = append(spans, next)
		}
	}
	return spans
}

func sortedPartitions(partitions map[int32]*Partition) []int32 {
	ids := make([]int32, 0, len(partitions))
	for id := range partitions {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func sortedRegions[T any](regions map[uint64]T) []uint64 {
	buckets := make([]uint64, 0, len(regions))
	for bucket := range regions {
		buckets = append(buckets, bucket)
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i] < buckets[j] })
	return buckets
}
