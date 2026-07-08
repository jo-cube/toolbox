#!/usr/bin/env sh

set -eu

/usr/bin/time -f "PERF bf_build_500k elapsed=%e maxrss_kb=%M" \
	sh -c "seq 1 500000 | bf build --expected-items 500000 --false-positive-rate 0.01 >/tmp/perf.bf"

/usr/bin/time -f "PERF bf_test_500k elapsed=%e maxrss_kb=%M" \
	sh -c "seq 250000 750000 | bf test /tmp/perf.bf >/dev/null"
