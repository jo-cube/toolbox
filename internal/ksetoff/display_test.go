package ksetoff

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestDescribeOffsetSpec(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		spec OffsetSpec
		want string
	}{
		{name: "numeric", spec: OffsetSpec{Mode: OffsetModeNumeric, Numeric: 42}, want: "42 (numeric)"},
		{name: "earliest", spec: OffsetSpec{Mode: OffsetModeEarliest}, want: "earliest (beginning of partition)"},
		{name: "latest", spec: OffsetSpec{Mode: OffsetModeLatest}, want: "latest (end of partition / high watermark)"},
		{name: "timestamp", spec: OffsetSpec{Mode: OffsetModeTimestamp, Timestamp: timestamp}, want: "timestamp:2026-04-13T00:00:00Z"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := DescribeOffsetSpec(tt.spec); got != tt.want {
				t.Fatalf("DescribeOffsetSpec() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDisplayPlan(t *testing.T) {
	t.Parallel()

	plan := Plan{
		GroupID: "group-a",
		Topic:   "topic-a",
		Spec:    OffsetSpec{Mode: OffsetModeLatest},
		DryRun:  true,
		Rows: []PlanRow{{
			Partition:     0,
			CurrentOffset: "-",
			NewOffset:     100,
			LowWatermark:  0,
			HighWatermark: 100,
		}},
	}

	var buf bytes.Buffer
	if err := DisplayPlan(&buf, plan); err != nil {
		t.Fatalf("DisplayPlan() error = %v", err)
	}

	out := buf.String()
	for _, want := range []string{"Group : group-a", "Topic : topic-a", "Mode  : DRY RUN", "Partition", "0", "100"} {
		if !strings.Contains(out, want) {
			t.Fatalf("DisplayPlan() output missing %q\n%s", want, out)
		}
	}
}

func TestWriteWarnings(t *testing.T) {
	t.Parallel()

	rows := []PlanRow{{Partition: 0, NewOffset: 5, LowWatermark: 10, HighWatermark: 20}, {Partition: 1, NewOffset: 25, LowWatermark: 10, HighWatermark: 20}}

	var buf bytes.Buffer
	hasWarnings, err := WriteWarnings(&buf, rows)
	if err != nil {
		t.Fatalf("WriteWarnings() error = %v", err)
	}
	if !hasWarnings {
		t.Fatal("WriteWarnings() = false, want true")
	}
	out := buf.String()
	for _, want := range []string{"partition 0", "earliest available", "partition 1", "wait for new messages"} {
		if !strings.Contains(out, want) {
			t.Fatalf("WriteWarnings() output missing %q\n%s", want, out)
		}
	}
}
