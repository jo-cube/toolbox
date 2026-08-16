package kshape

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"math"

	"github.com/jo-cube/toolbox/internal/hll"
	"github.com/jo-cube/toolbox/internal/prob"
)

const (
	Magic              = "KSHP"
	Version      uint8 = 2
	ChecksumName       = "crc32c"

	maxPartitions    = 1_000_000
	maxRegions       = 1_000_000
	maxCoverageSpans = 10_000_000
	maxSketchBytes   = 1 << 30
)

var checksumTable = crc32.MakeTable(crc32.Castagnoli)

func Write(w io.Writer, s *Summary) error {
	if err := validateSummary(s); err != nil {
		return err
	}
	checksum := crc32.New(checksumTable)
	body := io.MultiWriter(w, checksum)
	if _, err := io.WriteString(body, Magic); err != nil {
		return err
	}
	for _, field := range []any{Version, s.BucketWidth, s.Precision, hll.Version} {
		if err := binary.Write(body, binary.BigEndian, field); err != nil {
			return err
		}
	}
	if err := writeString(body, prob.HashName); err != nil {
		return err
	}
	if err := binary.Write(body, binary.BigEndian, uint16(len(s.Topic))); err != nil {
		return err
	}
	if _, err := io.WriteString(body, s.Topic); err != nil {
		return err
	}
	if err := binary.Write(body, binary.BigEndian, uint32(len(s.Partitions))); err != nil {
		return err
	}
	for _, partitionID := range sortedPartitions(s.Partitions) {
		partition := s.Partitions[partitionID]
		if err := binary.Write(body, binary.BigEndian, partitionID); err != nil {
			return err
		}
		if err := binary.Write(body, binary.BigEndian, uint32(len(partition.Regions))); err != nil {
			return err
		}
		for _, bucket := range sortedRegions(partition.Regions) {
			region := partition.Regions[bucket]
			fields := []any{
				bucket,
				region.Records,
				region.PayloadBytes,
				region.Tombstones,
				region.NullKeys,
				region.MissingTimestamps,
				region.MinTimestamp,
				region.MaxTimestamp,
			}
			for _, field := range fields {
				if err := binary.Write(body, binary.BigEndian, field); err != nil {
					return err
				}
			}
			if err := binary.Write(body, binary.BigEndian, uint32(len(region.Coverage))); err != nil {
				return err
			}
			for _, span := range region.Coverage {
				if err := binary.Write(body, binary.BigEndian, span); err != nil {
					return err
				}
			}
			if err := hll.Write(body, region.Keys); err != nil {
				return err
			}
		}
	}
	return binary.Write(w, binary.BigEndian, checksum.Sum32())
}

