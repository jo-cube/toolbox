package main

import (
	"context"
	"flag"
	"fmt"
	"os"
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
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s -F <config> -group <group-id> -topic <topic> -offset <spec> [options]\n\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Bootstrap a Kafka consumer group to specific offsets.")
		fmt.Fprintln(flag.CommandLine.Output())
		fmt.Fprintln(flag.CommandLine.Output(), "Offset spec:")
		fmt.Fprintln(flag.CommandLine.Output(), "  <number>                  Exact numeric offset (e.g. 42)")
		fmt.Fprintln(flag.CommandLine.Output(), "  earliest                  Beginning of each partition")
		fmt.Fprintln(flag.CommandLine.Output(), "  latest                    End of each partition (high watermark)")
		fmt.Fprintln(flag.CommandLine.Output(), "  timestamp:<ISO-8601>      Offset at a specific time (e.g. timestamp:2026-04-13T00:00:00Z)")
		fmt.Fprintln(flag.CommandLine.Output())
		fmt.Fprintln(flag.CommandLine.Output(), "Config file format (kcat/librdkafka key=value):")
		fmt.Fprintln(flag.CommandLine.Output(), "  bootstrap.servers=broker1:9092,broker2:9092")
		fmt.Fprintln(flag.CommandLine.Output(), "  security.protocol=SASL_SSL")
		fmt.Fprintln(flag.CommandLine.Output(), "  sasl.mechanism=PLAIN")
		fmt.Fprintln(flag.CommandLine.Output(), "  sasl.username=my-user")
		fmt.Fprintln(flag.CommandLine.Output(), "  sasl.password=my-password")
		fmt.Fprintln(flag.CommandLine.Output())
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
		os.Exit(1)
	}

	requestedPartitions, err := ksetoff.ParsePartitions(*partitionsRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ksetoff: %v\n", err)
		os.Exit(1)
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
