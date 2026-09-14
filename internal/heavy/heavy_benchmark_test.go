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