func Read(r io.Reader) (*Summary, error) {
	checksum := crc32.New(checksumTable)
	body := io.TeeReader(r, checksum)
	var magic [4]byte
	if _, err := io.ReadFull(body, magic[:]); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	if string(magic[:]) != Magic {
		if string(magic[:]) == "KSH1" {
			return nil, fmt.Errorf("unsupported kshape version 1")
		}
		return nil, fmt.Errorf("invalid kshape file magic %q", string(magic[:]))
	}
	var version uint8
	if err := binary.Read(body, binary.BigEndian, &version); err != nil {
		return nil, fmt.Errorf("read version: %w", err)
	}
	if version != Version {
		return nil, fmt.Errorf("unsupported kshape version %d", version)
	}
	var bucketWidth uint64
	var precision uint8
	var hllVersion uint8
	var topicLength uint16
	for _, field := range []struct {
		name  string
		value any
	}{
		{"bucket width", &bucketWidth},
		{"HLL precision", &precision},
		{"HLL version", &hllVersion},
	} {
		if err := binary.Read(body, binary.BigEndian, field.value); err != nil {
			return nil, fmt.Errorf("read %s: %w", field.name, err)
		}
	}
	if err := validateConfig(bucketWidth, precision); err != nil {
		return nil, err
	}
	if hllVersion != hll.Version {
		return nil, fmt.Errorf("unsupported HLL version %d", hllVersion)
	}
	hashName, err := readString(body)
	if err != nil {
		return nil, err
	}
	if hashName != prob.HashName {
		return nil, fmt.Errorf("unsupported hash %q", hashName)
	}
	if err := binary.Read(body, binary.BigEndian, &topicLength); err != nil {
		return nil, fmt.Errorf("read topic length: %w", err)
	}
	if topicLength > 249 {
		return nil, fmt.Errorf("invalid topic length %d", topicLength)
	}
	topicBytes := make([]byte, topicLength)
	if _, err := io.ReadFull(body, topicBytes); err != nil {
		return nil, fmt.Errorf("read topic: %w", err)
	}
	if len(topicBytes) != 0 {
		if err := validateTopic(string(topicBytes)); err != nil {
			return nil, err
		}
	}
	s, err := New(bucketWidth, precision)
	if err != nil {
		return nil, err
	}
	s.Topic = string(topicBytes)

	var partitionCount uint32
	if err := binary.Read(body, binary.BigEndian, &partitionCount); err != nil {
		return nil, fmt.Errorf("read partition count: %w", err)
	}
	if partitionCount > maxPartitions {
		return nil, fmt.Errorf("partition count %d exceeds limit %d", partitionCount, maxPartitions)
	}
	var previousPartition int32 = -1
	var totalRegions, totalCoverageSpans uint64
	for i := uint32(0); i < partitionCount; i++ {
		var partitionID int32
		var regionCount uint32
		if err := binary.Read(body, binary.BigEndian, &partitionID); err != nil {
			return nil, fmt.Errorf("read partition %d: %w", i, err)
		}
		if partitionID < 0 || partitionID <= previousPartition {
			return nil, fmt.Errorf("invalid or unsorted partition %d", partitionID)
		}
		previousPartition = partitionID
		if err := binary.Read(body, binary.BigEndian, &regionCount); err != nil {
			return nil, fmt.Errorf("read partition %d region count: %w", partitionID, err)
		}
		if regionCount == 0 || uint64(regionCount) > maxRegions-totalRegions {
			return nil, fmt.Errorf("invalid region count %d for partition %d", regionCount, partitionID)
		}
		totalRegions += uint64(regionCount)
		if totalRegions*(uint64(1)<<precision) > maxSketchBytes {
			return nil, fmt.Errorf("HLL register data exceeds %d-byte limit", maxSketchBytes)
		}
		partition := &Partition{Regions: make(map[uint64]*Region)}
		var previousBucket uint64
		for j := uint32(0); j < regionCount; j++ {
			region := &Region{}
			fields := []any{
				&region.Bucket,
				&region.Records,
				&region.PayloadBytes,
				&region.Tombstones,
				&region.NullKeys,
				&region.MissingTimestamps,
				&region.MinTimestamp,
				&region.MaxTimestamp,
			}
			for _, field := range fields {
				if err := binary.Read(body, binary.BigEndian, field); err != nil {
					return nil, fmt.Errorf("read partition %d region %d: %w", partitionID, j, err)
				}
			}
			if j != 0 && region.Bucket <= previousBucket {
				return nil, fmt.Errorf("unsorted or duplicate bucket %d in partition %d", region.Bucket, partitionID)
			}
			previousBucket = region.Bucket
			var coverageCount uint32
			if err := binary.Read(body, binary.BigEndian, &coverageCount); err != nil {
				return nil, fmt.Errorf("read partition %d bucket %d coverage count: %w", partitionID, region.Bucket, err)
			}
			if coverageCount == 0 || uint64(coverageCount) > maxCoverageSpans-totalCoverageSpans {
				return nil, fmt.Errorf("invalid coverage count %d for partition %d bucket %d", coverageCount, partitionID, region.Bucket)
			}
			totalCoverageSpans += uint64(coverageCount)
			region.Coverage = make([]OffsetSpan, coverageCount)
			if err := binary.Read(body, binary.BigEndian, region.Coverage); err != nil {
				return nil, fmt.Errorf("read partition %d bucket %d coverage: %w", partitionID, region.Bucket, err)
			}
			region.FirstOffset = region.Coverage[0].FirstOffset
			region.LastOffset = region.Coverage[len(region.Coverage)-1].LastOffset
			region.Keys, err = hll.Read(body)
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
		partition.lastOffset = partition.Regions[previousBucket].LastOffset
		partition.hasOffset = true
		s.Partitions[partitionID] = partition
	}
	if s.Topic == "" && len(s.Partitions) != 0 {
		return nil, fmt.Errorf("non-empty artifact has empty topic")
	}
	var storedChecksum uint32
	if err := binary.Read(r, binary.BigEndian, &storedChecksum); err != nil {
		return nil, fmt.Errorf("read checksum: %w", err)
	}
	if actual := checksum.Sum32(); actual != storedChecksum {
		return nil, fmt.Errorf("checksum mismatch: got %08x, want %08x", storedChecksum, actual)
	}
	var trailing [1]byte
	if n, err := r.Read(trailing[:]); n != 0 || err != io.EOF {
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
	if s.Topic == "" {
		if len(s.Partitions) != 0 {
			return fmt.Errorf("non-empty summary has empty topic")
		}
	} else if err := validateTopic(s.Topic); err != nil {
		return err
	}
	if len(s.Partitions) > maxPartitions {
		return fmt.Errorf("too many partitions")
	}
	var totalRegions, totalCoverageSpans uint64
	for partitionID, partition := range s.Partitions {
		if partitionID < 0 || partition == nil || len(partition.Regions) == 0 {
			return fmt.Errorf("invalid partition %d", partitionID)
		}
		totalRegions += uint64(len(partition.Regions))
		if totalRegions > maxRegions || totalRegions*(uint64(1)<<s.Precision) > maxSketchBytes {
			return fmt.Errorf("summary exceeds artifact size limits")
		}
		for bucket, region := range partition.Regions {
			if region == nil || region.Bucket != bucket || region.Keys == nil || region.Keys.Precision != s.Precision || len(region.Keys.Registers) != 1<<s.Precision {
				return fmt.Errorf("invalid partition %d bucket %d", partitionID, bucket)
			}
			totalCoverageSpans += uint64(len(region.Coverage))
			if totalCoverageSpans > maxCoverageSpans {
				return fmt.Errorf("summary has too many coverage spans")
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
	if len(region.Coverage) == 0 || region.FirstOffset != region.Coverage[0].FirstOffset ||
		region.LastOffset != region.Coverage[len(region.Coverage)-1].LastOffset {
		return fmt.Errorf("invalid observed offset coverage")
	}
	var covered uint64
	for i, span := range region.Coverage {
		if span.FirstOffset < 0 || span.FirstOffset > span.LastOffset ||
			uint64(span.FirstOffset) < start || uint64(span.LastOffset) > end ||
			(i != 0 && (span.FirstOffset <= region.Coverage[i-1].LastOffset || span.FirstOffset == region.Coverage[i-1].LastOffset+1)) {
			return fmt.Errorf("invalid observed offset coverage")
		}
		covered += uint64(span.LastOffset-span.FirstOffset) + 1
	}
	if region.FirstOffset < 0 || region.FirstOffset > region.LastOffset ||
		uint64(region.FirstOffset) < start || uint64(region.LastOffset) > end {
		return fmt.Errorf("observed offsets %d..%d are outside region %d..%d", region.FirstOffset, region.LastOffset, start, end)
	}
	if region.Records == 0 || region.Records > covered || region.Tombstones > region.Records || region.NullKeys > region.Records || region.MissingTimestamps > region.Records {
		return fmt.Errorf("invalid counters")
	}
	if region.MissingTimestamps == region.Records {
		if region.MinTimestamp != 0 || region.MaxTimestamp != 0 {
			return fmt.Errorf("timestamp bounds present when all timestamps are missing")
		}
	} else if region.MinTimestamp < 0 || region.MinTimestamp > region.MaxTimestamp {
		return fmt.Errorf("invalid timestamp bounds")
	}
	keyedRecords, nonzeroRegisters := region.Records-region.NullKeys, uint64(0)
	for _, rank := range region.Keys.Registers {
		if rank > 65-region.Keys.Precision {
			return fmt.Errorf("invalid key sketch register")
		}
		if rank != 0 {
			nonzeroRegisters++
		}
	}
	if nonzeroRegisters > keyedRecords || keyedRecords != 0 && nonzeroRegisters == 0 {
		return fmt.Errorf("invalid key sketch for %d keyed records", keyedRecords)
	}
	return nil
}

func writeString(w io.Writer, value string) error {
	if len(value) > math.MaxUint8 {
		return fmt.Errorf("string too long")
	}
	if err := binary.Write(w, binary.BigEndian, uint8(len(value))); err != nil {
		return err
	}
	_, err := io.WriteString(w, value)
	return err
}

func readString(r io.Reader) (string, error) {
	var size uint8
	if err := binary.Read(r, binary.BigEndian, &size); err != nil {
		return "", fmt.Errorf("read string length: %w", err)
	}
	value := make([]byte, size)
	if _, err := io.ReadFull(r, value); err != nil {
		return "", fmt.Errorf("read string: %w", err)
	}
	return string(value), nil
}
