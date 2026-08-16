package kshape

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/jo-cube/toolbox/internal/hll"
)

const (
	Magic         = "KSH1"
	Version uint8 = 1

	maxPartitions = 1_000_000
	maxRegions    = 10_000_000
)

func Write(w io.Writer, s *Summary) error {
	if err := validateSummary(s); err != nil {
		return err
	}
	if _, err := io.WriteString(w, Magic); err != nil {
		return err
	}
	for _, field := range []any{Version, s.BucketWidth, s.Precision, uint16(len(s.Topic))} {
		if err := binary.Write(w, binary.BigEndian, field); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, s.Topic); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint32(len(s.Partitions))); err != nil {
		return err
	}
	for _, partitionID := range sortedPartitions(s.Partitions) {
		partition := s.Partitions[partitionID]
		if err := binary.Write(w, binary.BigEndian, partitionID); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, uint32(len(partition.Regions))); err != nil {
			return err
		}
		for _, bucket := range sortedRegions(partition.Regions) {
			region := partition.Regions[bucket]
			fields := []any{
				bucket,
				region.FirstOffset,
				region.LastOffset,
				region.Records,
				region.PayloadBytes,
				region.Tombstones,
				region.NullKeys,
				region.MissingTimestamps,
				region.MinTimestamp,
				region.MaxTimestamp,
			}
			for _, field := range fields {
				if err := binary.Write(w, binary.BigEndian, field); err != nil {
					return err
				}
			}
			if err := hll.Write(w, region.Keys); err != nil {
				return err
			}
		}
	}
	return nil
}

func Read(r io.Reader) (*Summary, error) {
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	if string(magic[:]) != Magic {
		return nil, fmt.Errorf("invalid kshape file magic %q", string(magic[:]))
	}
	var version uint8
	if err := binary.Read(r, binary.BigEndian, &version); err != nil {
		return nil, fmt.Errorf("read version: %w", err)
	}
	if version != Version {
		return nil, fmt.Errorf("unsupported kshape version %d", version)
	}
	var bucketWidth uint64
	var precision uint8
	var topicLength uint16
	for _, field := range []struct {
		name  string
		value any
	}{
		{"bucket width", &bucketWidth},
		{"HLL precision", &precision},
		{"topic length", &topicLength},
	} {
		if err := binary.Read(r, binary.BigEndian, field.value); err != nil {
			return nil, fmt.Errorf("read %s: %w", field.name, err)
		}
	}
	if err := validateConfig(bucketWidth, precision); err != nil {
		return nil, err
	}
	if topicLength > 249 {
		return nil, fmt.Errorf("invalid topic length %d", topicLength)
	}
	topicBytes := make([]byte, topicLength)
	if _, err := io.ReadFull(r, topicBytes); err != nil {
		return nil, fmt.Errorf("read topic: %w", err)
	}
	s, err := New(bucketWidth, precision)
	if err != nil {
		return nil, err
	}
	s.Topic = string(topicBytes)

	var partitionCount uint32
	if err := binary.Read(r, binary.BigEndian, &partitionCount); err != nil {
		return nil, fmt.Errorf("read partition count: %w", err)
	}
	if partitionCount > maxPartitions {
		return nil, fmt.Errorf("partition count %d exceeds limit %d", partitionCount, maxPartitions)
	}
	var previousPartition int32 = -1
	for i := uint32(0); i < partitionCount; i++ {
		var partitionID int32
		var regionCount uint32
		if err := binary.Read(r, binary.BigEndian, &partitionID); err != nil {
			return nil, fmt.Errorf("read partition %d: %w", i, err)
		}
		if partitionID < 0 || partitionID <= previousPartition {
			return nil, fmt.Errorf("invalid or unsorted partition %d", partitionID)
		}
		previousPartition = partitionID
		if err := binary.Read(r, binary.BigEndian, &regionCount); err != nil {
			return nil, fmt.Errorf("read partition %d region count: %w", partitionID, err)
		}
		if regionCount == 0 || regionCount > maxRegions {
			return nil, fmt.Errorf("invalid region count %d for partition %d", regionCount, partitionID)
		}
		partition := &Partition{Regions: make(map[uint64]*Region)}
		var previousBucket uint64
		for j := uint32(0); j < regionCount; j++ {
			region := &Region{}
			fields := []any{
				&region.Bucket,
				&region.FirstOffset,
				&region.LastOffset,
				&region.Records,
				&region.PayloadBytes,
				&region.Tombstones,
				&region.NullKeys,
				&region.MissingTimestamps,
				&region.MinTimestamp,
				&region.MaxTimestamp,
			}
			for _, field := range fields {
				if err := binary.Read(r, binary.BigEndian, field); err != nil {
					return nil, fmt.Errorf("read partition %d region %d: %w", partitionID, j, err)
				}
			}
			if j != 0 && region.Bucket <= previousBucket {
				return nil, fmt.Errorf("unsorted or duplicate bucket %d in partition %d", region.Bucket, partitionID)
			}
			previousBucket = region.Bucket
			region.Keys, err = hll.Read(r)
			if err != nil {
				return nil, fmt.Errorf("read partition %d bucket %d keys: %w", partitionID, region.Bucket, err)
			}
			if region.Keys.Precision != precision {
				return nil, fmt.Errorf("partition %d bucket %d has HLL precision %d, want %d", partitionID, region.Bucket, region.Keys.Precision, precision)
			}
			if err := validateRegion(bucketWidth, region); err != nil {
				return nil, fmt.Errorf("partition %d bucket %d: %w", partitionID, region.Bucket, err)
			}
			partition.Regions[region.Bucket] = region
		}
		s.Partitions[partitionID] = partition
	}
	if s.Topic == "" && len(s.Partitions) != 0 {
		return nil, fmt.Errorf("non-empty artifact has empty topic")
	}
	var trailing [1]byte
	if n, err := r.Read(trailing[:]); n != 0 || (err != nil && err != io.EOF) {
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("read trailing data: %w", err)
		}
		return nil, fmt.Errorf("unexpected trailing data")
	}
	return s, nil
}

