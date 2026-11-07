package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/jo-cube/toolbox/internal/buildinfo"
	"github.com/jo-cube/toolbox/internal/heavy"
	"github.com/jo-cube/toolbox/internal/prob"
)

func main() {
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "print version information")
	flag.BoolVar(&showVersion, "V", false, "print version information")
	top := flag.Int("top", 10, "number of heavy hitters to print")
	capacity := flag.Int("capacity", 0, "tracked item capacity for approximate mode")
	exact := flag.Bool("exact", false, "use exact counts with unbounded memory")
	jsonOut := flag.Bool("json", false, "write JSON output")
	tsvOut := flag.Bool("tsv", false, "write TSV output")
	var input prob.InputOptions
	prob.AddInputFlags(flag.CommandLine, &input)

	flag.Usage = func() {
		name := filepath.Base(os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), `Usage: %s [options] [file...]

Find frequent values in newline-delimited input.
Use --delimiter and --field to rank one field per record.
Default mode uses bounded-memory approximate counts. Use --exact only when all distinct values fit in memory.

Examples:
  awk '{print $7}' access.log | heavy --top 20
  jq -r .tenant_id events.jsonl | heavy --top 50 --json
  heavy --top 20 --exact values.txt

Options:
`, name)
		flag.PrintDefaults()
	}
	flag.Parse()

	if showVersion {
		if len(os.Args) != 2 || (os.Args[1] != "--version" && os.Args[1] != "-V") {
			flag.Usage()
			os.Exit(2)
		}
		if _, err := fmt.Fprintf(os.Stdout, "heavy %s\n", buildinfo.Version()); err != nil {
			fmt.Fprintf(os.Stderr, "heavy: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *jsonOut && *tsvOut {
		fmt.Fprintln(os.Stderr, "heavy: choose only one of --json or --tsv")
		os.Exit(2)
	}
	if *top <= 0 {
		fmt.Fprintln(os.Stderr, "heavy: top must be a positive integer")
		os.Exit(2)
	}
	if *capacity != 0 && *capacity < *top {
		fmt.Fprintln(os.Stderr, "heavy: capacity must be at least top")
		os.Exit(2)
	}

	if err := input.Fields.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "heavy: %v\n", err)
		os.Exit(2)
	}

	results, err := heavy.Run(flag.Args(), heavy.Config{
		Top:      *top,
		Capacity: *capacity,
		Exact:    *exact,
		Input:    input,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "heavy: %v\n", err)
		os.Exit(1)
	}

	if err := writeResults(os.Stdout, results, *jsonOut, *tsvOut); err != nil {
		fmt.Fprintf(os.Stderr, "heavy: %v\n", err)
		os.Exit(1)
	}
}

func writeResults(out io.Writer, results []heavy.Result, jsonOut, tsvOut bool) error {
	if jsonOut {
		return json.NewEncoder(out).Encode(results)
	}

	var table *tabwriter.Writer
	if !tsvOut {
		table = tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		out = table
	}
	if _, err := fmt.Fprintln(out, "rank\tcount_estimate\tcount_lower_bound\titem"); err != nil {
		return err
	}
	for _, result := range results {
		if _, err := fmt.Fprintf(out, "%d\t%d\t%d\t%s\n", result.Rank, result.CountEstimate, result.CountLowerBound, result.Item); err != nil {
			return err
		}
	}
	if table != nil {
		return table.Flush()
	}
	return nil
}
