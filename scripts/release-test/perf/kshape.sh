#!/usr/bin/env sh

set -eu

/usr/bin/time -f "PERF kshape_build_1m elapsed=%e maxrss_kb=%M" \
	sh -c "awk 'BEGIN { for (i=0; i<1000000; i++) printf \"events\\t%d\\t%d\\t%d\\t100\\t8\\tkey%05d\\n\", i%16, int(i/16), i, i%10000 }' | kshape build >/dev/null"
