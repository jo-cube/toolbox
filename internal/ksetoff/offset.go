package ksetoff

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
)

type OffsetMode int

const (
	OffsetModeNumeric OffsetMode = iota
	OffsetModeEarliest
	OffsetModeLatest
	OffsetModeTimestamp
)

type OffsetSpec struct {
	Mode      OffsetMode
	Numeric   int64
	Timestamp time.Time
}

type RunParams struct {
	GroupID             string
	Topic               string
	Spec                OffsetSpec
	RequestedPartitions []int32
	DryRun              bool
	Out                 io.Writer
	ErrOut              io.Writer
}

type PlanRow struct {
	Partition     int32
	CurrentOffset string
	NewOffset     int64
	LowWatermark  int64
	HighWatermark int64
}

type Plan struct {
	GroupID string
	Topic   string
	Spec    OffsetSpec
	DryRun  bool
	Rows    []PlanRow
}

func ParseOffsetSpec(raw string) (OffsetSpec, error) {
	switch strings.ToLower(raw) {
	case "earliest", "beginning":
		return OffsetSpec{Mode: OffsetModeEarliest}, nil
	case "latest", "end":
		return OffsetSpec{Mode: OffsetModeLatest}, nil
	}

	if after, found := strings.CutPrefix(raw, "timestamp:"); found {
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05",
			"2006-01-02",
		}
		for _, layout := range formats {
			timestamp, err := time.Parse(layout, after)
			if err == nil {
				return OffsetSpec{Mode: OffsetModeTimestamp, Timestamp: timestamp}, nil
			}
		}
		return OffsetSpec{}, fmt.Errorf("invalid timestamp %q: expected ISO-8601 format (e.g. 2026-04-13T00:00:00Z or 2026-04-13)", after)
	}

	numeric, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return OffsetSpec{}, fmt.Errorf("invalid offset %q: expected a number, 'earliest', 'latest', or 'timestamp:<ISO-8601>'", raw)
	}
	if numeric < 0 {
		return OffsetSpec{}, fmt.Errorf("offset must be non-negative, got %d", numeric)
	}

	return OffsetSpec{Mode: OffsetModeNumeric, Numeric: numeric}, nil
}

func ParsePartitions(raw string) ([]int32, error) {
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	result := make([]int32, 0, len(parts))
	seen := map[int32]bool{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		numeric, err := strconv.ParseInt(part, 10, 32)
		if err != nil || numeric < 0 {
			return nil, fmt.Errorf("invalid partition %q: must be a non-negative integer", part)
		}

		partition := int32(numeric)
		if seen[partition] {
			continue
		}
		seen[partition] = true
		result = append(result, partition)
	}

	if len(result) == 0 {
		return nil, nil
	}

	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

func Run(ctx context.Context, admin *kadm.Client, params RunParams) error {
	out := params.Out
	if out == nil {
		out = io.Discard
	}
	errOut := params.ErrOut
	if errOut == nil {
		errOut = io.Discard
	}

	topics, err := admin.ListTopics(ctx, params.Topic)
	if err != nil {
		return fmt.Errorf("failed to fetch metadata: %w", err)
	}

	topicDetail, ok := topics[params.Topic]
	if !ok {
		return fmt.Errorf("topic %q not found in cluster", params.Topic)
	}
	if topicDetail.Err != nil {
		return fmt.Errorf("topic %q: %w", params.Topic, topicDetail.Err)
	}

	totalPartitions := int32(len(topicDetail.Partitions))
	if totalPartitions == 0 {
		return fmt.Errorf("topic %q has 0 partitions", params.Topic)
	}

	if _, err := fmt.Fprintf(errOut, "Connected to cluster. Topic %q has %d partition(s).\n", params.Topic, totalPartitions); err != nil {
		return err
	}

	targetPartitions, err := targetPartitions(totalPartitions, params.RequestedPartitions)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	startOffsets, err := admin.ListStartOffsets(ctx, params.Topic)
	if err != nil {
		return fmt.Errorf("failed to list start offsets: %w", err)
	}
	endOffsets, err := admin.ListEndOffsets(ctx, params.Topic)
	if err != nil {
		return fmt.Errorf("failed to list end offsets: %w", err)
	}

	rows, err := resolvePlanRows(ctx, admin, params.Topic, params.Spec, targetPartitions, startOffsets, endOffsets)
	if err != nil {
		return err
	}

	currentOffsets, err := fetchCurrentOffsets(ctx, admin, params.GroupID, params.Topic, targetPartitions)
	if err != nil {
		for partition := range currentOffsets {
			currentOffsets[partition] = "?"
		}
	}

	for i := range rows {
		rows[i].CurrentOffset = currentOffsets[rows[i].Partition]
	}

	plan := Plan{
		GroupID: params.GroupID,
		Topic:   params.Topic,
		Spec:    params.Spec,
		DryRun:  params.DryRun,
		Rows:    rows,
	}

	if err := DisplayPlan(out, plan); err != nil {
		return err
	}

	hasWarnings, err := WriteWarnings(errOut, rows)
	if err != nil {
		return err
	}
	if hasWarnings {
		if _, err := fmt.Fprintln(errOut); err != nil {
			return err
		}
	}

	if params.DryRun {
		_, err := fmt.Fprintln(out, "  Dry run complete. No offsets were committed.")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out)
		return err
	}

	offsets := make(kadm.Offsets)
	for _, row := range rows {
		offsets.Add(kadm.Offset{
			Topic:       params.Topic,
			Partition:   row.Partition,
			At:          row.NewOffset,
			LeaderEpoch: -1,
		})
	}

	committed, err := admin.CommitOffsets(ctx, params.GroupID, offsets)
	if err != nil {
		if strings.Contains(err.Error(), "not empty") || strings.Contains(err.Error(), "ILLEGAL_GENERATION") || strings.Contains(err.Error(), "REBALANCE_IN_PROGRESS") {
			return fmt.Errorf("consumer group %q has active members. Stop all consumers in the group before resetting offsets", params.GroupID)
		}
		return fmt.Errorf("failed to commit offsets: %w", err)
	}

	var commitErrors []string
	committed.Each(func(offset kadm.OffsetResponse) {
		if offset.Err != nil {
			commitErrors = append(commitErrors, fmt.Sprintf("partition %d: %v", offset.Partition, offset.Err))
		}
	})
	if len(commitErrors) > 0 {
		return fmt.Errorf("some partitions failed to commit:\n  %s", strings.Join(commitErrors, "\n  "))
	}

	_, err = fmt.Fprintf(out, "  Successfully committed offsets for %d partition(s).\n\n", len(rows))
	return err
}

