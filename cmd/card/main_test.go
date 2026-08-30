package main

import "testing"

func TestJSONSelectorDoesNotConsumeRelativePaths(t *testing.T) {
	t.Parallel()

	if !isJSONSelector(".user.id") {
		t.Fatal("JSON selector was rejected")
	}
	for _, path := range []string{"./events.jsonl", "../events.jsonl"} {
		if isJSONSelector(path) {
			t.Fatalf("relative path %q was treated as a JSON selector", path)
		}
	}
}
