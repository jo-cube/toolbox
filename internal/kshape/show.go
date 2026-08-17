package kshape

import (
	"fmt"
	"io"
	"math"
	"math/bits"
	"strings"
)

const showBarWidth = 24

// Show writes a concise, plain-text view of the supplied stream shape.
func Show(w io.Writer, summary *Summary) error {
	if err := validateSummary(summary); err != nil {
		return err
	}
	whole, err := summary.Report(0)
	if err != nil {
		return err
	}
	overview, _, err := buildRenderOverview(summary, whole)
	if err != nil {
		return err
	}

	topic := summary.Topic
	if topic == "" {
		topic = "(none)"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "topic: %s\n", topic)
	fmt.Fprintf(&out, "partitions: %d  records: %s  keys: %s  visible versions/key: %s\n",
		overview.PartitionCount, overview.Records, overview.DistinctKeys, strings.ReplaceAll(overview.RecordsPerKey, "×", "x"))
	fmt.Fprintf(&out, "density: %s of observed offset spans\n", overview.Occupancy)
	if overview.HasTombstones {
		fmt.Fprintf(&out, "tombstones: %s (%s of visible records)\n", overview.Tombstones, overview.TombstoneRatio)
	}
	out.WriteString("\noffset density\n")
	if len(whole.Partitions) == 0 {
		out.WriteString("(no observed partitions)\n")
		_, err = io.WriteString(w, out.String())
		return err
	}

	labelWidth := 0
	for _, partition := range whole.Partitions {
		labelWidth = max(labelWidth, len(fmt.Sprintf("p%d", partition.Partition)))
	}
	for _, partition := range whole.Partitions {
		region := partition.Regions[0]
		label := fmt.Sprintf("p%d", partition.Partition)
		fmt.Fprintf(&out, "%*s  [%s]  %s %s  %s dense\n", labelWidth, label,
			densityBar(region, summary.Partitions[partition.Partition], summary.BucketWidth), formatUint(region.ObservedRecords),
			plural(region.ObservedRecords, "record", "records"), formatPercent(region.ObservedOccupancy))
	}
	_, err = io.WriteString(w, out.String())
	return err
}

func densityBar(partition RegionReport, source *Partition, bucketWidth uint64) string {
	columns := uint64(showBarWidth)
	if partition.ObservedSpan < columns {
		columns = partition.ObservedSpan
	}
	values := make([]float64, columns)
	first := uint64(partition.ObservedFirstOffset)
	last := uint64(partition.ObservedLastOffset)
	span := partition.ObservedSpan
	for _, bucket := range sortedRegions(source.Regions) {
		region := source.Regions[bucket]
		start := max(bucket*bucketWidth, first)
		end := min(bucket*bucketWidth+bucketWidth-1, last)
		if start > end {
			continue
		}
		regionSpan := end - start + 1
		regionDensity := float64(region.Records) / float64(regionSpan)
		firstColumn := mulDiv(start-first, columns, span)
		lastColumn := min(mulDiv(end-first, columns, span), columns-1)
		for column := firstColumn; column <= lastColumn; column++ {
			cellStart := first + mulDiv(span, column, columns)
			cellEnd := first + mulDiv(span, column+1, columns) - 1
			overlapStart := max(start, cellStart)
			overlapEnd := min(end, cellEnd)
			if overlapStart <= overlapEnd {
				values[column] += regionDensity * float64(overlapEnd-overlapStart+1) / float64(cellEnd-cellStart+1)
			}
		}
	}
	const shades = " .:-=+*#%@"
	bar := make([]byte, len(values))
	for i, value := range values {
		index := int(math.Ceil(min(value, 1) * float64(len(shades)-1)))
		bar[i] = shades[index]
	}
	return string(bar)
}

func mulDiv(value, multiplier, divisor uint64) uint64 {
	high, low := bits.Mul64(value, multiplier)
	quotient, _ := bits.Div64(high, low, divisor)
	return quotient
}

func plural(value uint64, one, many string) string {
	if value == 1 {
		return one
	}
	return many
}
