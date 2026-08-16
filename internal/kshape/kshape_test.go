package kshape

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/jo-cube/toolbox/internal/hll"
	"github.com/jo-cube/toolbox/internal/prob"
)

func TestBuildProfilesBinaryKeysInterleavedPartitionsAndSparseOffsets(t *testing.T) {
	t.Parallel()

	binaryKey := []byte{0, '\t', '\n', 0xff}
	input := joinFrames(
		frame("events", 0, 0, 100, 10, binaryKey, false),
		frame("events", 1, 2, 200, 0, nil, true),
		frame("events", 0, 3, -1, -1, binaryKey, false),
		frame("events", 0, 4, 50, 5, nil, false),
		frame("events", 0, 9, 150, 7, []byte("new"), false),
	)
	summary, err := Build(bytes.NewReader(input), 4, 8)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Topic != "events" || len(summary.Partitions) != 2 {
		t.Fatalf("summary = %#v", summary)
	}

	report, err := summary.Report(8)
	if err != nil {
		t.Fatal(err)
	}
	p0 := report.Partitions[0]
	if p0.Partition != 0 || len(p0.Regions) != 2 {
		t.Fatalf("partition 0 report = %#v", p0)
	}
	first := p0.Regions[0]
	if first.RegionFirstOffset != 0 || first.RegionLastOffset != 7 ||
		first.ObservedRecords != 3 || first.LogicalPayloadBytes != 15 ||
		first.ObservedTombstones != 1 || first.KeyedRecords != 3 ||
		first.MissingTimestamps != 1 || first.ApproxDistinctKeys != 2 ||
		first.ApproxRecordsPerKey != 1.5 || first.ObservedOccupancy != 0.6 {
		t.Fatalf("first region = %#v", first)
	}
	if got := report.Partitions[1].Regions[0]; got.NullKeys != 1 || got.ApproxDistinctKeys != 0 {
		t.Fatalf("partition 1 region = %#v", got)
	}

	whole, err := summary.Report(0)
	if err != nil {
		t.Fatal(err)
	}
	got := whole.Partitions[0].Regions[0]
	if got.ObservedRecords != 4 || got.LogicalPayloadBytes != 22 || got.ApproxDistinctKeys != 3 {
		t.Fatalf("whole partition = %#v", got)
	}
	if got.MinTimestamp == nil || *got.MinTimestamp != 50 || *got.MaxTimestamp != 150 {
		t.Fatalf("timestamp bounds = %v..%v", got.MinTimestamp, got.MaxTimestamp)
	}
}

