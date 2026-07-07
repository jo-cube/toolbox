package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/jo-cube/toolbox/internal/buildinfo"
	"github.com/jo-cube/toolbox/internal/card"
	"github.com/jo-cube/toolbox/internal/hll"
)

func main() {
	showVersion := flag.Bool("version", false, "print version information")
	csvMode := flag.Bool("csv", false, "read CSV with a header row")
	delimiter := flag.String("delimiter", "", "read delimited text with this delimiter")
	columnsRaw := flag.String("columns", "", "comma-separated CSV columns or 1-based delimited fields")
	jsonMode := flag.Bool("json", false, "read JSON Lines; leading .paths are selectors")
	jsonOut := flag.Bool("output-json", false, "write JSON output")
	precision := flag.Uint("precision", uint(hll.DefaultP), "HLL precision, 4..20")

	flag.Usage = func() {
		name := filepath.Base(os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), `Usage: %s (--csv --columns a,b | --delimiter ',' --columns 1,2 | --json .path...) [file...]

Profile approximate cardinality for explicit CSV columns, delimited fields, or JSON Lines paths.

Examples:
  card --csv --columns user_id,country users.csv
  card --delimiter $'\t' --columns 1,3 data.tsv
  card --json .user_id .event_type events.jsonl

Notes:
  CSV mode selects header names.
  Delimited mode selects 1-based field numbers.
  JSON paths are simple dot paths; filters and array traversal are not supported.

Options:
`, name)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Fprintf(os.Stdout, "card %s\n", buildinfo.Version())
		return
	}

	mode := ""
	if *csvMode {
		mode = "csv"
	}
	if *jsonMode {
		if mode != "" {
			usageError("choose one input mode")
		}
		mode = "json"
	}
	if *delimiter != "" {
		if mode != "" {
			usageError("choose one input mode")
		}
		mode = "delimiter"
	}
	if mode == "" {
		usageError("choose one input mode")
	}

	args := flag.Args()
	var paths []string
	var jsonPaths []string
	if mode == "json" {
		for len(args) > 0 && strings.HasPrefix(args[0], ".") {
			jsonPaths = append(jsonPaths, args[0])
			args = args[1:]
		}
		paths = args
	} else {
		paths = args
	}
	if mode != "json" && *columnsRaw == "" {
		usageError("--columns is required")
	}
	if mode == "json" && len(jsonPaths) == 0 {
		usageError("at least one JSON path is required")
	}

	profiles, err := card.Run(paths, card.Config{
		Mode:      mode,
		Columns:   splitList(*columnsRaw),
		JSONPaths: jsonPaths,
		Delimiter: *delimiter,
		Precision: uint8(*precision),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "card: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		if err := json.NewEncoder(os.Stdout).Encode(profiles); err != nil {
			fmt.Fprintf(os.Stderr, "card: %v\n", err)
			os.Exit(1)
		}
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "field\tapprox_unique\tnulls\tmissing\tempty\ttotal")
	for _, p := range profiles {
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\n", p.Field, p.ApproxUnique, p.Nulls, p.Missing, p.Empty, p.Total)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "card: %v\n", err)
		os.Exit(1)
	}
}

func splitList(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := parts[:0]
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func usageError(msg string) {
	fmt.Fprintf(os.Stderr, "card: %s\n", msg)
	os.Exit(2)
}
