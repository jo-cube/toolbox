#!/usr/bin/env sh

set -eu

/usr/bin/time -f "PERF sample_rate_1m elapsed=%e maxrss_kb=%M" \
	sh -c "seq 1 1000000 | sample --rate 0.1 --stable >/dev/null"

/usr/bin/time -f "PERF sample_count_1m elapsed=%e maxrss_kb=%M" \
	sh -c "seq 1 1000000 | sample --count 10000 >/dev/null"
