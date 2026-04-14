package ksetoff

import (
	"fmt"
	"io"
	"time"
)

func DescribeOffsetSpec(spec OffsetSpec) string {
	switch spec.Mode {
	case OffsetModeNumeric:
		return fmt.Sprintf("%d (numeric)", spec.Numeric)
	case OffsetModeEarliest:
		return "earliest (beginning of partition)"
	case OffsetModeLatest:
		return "latest (end of partition / high watermark)"
	case OffsetModeTimestamp:
		return fmt.Sprintf("timestamp:%s", spec.Timestamp.Format(time.RFC3339))
	default:
		return "unknown"
	}
}

func DisplayPlan(w io.Writer, plan Plan) error {
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  Group : %s\n", plan.GroupID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  Topic : %s\n", plan.Topic); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  Offset: %s\n", DescribeOffsetSpec(plan.Spec)); err != nil {
		return err
	}
	if plan.DryRun {
		if _, err := fmt.Fprintln(w, "  Mode  : DRY RUN (no changes will be committed)"); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "  %-12s  %-16s  %-16s  %-16s  %-16s\n", "Partition", "Current Offset", "New Offset", "Low Watermark", "High Watermark"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  %-12s  %-16s  %-16s  %-16s  %-16s\n", "----------", "--------------", "----------", "-------------", "--------------"); err != nil {
		return err
	}

	for _, row := range plan.Rows {
		if _, err := fmt.Fprintf(w, "  %-12d  %-16s  %-16d  %-16d  %-16d\n", row.Partition, row.CurrentOffset, row.NewOffset, row.LowWatermark, row.HighWatermark); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintln(w)
	return err
}

func WriteWarnings(w io.Writer, rows []PlanRow) (bool, error) {
	hasWarnings := false
	for _, row := range rows {
		if row.NewOffset < row.LowWatermark {
			hasWarnings = true
			if _, err := fmt.Fprintf(w, "  warning: partition %d: offset %d < low watermark %d (will consume from earliest available)\n", row.Partition, row.NewOffset, row.LowWatermark); err != nil {
				return hasWarnings, err
			}
		}
		if row.NewOffset > row.HighWatermark {
			hasWarnings = true
			if _, err := fmt.Fprintf(w, "  warning: partition %d: offset %d > high watermark %d (consumer will wait for new messages)\n", row.Partition, row.NewOffset, row.HighWatermark); err != nil {
				return hasWarnings, err
			}
		}
	}

	return hasWarnings, nil
}
