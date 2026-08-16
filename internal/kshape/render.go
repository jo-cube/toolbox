package kshape

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math/big"
	"math/bits"
	"strconv"
	"strings"
	"time"

	"github.com/jo-cube/toolbox/internal/hll"
)

const renderTargetRegions = 240

//go:embed report.html
var reportHTML string

var reportTemplate = template.Must(template.New("report").Parse(reportHTML))

type renderPage struct {
	Title               string
	Topic               string
	Overview            renderOverview
	Partitions          []renderPartitionOverview
	Data                template.JS
	ArtifactBucketWidth string
	HLLRelativeError    string
}

type renderOverview struct {
	PartitionCount int
	Records        string
	PayloadBytes   string
	PayloadExact   string
	Occupancy      string
	Tombstones     string
	TombstoneRatio string
	DistinctKeys   string
	RecordsPerKey  string
	KeyedRecords   string
	NullKeys       string
	MissingTimes   string
	TimestampRange string
}

type renderPartitionOverview struct {
	ID            int32
	Offsets       string
	Records       string
	RecordsBar    float64
	PayloadBytes  string
	PayloadExact  string
	PayloadBar    float64
	Occupancy     string
	Tombstones    string
	DistinctKeys  string
	RecordsPerKey string
}

type renderData struct {
	InitialMetric string            `json:"initialMetric"`
	FirstOffset   string            `json:"firstOffset"`
	LastOffset    string            `json:"lastOffset"`
	Partitions    []renderPartition `json:"partitions"`
}

type renderPartition struct {
	ID     int32         `json:"id"`
	First  string        `json:"first"`
	Last   string        `json:"last"`
	Levels []renderLevel `json:"levels"`
}

type renderLevel struct {
	Width   string         `json:"width"`
	Regions []renderRegion `json:"regions"`
}

type renderRegion struct {
	Start               string  `json:"start"`
	End                 string  `json:"end"`
	First               string  `json:"first"`
	Last                string  `json:"last"`
	Span                string  `json:"span"`
	Records             string  `json:"records"`
	Occupancy           float64 `json:"occupancy"`
	PayloadBytes        string  `json:"payloadBytes"`
	Tombstones          string  `json:"tombstones"`
	NullKeys            string  `json:"nullKeys"`
	KeyedRecords        string  `json:"keyedRecords"`
	MissingTimestamps   string  `json:"missingTimestamps"`
	MinTimestamp        *string `json:"minTimestamp,omitempty"`
	MaxTimestamp        *string `json:"maxTimestamp,omitempty"`
	ApproxDistinctKeys  string  `json:"approxDistinctKeys"`
	ApproxRecordsPerKey float64 `json:"approxRecordsPerKey"`
}

func ValidRenderMetric(metric string) bool {
	switch metric {
	case "records", "occupancy", "bytes", "tombstones", "distinct", "rewrite":
		return true
	default:
		return false
	}
}

func Render(w io.Writer, summary *Summary, title, initialMetric string) error {
	if !ValidRenderMetric(initialMetric) {
		return fmt.Errorf("invalid render metric %q", initialMetric)
	}
	if err := validateSummary(summary); err != nil {
		return err
	}
	whole, err := summary.Report(0)
	if err != nil {
		return err
	}
	page, data, err := buildRenderPage(summary, whole, title, initialMetric)
	if err != nil {
		return err
	}
	rawData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	page.Data = template.JS(rawData)
	return reportTemplate.Execute(w, page)
}

func buildRenderPage(summary *Summary, whole Report, title, initialMetric string) (renderPage, renderData, error) {
	if title == "" {
		title = "Empty stream shape"
		if summary.Topic != "" {
			title = summary.Topic + " stream shape"
		}
	}
	page := renderPage{
		Title:               title,
		Topic:               summary.Topic,
		ArtifactBucketWidth: formatUint(summary.BucketWidth),
		HLLRelativeError:    fmt.Sprintf("%.2f%%", whole.HLLRelativeError*100),
	}
	data := renderData{InitialMetric: initialMetric, Partitions: []renderPartition{}}
	if len(whole.Partitions) == 0 {
		page.Overview = renderOverview{
			Records: "0", PayloadBytes: "0 B", PayloadExact: "0 bytes", Occupancy: "—",
			Tombstones: "0", TombstoneRatio: "—", DistinctKeys: "≈0", RecordsPerKey: "—",
			KeyedRecords: "0", NullKeys: "0", MissingTimes: "0", TimestampRange: "No observed timestamps",
		}
		return page, data, nil
	}

	widths := renderWidths(summary.BucketWidth, whole)
	var firstOffset, lastOffset int64
	firstSet := false
	for _, partition := range whole.Partitions {
		region := partition.Regions[0]
		if !firstSet || region.ObservedFirstOffset < firstOffset {
			firstOffset = region.ObservedFirstOffset
		}
		if !firstSet || region.ObservedLastOffset > lastOffset {
			lastOffset = region.ObservedLastOffset
		}
		firstSet = true
		data.Partitions = append(data.Partitions, renderPartition{
			ID: partition.Partition, First: strconv.FormatInt(region.ObservedFirstOffset, 10),
			Last: strconv.FormatInt(region.ObservedLastOffset, 10), Levels: make([]renderLevel, 0, len(widths)),
		})
	}
	for _, width := range widths {
		report, err := summary.Report(width)
		if err != nil {
			return renderPage{}, renderData{}, err
		}
		for i, reportedPartition := range report.Partitions {
			if reportedPartition.Partition != data.Partitions[i].ID {
				return renderPage{}, renderData{}, fmt.Errorf("partition report order changed")
			}
			level := renderLevel{
				Width:   strconv.FormatUint(width, 10),
				Regions: make([]renderRegion, 0, len(reportedPartition.Regions)),
			}
			for _, reportedRegion := range reportedPartition.Regions {
				level.Regions = append(level.Regions, makeRenderRegion(reportedRegion))
			}
			data.Partitions[i].Levels = append(data.Partitions[i].Levels, level)
		}
	}
	data.FirstOffset = strconv.FormatInt(firstOffset, 10)
	data.LastOffset = strconv.FormatInt(lastOffset, 10)

	overview, partitions, err := buildRenderOverview(summary, whole)
	if err != nil {
		return renderPage{}, renderData{}, err
	}
	page.Overview, page.Partitions = overview, partitions
	return page, data, nil
}

