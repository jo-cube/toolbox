package rdbsh

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		line    string
		want    []string
		wantErr string
	}{
		{name: "simple", line: "get mykey", want: []string{"get", "mykey"}},
		{name: "quoted", line: `put "hello world" value`, want: []string{"put", "hello world", "value"}},
		{name: "escaped space", line: `get hello\ world`, want: []string{"get", "hello world"}},
		{name: "empty quoted arg", line: `scan "" 10`, want: []string{"scan", "", "10"}},
		{name: "unterminated quote", line: `get "hello`, wantErr: "unterminated quoted string"},
		{name: "unfinished escape", line: `get hello\`, wantErr: "unfinished escape sequence"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := splitArgs(tt.line)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("splitArgs(%q) error = %v, want substring %q", tt.line, err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("splitArgs(%q) error = %v", tt.line, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("splitArgs(%q) = %#v, want %#v", tt.line, got, tt.want)
			}
		})
	}
}
