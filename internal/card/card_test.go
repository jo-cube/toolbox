package card

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const csvUsers = `user_id,country
u1,US
u2,CA
u1,
`

const jsonEvents = `{"user_id":"u1","country":"US"}
{"user_id":"u2","country":null}
{"user_id":"u1"}
`

const commaDelimitedPairs = `a,b
c,d
`

func TestCSVProfileCountsDistinctValuesAndEmptyFields(t *testing.T) {
	t.Parallel()

	path := writeInput(t, csvUsers)
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

func TestJSONProfileCountsNullMissingAndDistinctValues(t *testing.T) {
	t.Parallel()

	path := writeInput(t, jsonEvents)
	got, err := Run([]string{path}, Config{Mode: "json", JSONPaths: []string{".user_id", ".country"}, Precision: 8})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ApproxUnique != 2 || got[1].Nulls != 1 || got[1].Missing != 1 {
		t.Fatalf("Run() = %#v", got)
	}
}

func TestJSONProfileHandlesLongLines(t *testing.T) {
	t.Parallel()

	longValue := strings.Repeat("x", 1024*1024+1)
	path := writeInput(t, `{"message":"`+longValue+`"}`+"\n")
	got, err := Run([]string{path}, Config{Mode: "json", JSONPaths: []string{".message"}, Precision: 8})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ApproxUnique != 1 || got[0].Total != 1 {
		t.Fatalf("message profile = %#v", got[0])
	}
}

func TestJSONProfilePreservesNumberEncoding(t *testing.T) {
	t.Parallel()

	path := writeInput(t, "{\"id\":9007199254740992}\n{\"id\":9007199254740993}\n{\"id\":1}\n{\"id\":1.0}\n")
	got, err := Run([]string{path}, Config{Mode: "json", JSONPaths: []string{".id"}, Precision: 8})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ApproxUnique != 4 {
		t.Fatalf("approx_unique = %d, want 4 distinct JSON encodings", got[0].ApproxUnique)
	}
}

func TestDelimitedColumnsAreOneBased(t *testing.T) {
	t.Parallel()

	path := writeInput(t, commaDelimitedPairs)
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
