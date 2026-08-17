package kshape

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestShowMakesRepresentativeShapesObvious(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       []byte
		bucketWidth uint64
		want        []string
	}{
		{
			name: "dense append",
			input: records(24, func(offset int) []byte {
				return frame("events", 0, int64(offset), 1_000, 10, []byte(fmt.Sprint(offset)), false)
			}),
			bucketWidth: 1,
			want:        []string{"density: 100.0%", "p0  [@@@@@@@@@@@@@@@@@@@@@@@@]  24 records  100.0% dense"},
		},
		{
			name: "sparse retained shape",
			input: joinFrames(
				frame("events", 0, 0, 1_000, 10, []byte("a"), false),
				frame("events", 0, 23, 2_000, 10, []byte("b"), false),
			),
			bucketWidth: 1,
			want:        []string{"density: 8.3%", "p0  [@                      @]  2 records  8.3% dense"},
		},
		{
			name: "visible key churn",
			input: records(12, func(offset int) []byte {
				return frame("events", 0, int64(offset), 1_000, 10, []byte("same"), false)
			}),
			bucketWidth: 1,
			want:        []string{"keys: ~1  visible versions/key: ~12x"},
		},
		{
			name: "tombstone transform",
			input: records(6, func(offset int) []byte {
				payloadLength := int64(10)
				if offset >= 2 {
					payloadLength = -1
				}
				return frame("events", 0, int64(offset), 1_000, payloadLength, []byte(fmt.Sprint(offset)), false)
			}),
			bucketWidth: 1,
			want:        []string{"tombstones: 4 (66.7% of visible records)"},
		},
		{
			name: "partition skew",
			input: joinFrames(
				records(24, func(offset int) []byte {
					return frame("events", 0, int64(offset), 1_000, 10, []byte(fmt.Sprint(offset)), false)
				}),
				frame("events", 1, 0, 1_000, 10, []byte("a"), false),
				frame("events", 1, 23, 2_000, 10, []byte("b"), false),
			),
			bucketWidth: 1,
			want:        []string{"p0  [@@@@@@@@@@@@@@@@@@@@@@@@]  24 records", "p1  [@                      @]  2 records"},
		},
		{
			name:        "extreme offset",
			input:       frame("events", 0, math.MaxInt64, -1, 0, nil, false),
			bucketWidth: 1 << 63,
			want:        []string{"p0  [@]  1 record  100.0% dense"},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			summary, err := Build(bytes.NewReader(test.input), test.bucketWidth, 8)
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := Show(&out, summary); err != nil {
				t.Fatal(err)
			}
			for _, want := range test.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("show output does not contain %q:\n%s", want, out.String())
				}
			}
		})
	}
}

func records(count int, makeRecord func(int) []byte) []byte {
	frames := make([][]byte, count)
	for offset := range count {
		frames[offset] = makeRecord(offset)
	}
	return joinFrames(frames...)
}
