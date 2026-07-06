package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jo-cube/toolbox/internal/buildinfo"
	"github.com/jo-cube/toolbox/internal/heavy"
	"github.com/jo-cube/toolbox/internal/prob"
)

func main() {
	showVersion := flag.Bool("version", false, "print version information")
	top := flag.Int("top", 10, "number of heavy hitters to print")
	capacity := flag.Int("capacity", 0, "tracked item capacity for approximate mode")
	exact := flag.Bool("exact", false, "use exact counts with unbounded memory")
	jsonOut := flag.Bool("json", false, "write JSON output")
	tsvOut := flag.Bool("tsv", false, "write TSV output")
	var input prob.InputOptions
	prob.AddInputFlags(flag.CommandLine, &input)

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options] [file...]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Fprintf(os.Stdout, "heavy %s\n", buildinfo.Version())
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

	if *jsonOut {
		if err := json.NewEncoder(os.Stdout).Encode(results); err != nil {
			fmt.Fprintf(os.Stderr, "heavy: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *tsvOut {
		fmt.Fprintln(os.Stdout, "rank\tcount_estimate\titem")
		for _, result := range results {
			fmt.Fprintf(os.Stdout, "%d\t%d\t%s\n", result.Rank, result.CountEstimate, result.Item)
		}
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "rank\tcount_estimate\titem")
	for _, result := range results {
		fmt.Fprintf(w, "%d\t%d\t%s\n", result.Rank, result.CountEstimate, result.Item)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "heavy: %v\n", err)
		os.Exit(1)
	}
}
