package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jo-cube/toolbox/internal/buildinfo"
	"github.com/jo-cube/toolbox/internal/hll"
	"github.com/jo-cube/toolbox/internal/kshape"
)

var errHelp = errors.New("help")

type usageError string

func (e usageError) Error() string { return string(e) }

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-V") {
		fmt.Fprintf(out, "kshape %s\n", buildinfo.Version())
		return 0
	}
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		usage(errOut)
		return 0
	}
	if len(args) == 0 {
		usage(errOut)
		return 2
	}

	var err error
	switch args[0] {
	case "format":
		err = format(args[1:], out)
	case "build":
		err = build(args[1:], in, out, errOut)
	case "inspect":
		err = inspect(args[1:], out, errOut)
	case "show":
		err = show(args[1:], out, errOut)
	case "render":
		err = render(args[1:], out, errOut)
	case "merge":
		err = merge(args[1:], out, errOut)
	default:
		usage(errOut)
		return 2
	}
	if errors.Is(err, errHelp) {
		return 0
	}
	var usageErr usageError
	if errors.As(err, &usageErr) {
		if usageErr != "" {
			fmt.Fprintf(errOut, "kshape: %s\n", usageErr)
		}
		return 2
	}
	if err != nil {
		fmt.Fprintf(errOut, "kshape: %v\n", err)
		return 1
	}
	return 0
}

func usage(out io.Writer) {
	fmt.Fprint(out, `Usage: kshape <command> [options]

Build and read mergeable summaries of observed Kafka record streams.

Commands:
  format   Print the canonical jkq/kcat -f expression.
  build    Read canonical records and write a binary .kshape artifact.
  show     Print a concise terminal view of the stream shape.
  inspect  Print detailed artifact metrics for diagnostics or scripts.
  render   Write a self-contained offline HTML report.
  merge    Merge compatible non-overlapping artifacts.

Global options:
  -V, --version  Print version information.

Example:
  jkq ... -f "$(kshape format)" | kshape build > topic.kshape
  kshape show topic.kshape
  kshape render topic.kshape > topic.html
`)
}

func format(args []string, out io.Writer) error {
	if len(args) != 0 {
		return usageError("usage: kshape format")
	}
	_, err := io.WriteString(out, kshape.CanonicalFormat+"\n")
	return err
}

func build(args []string, in io.Reader, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("kshape build", flag.ContinueOnError)
	fs.SetOutput(errOut)
	bucketWidth := fs.Uint64("bucket-width", kshape.DefaultBucketWidth, "finest offset bucket width (power of two)")
	precision := fs.Uint("precision", uint(kshape.DefaultPrecision), "per-bucket HLL precision, 4..20")
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: kshape build [options] > topic.kshape

Read canonical records from stdin and write a binary summary to stdout.
Partitions may interleave; offsets must increase within each partition.

Options:
`)
		fs.PrintDefaults()
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return usageError("usage: kshape build [options]")
	}
	p, err := hll.Precision(*precision)
	if err != nil {
		return usageError(err.Error())
	}
	if _, err := kshape.New(*bucketWidth, p); err != nil {
		return usageError(err.Error())
	}
	summary, err := kshape.Build(in, *bucketWidth, p)
	if err != nil {
		return err
	}
	return kshape.Write(out, summary)
}

func inspect(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("kshape inspect", flag.ContinueOnError)
	fs.SetOutput(errOut)
	jsonOut := fs.Bool("json", false, "write JSON output")
	bucketWidth := fs.Uint64("bucket-width", 0, "report bucket width; zero summarizes whole partitions")
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: kshape inspect [--json] [--bucket-width n] <file.kshape>

Print observed record metrics. A non-zero report width must be a power-of-two
multiple of the artifact's finest bucket width.

Options:
`)
		fs.PrintDefaults()
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return usageError("usage: kshape inspect [options] <file>")
	}
	summary, err := readSummary(fs.Arg(0))
	if err != nil {
		return err
	}
	report, err := summary.Report(*bucketWidth)
	if err != nil {
		return usageError(err.Error())
	}
	if *jsonOut {
		return json.NewEncoder(out).Encode(report)
	}
	return writeReport(out, report)
}