func targetPartitions(totalPartitions int32, requested []int32) ([]int32, error) {
	if requested == nil {
		result := make([]int32, totalPartitions)
		for i := int32(0); i < totalPartitions; i++ {
			result[i] = i
		}
		return result, nil
	}

	for _, partition := range requested {
		if partition < 0 || partition >= totalPartitions {
			return nil, fmt.Errorf("partition %d out of range (topic has partitions 0..%d)", partition, totalPartitions-1)
		}
	}

	return requested, nil
}

func resolvePlanRows(
	ctx context.Context,
	admin *kadm.Client,
	topic string,
	spec OffsetSpec,
	targetPartitions []int32,
	startOffsets kadm.ListedOffsets,
	endOffsets kadm.ListedOffsets,
) ([]PlanRow, error) {
	type watermark struct {
		low  int64
		high int64
	}

	watermarks := make(map[int32]watermark, len(targetPartitions))
	for _, partition := range targetPartitions {
		var wm watermark
		if startOffset, ok := startOffsets.Lookup(topic, partition); ok {
			wm.low = startOffset.Offset
		}
		if endOffset, ok := endOffsets.Lookup(topic, partition); ok {
			wm.high = endOffset.Offset
		}
		watermarks[partition] = wm
	}

	rows := make([]PlanRow, 0, len(targetPartitions))
	switch spec.Mode {
	case OffsetModeNumeric:
		for _, partition := range targetPartitions {
			wm := watermarks[partition]
			rows = append(rows, PlanRow{Partition: partition, NewOffset: spec.Numeric, LowWatermark: wm.low, HighWatermark: wm.high})
		}
	case OffsetModeEarliest:
		for _, partition := range targetPartitions {
			wm := watermarks[partition]
			rows = append(rows, PlanRow{Partition: partition, NewOffset: wm.low, LowWatermark: wm.low, HighWatermark: wm.high})
		}
	case OffsetModeLatest:
		for _, partition := range targetPartitions {
			wm := watermarks[partition]
			rows = append(rows, PlanRow{Partition: partition, NewOffset: wm.high, LowWatermark: wm.low, HighWatermark: wm.high})
		}
	case OffsetModeTimestamp:
		offsets, err := admin.ListOffsetsAfterMilli(ctx, spec.Timestamp.UnixMilli(), topic)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve offsets for timestamp %s: %w", spec.Timestamp.Format(time.RFC3339), err)
		}
		for _, partition := range targetPartitions {
			listedOffset, ok := offsets.Lookup(topic, partition)
			if !ok {
				return nil, fmt.Errorf("no offset returned for partition %d at timestamp %s", partition, spec.Timestamp.Format(time.RFC3339))
			}
			if listedOffset.Err != nil {
				return nil, fmt.Errorf("partition %d at timestamp %s: %w", partition, spec.Timestamp.Format(time.RFC3339), listedOffset.Err)
			}
			wm := watermarks[partition]
			rows = append(rows, PlanRow{Partition: partition, NewOffset: listedOffset.Offset, LowWatermark: wm.low, HighWatermark: wm.high})
		}
	default:
		return nil, fmt.Errorf("unsupported offset mode: %d", spec.Mode)
	}

	return rows, nil
}

func fetchCurrentOffsets(ctx context.Context, admin *kadm.Client, groupID string, topic string, partitions []int32) (map[int32]string, error) {
	currentOffsets := make(map[int32]string, len(partitions))
	fetchedOffsets, err := admin.FetchOffsets(ctx, groupID)
	if err != nil {
		for _, partition := range partitions {
			currentOffsets[partition] = "?"
		}
		return currentOffsets, err
	}

	for _, partition := range partitions {
		if fetchedOffset, ok := fetchedOffsets.Lookup(topic, partition); ok && fetchedOffset.At >= 0 {
			currentOffsets[partition] = strconv.FormatInt(fetchedOffset.At, 10)
			continue
		}
		currentOffsets[partition] = "-"
	}

	return currentOffsets, nil
}