func renderWidths(finest uint64, whole Report) []uint64 {
	coarsest, maxSpan := finest, uint64(0)
	for _, partition := range whole.Partitions {
		maxSpan = max(maxSpan, partition.Regions[0].ObservedSpan)
	}
	for coarsest < 1<<63 && maxSpan != 0 && (maxSpan-1)/coarsest+1 > renderTargetRegions {
		coarsest <<= 1
	}
	steps := bits.TrailingZeros64(coarsest / finest)
	middle := finest << (steps / 2)
	// ponytail: three derived levels cap HTML growth; add more only if real zoom use needs smoother transitions.
	widths := []uint64{finest}
	if middle != finest && middle != coarsest {
		widths = append(widths, middle)
	}
	if coarsest != finest {
		widths = append(widths, coarsest)
	}
	return widths
}

func makeRenderRegion(region RegionReport) renderRegion {
	rendered := renderRegion{
		Start: strconv.FormatUint(region.RegionFirstOffset, 10), End: strconv.FormatUint(region.RegionLastOffset, 10),
		First: strconv.FormatInt(region.ObservedFirstOffset, 10), Last: strconv.FormatInt(region.ObservedLastOffset, 10),
		Span: strconv.FormatUint(region.ObservedSpan, 10), Records: strconv.FormatUint(region.ObservedRecords, 10),
		Occupancy: region.ObservedOccupancy, PayloadBytes: strconv.FormatUint(region.LogicalPayloadBytes, 10),
		Tombstones: strconv.FormatUint(region.ObservedTombstones, 10), NullKeys: strconv.FormatUint(region.NullKeys, 10),
		KeyedRecords: strconv.FormatUint(region.KeyedRecords, 10), MissingTimestamps: strconv.FormatUint(region.MissingTimestamps, 10),
		ApproxDistinctKeys: strconv.FormatUint(region.ApproxDistinctKeys, 10), ApproxRecordsPerKey: region.ApproxRecordsPerKey,
	}
	if region.MinTimestamp != nil {
		minimum, maximum := strconv.FormatInt(*region.MinTimestamp, 10), strconv.FormatInt(*region.MaxTimestamp, 10)
		rendered.MinTimestamp, rendered.MaxTimestamp = &minimum, &maximum
	}
	return rendered
}