func show(args []string, out, errOut io.Writer) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprint(errOut, `Usage: kshape show <file.kshape>

Print a concise plain-ASCII view of density across each partition's observed
offset span. Output is identical on terminals and when redirected.
`)
		return errHelp
	}
	if len(args) != 1 {
		return usageError("usage: kshape show <file.kshape>")
	}
	summary, err := readSummary(args[0])
	if err != nil {
		return err
	}
	return kshape.Show(out, summary)
}

func render(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("kshape render", flag.ContinueOnError)
	fs.SetOutput(errOut)
	title := fs.String("title", "", "report title; defaults to the topic identity")
	metric := fs.String("metric", "density", "initial view: density, churn, tombstones, or payload")
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: kshape render [options] <file.kshape> > report.html

Write a self-contained offline HTML report to stdout.

Options:
`)
		fs.PrintDefaults()
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return usageError("usage: kshape render [options] <file>")
	}
	if !kshape.ValidRenderMetric(*metric) {
		return usageError("render metric must be density, churn, tombstones, or payload")
	}
	summary, err := readSummary(fs.Arg(0))
	if err != nil {
		return err
	}
	return kshape.Render(out, summary, *title, *metric)
}

func merge(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("kshape merge", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: kshape merge <file.kshape> <file.kshape>... > merged.kshape

Merge artifacts with matching topic, bucket width, precision, version, and hash.
Observed offset coverage within the same finest bucket must not overlap.
`)
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return usageError("usage: kshape merge <file> <file>...")
	}
	merged, err := readSummary(fs.Arg(0))
	if err != nil {
		return err
	}
	for _, path := range fs.Args()[1:] {
		next, err := readSummary(path)
		if err != nil {
			return err
		}
		if err := merged.Merge(next); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return kshape.Write(out, merged)
}

func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return errHelp
		}
		return usageError("")
	}
	return nil
}

func readSummary(path string) (*kshape.Summary, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	summary, err := kshape.Read(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return summary, nil
}

func writeReport(out io.Writer, report kshape.Report) error {
	if _, err := fmt.Fprintf(out, "type=%s\nversion=%d\ntopic=%s\nartifact_bucket_width=%d\nreport_bucket_width=%d\nhll_precision=%d\nhll_version=%d\nhash=%s\nchecksum=%s\nhll_relative_error=%.2f%%\n",
		report.Type, report.Version, report.Topic, report.ArtifactBucketWidth, report.ReportBucketWidth,
		report.HLLPrecision, report.HLLVersion, report.Hash, report.Checksum, report.HLLRelativeError*100); err != nil {
		return err
	}
	for _, partition := range report.Partitions {
		for _, region := range partition.Regions {
			minTimestamp, maxTimestamp := "missing", "missing"
			if region.MinTimestamp != nil {
				minTimestamp = fmt.Sprint(*region.MinTimestamp)
				maxTimestamp = fmt.Sprint(*region.MaxTimestamp)
			}
			if _, err := fmt.Fprintf(out, "partition=%d region=%d..%d observed_offsets=%d..%d observed_span=%d observed_records=%d observed_occupancy=%.6f logical_payload_bytes=%d observed_tombstones=%d null_keys=%d keyed_records=%d missing_timestamps=%d min_timestamp=%s max_timestamp=%s approx_distinct_keys=%d approx_keyed_records_per_distinct_key=%.3f\n",
				partition.Partition, region.RegionFirstOffset, region.RegionLastOffset,
				region.ObservedFirstOffset, region.ObservedLastOffset, region.ObservedSpan,
				region.ObservedRecords, region.ObservedOccupancy, region.LogicalPayloadBytes,
				region.ObservedTombstones, region.NullKeys, region.KeyedRecords, region.MissingTimestamps,
				minTimestamp, maxTimestamp, region.ApproxDistinctKeys, region.ApproxRecordsPerKey); err != nil {
				return err
			}
		}
	}
	return nil
}
