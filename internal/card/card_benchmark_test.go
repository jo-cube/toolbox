package card

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func BenchmarkProfile(b *testing.B) {
	for _, width := range []int{3, 100} {
		fields := make([]string, width)
		columns := make([]string, width)
		for i := range fields {
			fields[i] = fmt.Sprintf("value-%03d", i)
			columns[i] = strconv.Itoa(i + 1)
		}
		var rows strings.Builder
		for i := range 10000 {
			fields[0] = strconv.Itoa(i)
			rows.WriteString(strings.Join(fields, ","))
			rows.WriteByte('\n')
		}
		for _, mode := range []string{"csv", "delimiter"} {
			for _, selection := range []string{"first", "last", "all"} {
				b.Run(fmt.Sprintf("%s/%d/%s", mode, width, selection), func(b *testing.B) {
					selected := columns
					if selection == "first" {
						selected = columns[:1]
					}
					if selection == "last" {
						selected = columns[width-1:]
					}
					input := rows.String()
					if mode == "csv" {
						input = strings.Join(columns, ",") + "\n" + input
					}
					cfg := Config{Mode: mode, Columns: selected, Delimiter: ","}
					b.SetBytes(int64(len(input)))
					b.ReportAllocs()
					for b.Loop() {
						profiles, err := RunFrom(nil, cfg, strings.NewReader(input))
						if err != nil {
							b.Fatal(err)
						}
						if profiles[0].Total != 10000 {
							b.Fatal(profiles[0])
						}
					}
				})
			}
		}
	}
}

func BenchmarkProfileLongDelimited(b *testing.B) {
	for _, size := range []int{8 << 10, 64 << 10} {
		var rows strings.Builder
		for i := range 256 {
			fmt.Fprintf(&rows, "%d,%s,last\n", i, strings.Repeat("x", size))
		}
		input := rows.String()
		for _, column := range []string{"1", "3"} {
			b.Run(fmt.Sprintf("bytes=%d/column=%s", size, column), func(b *testing.B) {
				cfg := Config{Mode: "delimiter", Delimiter: ",", Columns: []string{column}}
				b.SetBytes(int64(len(input)))
				b.ReportAllocs()
				for b.Loop() {
					profiles, err := RunFrom(nil, cfg, strings.NewReader(input))
					if err != nil {
						b.Fatal(err)
					}
					if profiles[0].Total != 256 {
						b.Fatal(profiles[0])
					}
				}
			})
		}
	}
}

func BenchmarkProfileJSON(b *testing.B) {
	var rows strings.Builder
	for i := range 10000 {
		fmt.Fprintf(&rows, "{\"user_id\":%d,\"event\":\"request\",\"metadata\":{\"country\":\"US\"}}\n", i)
	}
	input := rows.String()
	cfg := Config{Mode: "json", JSONPaths: []string{".user_id", ".metadata.country"}}
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	for b.Loop() {
		profiles, err := RunFrom(nil, cfg, strings.NewReader(input))
		if err != nil {
			b.Fatal(err)
		}
		if profiles[0].Total != 10000 {
			b.Fatal(profiles[0])
		}
	}
}
