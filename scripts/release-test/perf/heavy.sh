#!/usr/bin/env sh

set -eu

/usr/bin/time -f "PERF heavy_500k elapsed=%e maxrss_kb=%M" \
	sh -c "seq 1 500000 | awk '{print \$1 % 1000}' | heavy --top 20 >/dev/null"
