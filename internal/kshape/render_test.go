package kshape

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestRenderProducesDeterministicOfflineMultiresolutionReport(t *testing.T) {
	t.Parallel()

	summary, err := Build(bytes.NewReader(joinFrames(
		frame("events", 0, 0, 1_000, 5, []byte("same"), false),
		frame("events", 1, 5, -1, 10, nil, true),
		frame("events", 0, 2_000, 2_000, -1, []byte("same"), false),
	)), 4, 8)
	if err != nil {
		t.Fatal(err)
	}
	title := `shape </title><script>alert("no")</script>`
	var first, second bytes.Buffer
	for _, out := range []*bytes.Buffer{&first, &second} {
		if err := Render(out, summary, title, "distinct"); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("rendered HTML is not deterministic")
	}
	html := first.String()
	for _, want := range []string{
		"<!doctype html>", "Partition × offset-space shape", "Approx. distinct keys",
		"theoretical relative error 6.50%", "Zoom stops at the artifact’s finest bucket width",
		"shape &lt;/title&gt;&lt;script&gt;alert",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered HTML does not contain %q", want)
		}
	}
	for _, unwanted := range []string{"<script>alert(\"no\")</script>", "https://", "<script src="} {
		if strings.Contains(html, unwanted) {
			t.Errorf("rendered HTML unexpectedly contains %q", unwanted)
		}
	}

	data := decodeRenderData(t, html)
	if data.InitialMetric != "distinct" || len(data.Partitions) != 2 {
		t.Fatalf("render data = %#v", data)
	}
	if got := data.Partitions[0].Levels; len(got) != 3 || got[0].Width != "4" || got[1].Width != "8" || got[2].Width != "16" {
		t.Fatalf("multiresolution levels = %#v", got)
	}
	for _, level := range data.Partitions[1].Levels {
		if len(level.Regions) != 1 || level.Regions[0].MinTimestamp != nil || level.Regions[0].MaxTimestamp != nil {
			t.Fatalf("missing timestamp region = %#v", level.Regions)
		}
	}
}

func TestRenderHandlesEmptyAndManyPartitionSummaries(t *testing.T) {
	t.Parallel()

	empty, err := Build(bytes.NewReader(nil), 4, 8)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Render(&out, empty, "", "records"); err != nil {
		t.Fatal(err)
	}
	if data := decodeRenderData(t, out.String()); data.Partitions == nil || len(data.Partitions) != 0 {
		t.Fatalf("empty render data = %#v", data)
	}
	if !strings.Contains(out.String(), "No partitions were observed") {
		t.Fatal("empty report has no empty-state explanation")
	}

	frames := make([][]byte, 128)
	for partition := range frames {
		frames[partition] = frame("events", int32(partition), 0, int64(partition), 1, []byte(fmt.Sprint(partition)), false)
	}
	many, err := Build(bytes.NewReader(joinFrames(frames...)), 4, 8)
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Render(&out, many, "", "records"); err != nil {
		t.Fatal(err)
	}
	if data := decodeRenderData(t, out.String()); len(data.Partitions) != len(frames) || data.Partitions[127].ID != 127 {
		t.Fatalf("many-partition render has %d partitions", len(data.Partitions))
	}
	if err := Render(&bytes.Buffer{}, many, "", "made-up"); err == nil {
		t.Fatal("Render accepted an unknown metric")
	}

	maximum, err := Build(bytes.NewReader(frame("events", 0, math.MaxInt64, -1, 0, nil, false)), 1<<63, 8)
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Render(&out, maximum, "", "records"); err != nil {
		t.Fatal(err)
	}
	data := decodeRenderData(t, out.String())
	if data.LastOffset != "9223372036854775807" || data.Partitions[0].Levels[0].Regions[0].End != "9223372036854775807" {
		t.Fatalf("maximum offset lost precision: %#v", data)
	}
}

func decodeRenderData(t *testing.T, html string) renderData {
	t.Helper()
	const marker = `<script id="kshape-data" type="application/json">`
	start := strings.Index(html, marker)
	if start < 0 {
		t.Fatal("rendered HTML has no embedded data")
	}
	start += len(marker)
	end := strings.Index(html[start:], "</script>")
	if end < 0 {
		t.Fatal("embedded data has no closing script tag")
	}
	var data renderData
	if err := json.Unmarshal([]byte(html[start:start+end]), &data); err != nil {
		t.Fatalf("decode embedded data: %v", err)
	}
	return data
}
