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
