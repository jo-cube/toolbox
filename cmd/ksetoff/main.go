package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jo-cube/toolbox/internal/buildinfo"
	"github.com/jo-cube/toolbox/internal/ksetoff"
)

func main() {
	showVersion := flag.Bool("version", false, "print version information")
	configFile := flag.String("F", "", "path to kcat-style config file (librdkafka key=value format) [required]")
	groupID := flag.String("group", "", "consumer group ID [required]")
	topic := flag.String("topic", "", "kafka topic [required]")
	offsetRaw := flag.String("offset", "", "offset spec: a number, 'earliest', 'latest', or 'timestamp:<ISO-8601>' [required]")
	partitionsRaw := flag.String("partitions", "", "comma-separated partition numbers (default: all partitions)")
	dryRun := flag.Bool("dry-run", false, "show planned offset changes without committing")
	timeout := flag.Duration("timeout", 30*time.Second, "overall operation timeout")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `Usage: %s -F <config> -group <group-id> -topic <topic> -offset <spec> [options]

Set Kafka consumer group offsets without starting the consumer application.

Examples:
  ksetoff -F kafka.conf -group my-cg -topic events -offset latest -dry-run
  ksetoff -F kafka.conf -group my-cg -topic events -offset earliest
  ksetoff -F kafka.conf -group my-cg -topic events -offset timestamp:2026-04-13T00:00:00Z

Offset specs:
  <number>                  Exact numeric offset, such as 42.
  earliest                  Beginning of each partition.
  latest                    End of each partition.
  timestamp:<ISO-8601>      First offset at or after a timestamp.

Notes:
  Run with -dry-run first. The plan is written to stdout; warnings go to stderr.
  The config file uses kcat/librdkafka key=value settings such as bootstrap.servers.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	flag.Parse()

	if *showVersion {
		fmt.Fprintf(os.Stdout, "ksetoff %s\n", buildinfo.Version())
		return
	}

	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}

	var missing []string
	if *configFile == "" {
		missing = append(missing, "-F")
	}
	if *groupID == "" {
		missing = append(missing, "-group")
	}
	if *topic == "" {
		missing = append(missing, "-topic")
	}
	if *offsetRaw == "" {
		missing = append(missing, "-offset")
	}
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "ksetoff: missing required flag(s): %s\n\n", strings.Join(missing, ", "))
		flag.Usage()
		os.Exit(2)
	}

	spec, err := ksetoff.ParseOffsetSpec(*offsetRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ksetoff: %v\n", err)
		os.Exit(2)
	}

	requestedPartitions, err := ksetoff.ParsePartitions(*partitionsRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ksetoff: %v\n", err)
		os.Exit(2)
	}

	kafkaConfig, err := ksetoff.ParseConfigFile(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ksetoff: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	admin, cleanup, err := ksetoff.NewAdminClient(ctx, kafkaConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ksetoff: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()
	defer admin.Close()

	err = ksetoff.Run(ctx, admin, ksetoff.RunParams{
		GroupID:             *groupID,
		Topic:               *topic,
		Spec:                spec,
		RequestedPartitions: requestedPartitions,
		DryRun:              *dryRun,
		Out:                 os.Stdout,
		ErrOut:              os.Stderr,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ksetoff: %v\n", err)
		os.Exit(1)
	}
}