func TestBuildStreamsLargeBinaryKey(t *testing.T) {
	t.Parallel()

	key := bytes.Repeat([]byte{0, '\t', '\n', 0xff}, 20<<10)
	summary, err := Build(bytes.NewReader(frame("events", 0, 0, 1, 1, key, false)), 4, 8)
	if err != nil {
		t.Fatal(err)
	}
	report, err := summary.Report(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := report.Partitions[0].Regions[0]; got.ObservedRecords != 1 || got.ApproxDistinctKeys != 1 {
		t.Fatalf("region = %#v", got)
	}
}

func TestBuildRejectsMalformedAndTruncatedFrames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"truncated key", []byte("events\t0\t1\t2\t3\t4\tab"), "read 4-byte key"},
		{"bad terminator", []byte("events\t0\t1\t2\t3\t1\taX"), "invalid record terminator"},
		{"negative offset", frame("events", 0, -1, 2, 3, nil, true), "offset must be non-negative"},
		{"invalid timestamp", frame("events", 0, 1, -2, 3, nil, true), "timestamp must be -1 or non-negative"},
		{"bad payload length", []byte("events\t0\t1\t2\t-2\t-1\n"), "payload length must be -1"},
		{"bad key length", []byte("events\t0\t1\t2\t3\t-2\t\n"), "key length must be -1 through"},
		{"excessive key length", []byte("events\t0\t1\t2\t3\t2147483648\t\n"), "key length must be -1 through"},
		{"overflow", []byte("events\t0\t9223372036854775808\t2\t3\t-1\n"), "integer overflow"},
		{"invalid topic", frame("bad topic", 0, 1, 2, 3, nil, true), "invalid Kafka topic"},
		{"topic change", joinFrames(frame("a", 0, 1, 2, 3, nil, true), frame("b", 0, 2, 3, 4, nil, true)), "differs from"},
		{"duplicate offset", joinFrames(frame("events", 0, 1, 2, 3, nil, true), frame("events", 0, 1, 3, 4, nil, true)), "not greater than previous"},
		{"out-of-order offset", joinFrames(frame("events", 0, 2, 2, 3, nil, true), frame("events", 0, 1, 3, 4, nil, true)), "not greater than previous"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Build(bytes.NewReader(tt.input), 4, 8)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Build() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestReportAggregatesPowerOfTwoResolutions(t *testing.T) {
	t.Parallel()

	summary, err := Build(bytes.NewReader(joinFrames(
		frame("events", 0, 0, 1, 1, []byte("a"), false),
		frame("events", 0, 4, 2, 1, []byte("b"), false),
		frame("events", 0, 8, 3, 1, []byte("c"), false),
	)), 4, 8)
	if err != nil {
		t.Fatal(err)
	}
	report, err := summary.Report(8)
	if err != nil {
		t.Fatal(err)
	}
	if got := report.Partitions[0].Regions; len(got) != 2 || got[0].ObservedRecords != 2 || got[1].ObservedRecords != 1 {
		t.Fatalf("regions = %#v", got)
	}
	for _, width := range []uint64{2, 12} {
		if _, err := summary.Report(width); err == nil {
			t.Fatalf("Report(%d) succeeded", width)
		}
	}
}

func TestArtifactIsDeterministicAndRejectsCorruption(t *testing.T) {
	t.Parallel()

	a, err := Build(bytes.NewReader(joinFrames(
		frame("events", 1, 5, 50, 5, []byte("b"), false),
		frame("events", 0, 1, 10, 3, []byte("a"), false),
	)), 4, 8)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(bytes.NewReader(joinFrames(
		frame("events", 0, 1, 10, 3, []byte("a"), false),
		frame("events", 1, 5, 50, 5, []byte("b"), false),
	)), 4, 8)
	if err != nil {
		t.Fatal(err)
	}
	dataA := writeArtifact(t, a)
	dataB := writeArtifact(t, b)
	if !bytes.Equal(dataA, dataB) {
		t.Fatal("serialization depends on input order")
	}
	roundTrip, err := Read(bytes.NewReader(dataA))
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip.Topic != "events" || len(roundTrip.Partitions) != 2 {
		t.Fatalf("round trip = %#v", roundTrip)
	}

	badMagic := append([]byte(nil), dataA...)
	copy(badMagic, "NOPE")
	badVersion := append([]byte(nil), dataA...)
	badVersion[4] = 99
	badHash := append([]byte(nil), dataA...)
	badHash[bytes.Index(badHash, []byte(prob.HashName))] ^= 1
	badChecksum := append([]byte(nil), dataA...)
	badChecksum[len(badChecksum)-5] ^= 1
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"bad magic", badMagic, "invalid kshape file magic"},
		{"bad version", badVersion, "unsupported kshape version 99"},
		{"bad hash", badHash, "unsupported hash"},
		{"truncated", dataA[:len(dataA)-1], "unexpected EOF"},
		{"trailing", append(append([]byte(nil), dataA...), 0), "unexpected trailing data"},
		{"checksum", badChecksum, "checksum mismatch"},
	}
	for _, tt := range tests {
		if _, err := Read(bytes.NewReader(tt.data)); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Read(%s) error = %v, want %q", tt.name, err, tt.want)
		}
	}

	invalid := mustBuild(t, "events", 4, 8, 1)
	invalid.Partitions[0].Regions[0].Records = 2
	if err := Write(&bytes.Buffer{}, invalid); err == nil || !strings.Contains(err.Error(), "invalid counters") {
		t.Fatalf("Write(invalid) error = %v", err)
	}
}

