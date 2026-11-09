package heavy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func BenchmarkHighCardinality(b *testing.B) {
	for _, skewed := range []bool{false, true} {
		var input strings.Builder
		for i := range 100000 {
			if skewed && i%10 != 0 {
				fmt.Fprintln(&input, "dominant")
			} else {
				fmt.Fprintf(&input, "key-%08d\n", i)
			}
		}
		path := filepath.Join(b.TempDir(), "input")
		if err := os.WriteFile(path, []byte(input.String()), 0600); err != nil {
			b.Fatal(err)
		}
		for _, capacity := range []int{1000, 10000} {
			b.Run(fmt.Sprintf("skewed=%t/capacity=%d", skewed, capacity), func(b *testing.B) {
				b.SetBytes(int64(input.Len()))
				b.ReportAllocs()
				for b.Loop() {
					if _, err := Run([]string{path}, Config{Top: 20, Capacity: capacity}); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func BenchmarkExactRanking(b *testing.B) {
	for _, distinct := range []int{32, 1000, 100000, 1000000} {
		counts := make(map[string]uint64, distinct)
		for i := range distinct {
			counts[fmt.Sprintf("key-%08d", i)] = 1
		}
		tops := []int{20, distinct * 9 / 10}
		if distinct == 32 {
			tops = []int{1, 20}
		}
		if distinct == 100000 {
			tops = []int{20, distinct / 8, distinct / 4, distinct / 2, distinct * 9 / 10}
		}
		for _, top := range tops {
			b.Run(fmt.Sprintf("distinct=%d/top=%d", distinct, top), func(b *testing.B) {
				b.ReportAllocs()
				var results []Result
				for b.Loop() {
					results = ranked(counts, top)
				}
				if len(results) != min(top, distinct) {
					b.Fatal(len(results))
				}
			})
		}
	}
}

func BenchmarkExactCounting(b *testing.B) {
	for _, tt := range []struct {
		name    string
		records int
		skewed  bool
	}{
		{name: "distinct_100k", records: 100000},
		{name: "distinct_1m", records: 1000000},
		{name: "skewed_1m", records: 1000000, skewed: true},
	} {
		b.Run(tt.name, func(b *testing.B) {
			var input strings.Builder
			for i := range tt.records {
				if tt.skewed && i%10 != 0 {
					fmt.Fprintln(&input, "dominant")
				} else {
					fmt.Fprintf(&input, "key-%08d\n", i)
				}
			}
			path := filepath.Join(b.TempDir(), "input")
			if err := os.WriteFile(path, []byte(input.String()), 0600); err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(input.Len()))
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Run([]string{path}, Config{Top: 20, Exact: true}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
