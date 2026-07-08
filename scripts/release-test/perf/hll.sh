#!/usr/bin/env sh

set -eu

/usr/bin/time -f "PERF hll_count_1m elapsed=%e maxrss_kb=%M" \
	sh -c "seq 1 1000000 | hll count >/dev/null"
