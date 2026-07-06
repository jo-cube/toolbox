package card

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCSVProfile(t *testing.T) {
	t.Parallel()

	path := writeInput(t, "user_id,country\nu1,US\nu2,CA\nu1,\n")
	got, err := Run([]string{path}, Config{Mode: "csv", Columns: []string{"user_id", "country"}, Precision: 8})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ApproxUnique != 2 || got[0].Total != 3 {
		t.Fatalf("user_id profile = %#v", got[0])
	}
	if got[1].Empty != 1 {
		t.Fatalf("country profile = %#v, want one empty", got[1])
	}
}

func TestJSONProfile(t *testing.T) {
	t.Parallel()

	path := writeInput(t, `{"user_id":"u1","country":"US"}`+"\n"+`{"user_id":"u2","country":null}`+"\n"+`{"user_id":"u1"}`+"\n")
	got, err := Run([]string{path}, Config{Mode: "json", JSONPaths: []string{".user_id", ".country"}, Precision: 8})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ApproxUnique != 2 || got[1].Nulls != 1 || got[1].Missing != 1 {
		t.Fatalf("Run() = %#v", got)
	}
}

func TestDelimitedColumnsAreOneBased(t *testing.T) {
	t.Parallel()

	path := writeInput(t, "a,b\nc,d\n")
	got, err := Run([]string{path}, Config{Mode: "delimiter", Delimiter: ",", Columns: []string{"2"}, Precision: 8})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Field != "2" || got[0].ApproxUnique != 2 {
		t.Fatalf("Run() = %#v", got)
	}
}

func writeInput(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
