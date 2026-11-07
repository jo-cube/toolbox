package ksetoff

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
)

func TestParseOffsetSpec(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		raw     string
		want    OffsetSpec
		wantErr string
	}{
		{name: "numeric", raw: "42", want: OffsetSpec{Mode: OffsetModeNumeric, Numeric: 42}},
		{name: "earliest", raw: "earliest", want: OffsetSpec{Mode: OffsetModeEarliest}},
		{name: "beginning alias", raw: "beginning", want: OffsetSpec{Mode: OffsetModeEarliest}},
		{name: "latest", raw: "latest", want: OffsetSpec{Mode: OffsetModeLatest}},
		{name: "end alias", raw: "end", want: OffsetSpec{Mode: OffsetModeLatest}},
		{name: "timestamp", raw: "timestamp:2026-04-13T00:00:00Z", want: OffsetSpec{Mode: OffsetModeTimestamp, Timestamp: timestamp}},
		{name: "timestamp without timezone", raw: "timestamp:2026-04-13T00:00:00", want: OffsetSpec{Mode: OffsetModeTimestamp, Timestamp: timestamp}},
		{name: "timestamp with space", raw: "timestamp:2026-04-13 00:00:00", want: OffsetSpec{Mode: OffsetModeTimestamp, Timestamp: timestamp}},
		{name: "timestamp date", raw: "timestamp:2026-04-13", want: OffsetSpec{Mode: OffsetModeTimestamp, Timestamp: timestamp}},
		{name: "negative", raw: "-1", wantErr: "offset must be non-negative"},
		{name: "invalid", raw: "tomorrow", wantErr: "invalid offset"},
		{name: "invalid timestamp", raw: "timestamp:nope", wantErr: "invalid timestamp"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseOffsetSpec(tt.raw)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParseOffsetSpec() error = %v, want substring %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseOffsetSpec() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseOffsetSpec() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseOffsetSpecTimestampMilliseconds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		raw        string
		wantMillis int64
		wantErr    bool
	}{
		{name: "before epoch", raw: "1969-12-31T23:59:59.999Z", wantErr: true},
		{name: "nanosecond before epoch", raw: "1969-12-31T23:59:59.999999999Z", wantErr: true},
		{name: "earliest year", raw: "0000-01-01", wantErr: true},
		{name: "epoch", raw: "1970-01-01T00:00:00Z"},
		{name: "nanosecond after epoch", raw: "1970-01-01T00:00:00.000000001Z"},
		{name: "millisecond after epoch", raw: "1970-01-01T00:00:00.001Z", wantMillis: 1},
		{name: "timezone at epoch", raw: "1970-01-01T05:30:00+05:30"},
		{name: "timezone before epoch", raw: "1970-01-01T00:00:00+00:01", wantErr: true},
		{name: "pre-epoch date at epoch", raw: "1969-12-31T23:00:00-01:00"},
		{name: "latest year", raw: "9999-12-31T23:59:59.999999999Z", wantMillis: 253402300799999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseOffsetSpec("timestamp:" + tt.raw)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "must be on or after the Unix epoch") {
					t.Fatalf("ParseOffsetSpec() error = %v, want pre-epoch timestamp error", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Mode != OffsetModeTimestamp || got.Timestamp.UnixMilli() != tt.wantMillis {
				t.Fatalf("ParseOffsetSpec() = %#v (%d ms), want timestamp at %d ms", got, got.Timestamp.UnixMilli(), tt.wantMillis)
			}
		})
	}
}

func TestParsePartitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    []int32
		wantErr string
	}{
		{name: "empty means all", raw: "", want: nil},
		{name: "single", raw: "2", want: []int32{2}},
		{name: "multiple sorted deduped", raw: "3,1,3,2", want: []int32{1, 2, 3}},
		{name: "blank entries rejected", raw: "1, ,2", wantErr: "empty entry"},
		{name: "only blank entries rejected", raw: ",", wantErr: "empty entry"},
		{name: "negative invalid", raw: "-1", wantErr: "invalid partition"},
		{name: "non numeric invalid", raw: "abc", wantErr: "invalid partition"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParsePartitions(tt.raw)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParsePartitions() error = %v, want substring %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParsePartitions() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParsePartitions() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestTargetPartitions(t *testing.T) {
	t.Parallel()

	got, err := targetPartitions(3, nil)
	if err != nil {
		t.Fatalf("targetPartitions() error = %v", err)
	}
	if !reflect.DeepEqual(got, []int32{0, 1, 2}) {
		t.Fatalf("targetPartitions() = %#v, want %#v", got, []int32{0, 1, 2})
	}

	_, err = targetPartitions(3, []int32{3})
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("targetPartitions() error = %v, want out of range", err)
	}
}

func TestWatermarksForPartitions(t *testing.T) {
	t.Parallel()

	startOffsets := kadm.ListedOffsets{"topic-a": map[int32]kadm.ListedOffset{0: {Offset: 5}, 1: {Offset: 10}}}
	endOffsets := kadm.ListedOffsets{"topic-a": map[int32]kadm.ListedOffset{0: {Offset: 20}, 1: {Offset: 30}}}

	got, err := watermarksForPartitions("topic-a", []int32{0, 1}, startOffsets, endOffsets)
	if err != nil {
		t.Fatalf("watermarksForPartitions() error = %v", err)
	}
	if got[0] != (watermark{low: 5, high: 20}) || got[1] != (watermark{low: 10, high: 30}) {
		t.Fatalf("watermarksForPartitions() = %#v", got)
	}

	_, err = watermarksForPartitions("topic-a", []int32{2}, startOffsets, endOffsets)
	if err == nil || !strings.Contains(err.Error(), "no start offset returned") {
		t.Fatalf("watermarksForPartitions() error = %v, want missing start offset", err)
	}

	brokenStart := kadm.ListedOffsets{"topic-a": map[int32]kadm.ListedOffset{0: {Err: errors.New("offline")}}}
	_, err = watermarksForPartitions("topic-a", []int32{0}, brokenStart, endOffsets)
	if err == nil || !strings.Contains(err.Error(), "start offset") {
		t.Fatalf("watermarksForPartitions() error = %v, want start offset error", err)
	}
}

func TestTimestampOffsetOrHighWatermark(t *testing.T) {
	t.Parallel()

	wm := watermark{low: 5, high: 20}
	if got := timestampOffsetOrHighWatermark(12, wm); got != 12 {
		t.Fatalf("timestampOffsetOrHighWatermark() = %d, want 12", got)
	}
	if got := timestampOffsetOrHighWatermark(-1, wm); got != 20 {
		t.Fatalf("timestampOffsetOrHighWatermark() = %d, want high watermark 20", got)
	}
}

func TestIsActiveGroupError(t *testing.T) {
	t.Parallel()

	if !isActiveGroupError(kerr.IllegalGeneration) {
		t.Fatal("IllegalGeneration was not recognized as an active group error")
	}
	if !isActiveGroupError(kerr.NonEmptyGroup) {
		t.Fatal("NonEmptyGroup was not recognized as an active group error")
	}
	if isActiveGroupError(errors.New("unrelated")) {
		t.Fatal("unrelated error was recognized as an active group error")
	}
}