func buildRenderOverview(summary *Summary, whole Report) (renderOverview, []renderPartitionOverview, error) {
	records, payload, tombstones := new(big.Int), new(big.Int), new(big.Int)
	keyed, nullKeys, spans, missingTimes := new(big.Int), new(big.Int), new(big.Int), new(big.Int)
	distinctSketch, _ := hll.New(summary.Precision)
	var minTimestamp, maxTimestamp int64
	hasTimestamp := false
	partitions := make([]renderPartitionOverview, 0, len(whole.Partitions))
	var maxRecords, maxPayload uint64

	for _, partition := range whole.Partitions {
		region := partition.Regions[0]
		addUint(records, region.ObservedRecords)
		addUint(payload, region.LogicalPayloadBytes)
		addUint(tombstones, region.ObservedTombstones)
		addUint(keyed, region.KeyedRecords)
		addUint(nullKeys, region.NullKeys)
		addUint(spans, region.ObservedSpan)
		addUint(missingTimes, region.MissingTimestamps)
		maxRecords = max(maxRecords, region.ObservedRecords)
		maxPayload = max(maxPayload, region.LogicalPayloadBytes)
		if region.MinTimestamp != nil {
			if !hasTimestamp || *region.MinTimestamp < minTimestamp {
				minTimestamp = *region.MinTimestamp
			}
			if !hasTimestamp || *region.MaxTimestamp > maxTimestamp {
				maxTimestamp = *region.MaxTimestamp
			}
			hasTimestamp = true
		}
		for _, sourceRegion := range summary.Partitions[partition.Partition].Regions {
			if err := distinctSketch.Merge(sourceRegion.Keys); err != nil {
				return renderOverview{}, nil, err
			}
		}
		partitions = append(partitions, renderPartitionOverview{
			ID:      partition.Partition,
			Offsets: fmt.Sprintf("%s–%s", formatInt(region.ObservedFirstOffset), formatInt(region.ObservedLastOffset)),
			Records: formatUint(region.ObservedRecords), PayloadBytes: formatBytes(region.LogicalPayloadBytes),
			PayloadExact: formatUint(region.LogicalPayloadBytes) + " bytes", Occupancy: formatPercent(region.ObservedOccupancy),
			Tombstones:   fmt.Sprintf("%s (%s)", formatUint(region.ObservedTombstones), ratioPercent(region.ObservedTombstones, region.ObservedRecords)),
			DistinctKeys: "≈" + formatUint(region.ApproxDistinctKeys), RecordsPerKey: formatFactor(region.ApproxRecordsPerKey),
		})
	}
	for i, partition := range whole.Partitions {
		region := partition.Regions[0]
		if maxRecords != 0 {
			partitions[i].RecordsBar = float64(region.ObservedRecords) / float64(maxRecords)
		}
		if maxPayload != 0 {
			partitions[i].PayloadBar = float64(region.LogicalPayloadBytes) / float64(maxPayload)
		}
	}

	distinct := distinctSketch.Estimate()
	if keyed.IsUint64() {
		distinct = min(distinct, keyed.Uint64())
	}
	overview := renderOverview{
		PartitionCount: len(whole.Partitions), Records: formatBig(records), PayloadBytes: formatBigBytes(payload),
		PayloadExact: formatBig(payload) + " bytes", Occupancy: bigPercent(records, spans),
		Tombstones: formatBig(tombstones), TombstoneRatio: bigPercent(tombstones, records), DistinctKeys: "≈" + formatUint(distinct),
		RecordsPerKey: bigFactor(keyed, distinct), KeyedRecords: formatBig(keyed), NullKeys: formatBig(nullKeys),
		MissingTimes: formatBig(missingTimes), TimestampRange: "No observed timestamps",
	}
	if hasTimestamp {
		overview.TimestampRange = formatTimestamp(minTimestamp) + " – " + formatTimestamp(maxTimestamp)
	}
	return overview, partitions, nil
}

func addUint(total *big.Int, value uint64) {
	total.Add(total, new(big.Int).SetUint64(value))
}

func formatBig(value *big.Int) string { return commas(value.String()) }

func formatUint(value uint64) string { return commas(strconv.FormatUint(value, 10)) }

func formatInt(value int64) string { return commas(strconv.FormatInt(value, 10)) }

func commas(value string) string {
	start := 0
	if strings.HasPrefix(value, "-") {
		start = 1
	}
	for i := len(value) - 3; i > start; i -= 3 {
		value = value[:i] + "," + value[i:]
	}
	return value
}

func formatPercent(value float64) string { return fmt.Sprintf("%.2f%%", value*100) }

func ratioPercent(numerator, denominator uint64) string {
	if denominator == 0 {
		return "—"
	}
	return formatPercent(float64(numerator) / float64(denominator))
}

func bigPercent(numerator, denominator *big.Int) string {
	if denominator.Sign() == 0 {
		return "—"
	}
	value, _ := new(big.Rat).SetFrac(numerator, denominator).Float64()
	return formatPercent(value)
}

func formatFactor(value float64) string {
	if value == 0 {
		return "—"
	}
	return fmt.Sprintf("≈%.2f×", value)
}

func bigFactor(records *big.Int, distinct uint64) string {
	if distinct == 0 {
		return "—"
	}
	value, _ := new(big.Rat).SetFrac(records, new(big.Int).SetUint64(distinct)).Float64()
	return formatFactor(value)
}

func formatBytes(value uint64) string { return humanBytes(new(big.Int).SetUint64(value)) }

func formatBigBytes(value *big.Int) string { return humanBytes(value) }

func humanBytes(value *big.Int) string {
	if value.Cmp(big.NewInt(1024)) < 0 {
		return formatBig(value) + " B"
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	amount := new(big.Float).SetInt(value)
	divisor := big.NewFloat(1024)
	unit := units[0]
	for i := 0; i < len(units); i++ {
		amount.Quo(amount, divisor)
		unit = units[i]
		if amount.Cmp(divisor) < 0 || i == len(units)-1 {
			break
		}
	}
	formatted := amount.Text('f', 1)
	formatted = strings.TrimSuffix(formatted, ".0")
	return formatted + " " + unit
}

func formatTimestamp(value int64) string {
	return time.UnixMilli(value).UTC().Format("2006-01-02 15:04:05.000Z")
}
