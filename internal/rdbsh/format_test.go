package rdbsh

import (
	"strings"
	"testing"
)

func TestIsPrintable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{name: "empty", data: nil, want: true},
		{name: "ascii", data: []byte("hello"), want: true},
		{name: "whitespace", data: []byte("a\n\tb\r"), want: true},
		{name: "binary", data: []byte{0x00, 0x01, 0xFF}, want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isPrintable(tt.data); got != tt.want {
				t.Fatalf("isPrintable(%v) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
		want string
	}{
		{name: "empty", data: nil, want: "(empty)"},
		{name: "ascii", data: []byte("hello"), want: "hello"},
		{name: "binary", data: []byte{0x00, 0x01, 0xAB}, want: "0001ab"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := formatBytes(tt.data); got != tt.want {
				t.Fatalf("formatBytes(%v) = %q, want %q", tt.data, got, tt.want)
			}
		})
	}
}

func TestParseInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    []byte
		wantErr string
	}{
		{name: "ascii", raw: "hello", want: []byte("hello")},
		{name: "hex lowercase", raw: "0x68656c6c6f", want: []byte("hello")},
		{name: "hex uppercase prefix", raw: "0X00ff", want: []byte{0x00, 0xFF}},
		{name: "empty hex", raw: "0x", want: []byte{}},
		{name: "invalid hex", raw: "0x0g", wantErr: "invalid hex input"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseInput(tt.raw)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseInput(%q) error = %v, want substring %q", tt.raw, err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseInput(%q) error = %v", tt.raw, err)
			}
			if string(got) != string(tt.want) {
				t.Fatalf("parseInput(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