func validateSummary(s *Summary) error {
	if s == nil {
		return fmt.Errorf("summary is nil")
	}
	if err := validateConfig(s.BucketWidth, s.Precision); err != nil {
		return err
	}
	if len(s.Topic) > 249 || (s.Topic == "" && len(s.Partitions) != 0) {
		return fmt.Errorf("invalid topic")
	}
	if len(s.Partitions) > maxPartitions {
		return fmt.Errorf("too many partitions")
	}
	for partitionID, partition := range s.Partitions {
		if partitionID < 0 || partition == nil || len(partition.Regions) == 0 || len(partition.Regions) > maxRegions {
			return fmt.Errorf("invalid partition %d", partitionID)
		}
		for bucket, region := range partition.Regions {
			if region == nil || region.Bucket != bucket || region.Keys == nil || region.Keys.Precision != s.Precision {
				return fmt.Errorf("invalid partition %d bucket %d", partitionID, bucket)
			}
			if err := validateRegion(s.BucketWidth, region); err != nil {
				return fmt.Errorf("partition %d bucket %d: %w", partitionID, bucket, err)
			}
		}
	}
	return nil
}

func validateRegion(bucketWidth uint64, region *Region) error {
	if region.Bucket > uint64(math.MaxInt64)/bucketWidth {
		return fmt.Errorf("bucket is outside Kafka offset space")
	}
	start := region.Bucket * bucketWidth
	end := min(start+bucketWidth-1, uint64(math.MaxInt64))
	if region.FirstOffset < 0 || region.FirstOffset > region.LastOffset ||
		uint64(region.FirstOffset) < start || uint64(region.LastOffset) > end {
		return fmt.Errorf("observed offsets %d..%d are outside region %d..%d", region.FirstOffset, region.LastOffset, start, end)
	}
	if region.Records == 0 || region.Tombstones > region.Records || region.NullKeys > region.Records || region.MissingTimestamps > region.Records {
		return fmt.Errorf("invalid counters")
	}
	if region.MissingTimestamps == region.Records {
		if region.MinTimestamp != 0 || region.MaxTimestamp != 0 {
			return fmt.Errorf("timestamp bounds present when all timestamps are missing")
		}
	} else if region.MinTimestamp > region.MaxTimestamp || region.MinTimestamp == -1 || region.MaxTimestamp == -1 {
		return fmt.Errorf("invalid timestamp bounds")
	}
	return nil
}
