package card

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
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

func TestExplicitStdinPath(t *testing.T) {
	t.Parallel()

	got, err := RunFrom([]string{"-"}, Config{Mode: "json", JSONPaths: []string{".user_id"}, Precision: 8}, bytes.NewBufferString(jsonEvents))
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ApproxUnique != 2 {
		t.Fatalf("profile = %#v", got[0])
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

func TestJSONProfileDistinguishesTypesAndNormalizesObjects(t *testing.T) {
	t.Parallel()
	input := `{"v":1}
{"v":"1"}
{"v":true}
{"v":"true"}
{"v":[]}
{"v":"[]"}
{"v":{"a":1,"b":2}}
{"v":{"b":2,"a":1}}
{"v":null}
{"v":""}
{}
`
	profiles, err := RunFrom(nil, Config{Mode: "json", JSONPaths: []string{".v"}}, strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	got := profiles[0]
	if got.ApproxUnique != 7 || got.Nulls != 1 || got.Empty != 1 || got.Missing != 1 || got.Total != 11 {
		t.Fatalf("profile = %#v", got)
	}
}

func TestCSVFilesEachUseTheirOwnHeader(t *testing.T) {
	t.Parallel()
	path := writeInput(t, "id,country\nu1,US\n")
	profiles, err := RunFrom([]string{path, "-"}, Config{Mode: "csv", Columns: []string{"id"}}, strings.NewReader("country,id\nCA,u2\nUS,u1\n"))
	if err != nil || len(profiles) != 1 || profiles[0].ApproxUnique != 2 || profiles[0].Total != 3 {
		t.Fatalf("profiles = %#v, error = %v", profiles, err)
	}
}

func TestDelimitedSelectionMatchesSplit(t *testing.T) {
	t.Parallel()
	columns := []string{"4", "2", "1", "2", strconv.Itoa(int(^uint(0) >> 1)), "3"}
	for _, delimiter := range []string{"::", "aa", "\t", "💠"} {
		records := []string{
			strings.Join([]string{"a", "", "c", ""}, delimiter),
			strings.Join([]string{"", "b"}, delimiter),
			"solo", "",
			"last" + delimiter,
			strings.Join([]string{strings.Repeat("x", 64<<10), "z", ""}, delimiter),
		}
		cfg := Config{Mode: "delimiter", Delimiter: delimiter, Columns: columns, Precision: 8}
		got, err := RunFrom(nil, cfg, strings.NewReader(strings.Join(records, "\r\n")+"\r"))
		if err != nil {
			t.Fatal(err)
		}
		counters, err := newCounters(columns, cfg.Precision)
		if err != nil {
			t.Fatal(err)
		}
		for _, record := range records {
			parts := strings.Split(record, delimiter)
			for i, col := range columns {
				index, _ := strconv.Atoi(col)
				value, ok := recordValue(parts, index-1)
				addValue(counters[i], value, ok)
			}
		}
		want, err := finish(counters, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("delimiter %q: got %#v, want %#v", delimiter, got, want)
		}
	}
}