func TestArtifactRejectsExcessiveDeclaredSketchDataBeforeAllocation(t *testing.T) {
	t.Parallel()

	var artifact bytes.Buffer
	artifact.WriteString(Magic)
	for _, field := range []any{Version, uint64(1), uint8(20), hll.Version} {
		if err := binary.Write(&artifact, binary.BigEndian, field); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeString(&artifact, prob.HashName); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&artifact, binary.BigEndian, uint16(len("events"))); err != nil {
		t.Fatal(err)
	}
	artifact.WriteString("events")
	for _, field := range []any{uint32(1), int32(0), uint32(maxSketchBytes/(1<<20) + 1)} {
		if err := binary.Write(&artifact, binary.BigEndian, field); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Read(bytes.NewReader(artifact.Bytes())); err == nil || !strings.Contains(err.Error(), "HLL register data exceeds") {
		t.Fatalf("Read() error = %v", err)
	}
}

func TestMergeCombinesDisjointScansAndRejectsInvalidMerges(t *testing.T) {
	t.Parallel()

	a := mustBuild(t, "events", 8, 8, 0, 1)
	b := mustBuild(t, "events", 8, 8, 2, 3)
	if err := a.Merge(b); err != nil {
		t.Fatal(err)
	}
	report, err := a.Report(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := report.Partitions[0].Regions[0].ObservedRecords; got != 4 {
		t.Fatalf("observed records = %d, want 4", got)
	}

	fragments := mustBuild(t, "events", 8, 8, 0, 1)
	if err := fragments.Merge(mustBuild(t, "events", 8, 8, 4, 5)); err != nil {
		t.Fatal(err)
	}
	fragments, err = Read(bytes.NewReader(writeArtifact(t, fragments)))
	if err != nil {
		t.Fatal(err)
	}
	if err := fragments.Merge(mustBuild(t, "events", 8, 8, 2, 3)); err != nil {
		t.Fatalf("merge of disjoint middle fragment depends on input order: %v", err)
	}
	if got := fragments.Partitions[0].Regions[0].Coverage; len(got) != 1 || got[0] != (OffsetSpan{0, 5}) {
		t.Fatalf("merged coverage = %#v", got)
	}
	otherOrder := mustBuild(t, "events", 8, 8, 2, 3)
	if err := otherOrder.Merge(mustBuild(t, "events", 8, 8, 4, 5)); err != nil {
		t.Fatal(err)
	}
	if err := otherOrder.Merge(mustBuild(t, "events", 8, 8, 0, 1)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(writeArtifact(t, fragments), writeArtifact(t, otherOrder)) {
		t.Fatal("artifact serialization depends on merge order")
	}

	overlapA := mustBuild(t, "events", 8, 8, 0, 3)
	overlapB := mustBuild(t, "events", 8, 8, 1, 2)
	if err := overlapA.Merge(overlapB); err == nil || !strings.Contains(err.Error(), "overlapping") {
		t.Fatalf("overlap error = %v", err)
	}
	identical := mustBuild(t, "events", 8, 8, 0, 3)
	if err := identical.Merge(mustBuild(t, "events", 8, 8, 0, 3)); err == nil || !strings.Contains(err.Error(), "overlapping") {
		t.Fatalf("identical artifact error = %v", err)
	}
	for name, other := range map[string]*Summary{
		"topic":     mustBuild(t, "other", 8, 8, 4),
		"width":     mustBuild(t, "events", 4, 8, 4),
		"precision": mustBuild(t, "events", 8, 9, 4),
	} {
		base := mustBuild(t, "events", 8, 8, 0)
		if err := base.Merge(other); err == nil {
			t.Errorf("%s mismatch merged", name)
		}
	}
}

func TestEmptyAndMaximumOffsetArtifacts(t *testing.T) {
	t.Parallel()

	empty, err := Build(bytes.NewReader(nil), DefaultBucketWidth, DefaultPrecision)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Read(bytes.NewReader(writeArtifact(t, empty)))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Topic != "" || len(decoded.Partitions) != 0 {
		t.Fatalf("empty artifact = %#v", decoded)
	}

	maximum, err := Build(bytes.NewReader(frame("events", math.MaxInt32, math.MaxInt64, -1, 0, nil, false)), 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Read(bytes.NewReader(writeArtifact(t, maximum))); err != nil {
		t.Fatal(err)
	}
	maximumWidth, err := Build(bytes.NewReader(frame("events", 0, math.MaxInt64, -1, 0, nil, false)), 1<<63, 8)
	if err != nil {
		t.Fatal(err)
	}
	maximumReport, err := maximumWidth.Report(1 << 63)
	if err != nil || maximumReport.Partitions[0].Regions[0].RegionLastOffset != math.MaxInt64 {
		t.Fatalf("maximum-width report = %#v, %v", maximumReport, err)
	}
}

func TestAddRejectsPayloadCounterOverflow(t *testing.T) {
	t.Parallel()

	summary, err := New(8, 8)
	if err != nil {
		t.Fatal(err)
	}
	record := Record{Topic: "events", Offset: 0, PayloadLength: math.MaxInt64, NullKey: true}
	if err := summary.Add(record); err != nil {
		t.Fatal(err)
	}
	record.Offset = 1
	if err := summary.Add(record); err != nil {
		t.Fatal(err)
	}
	record.Offset = 2
	record.PayloadLength = 2
	if err := summary.Add(record); err == nil || !strings.Contains(err.Error(), "overflow") {
		t.Fatalf("Add() error = %v, want overflow", err)
	}
}

func TestNewRequiresExplicitValidConfiguration(t *testing.T) {
	t.Parallel()

	for _, config := range [][2]uint64{{0, 8}, {8, 0}, {3, 8}} {
		if _, err := New(config[0], uint8(config[1])); err == nil {
			t.Fatalf("New(%d, %d) succeeded", config[0], config[1])
		}
	}
}

func frame(topic string, partition int32, offset, timestamp, payloadLength int64, key []byte, nullKey bool) []byte {
	var out bytes.Buffer
	keyLength := len(key)
	if nullKey {
		keyLength = -1
	}
	fmt.Fprintf(&out, "%s\t%d\t%d\t%d\t%d\t%d\t", topic, partition, offset, timestamp, payloadLength, keyLength)
	if !nullKey {
		out.Write(key)
	}
	out.WriteByte('\n')
	return out.Bytes()
}

func joinFrames(frames ...[]byte) []byte {
	return bytes.Join(frames, nil)
}

func mustBuild(t *testing.T, topic string, width uint64, precision uint8, offsets ...int64) *Summary {
	t.Helper()
	frames := make([][]byte, 0, len(offsets))
	for _, offset := range offsets {
		frames = append(frames, frame(topic, 0, offset, offset, 1, []byte(fmt.Sprint(offset)), false))
	}
	summary, err := Build(bytes.NewReader(joinFrames(frames...)), width, precision)
	if err != nil {
		t.Fatal(err)
	}
	return summary
}

func writeArtifact(t *testing.T, summary *Summary) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := Write(&out, summary); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
