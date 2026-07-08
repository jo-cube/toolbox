#!/usr/bin/env sh

set -eu

/usr/bin/time -f "PERF card_csv_200k elapsed=%e maxrss_kb=%M" \
	sh -c "awk 'BEGIN{print \"user_id,country,plan\"; for (i=1; i<=200000; i++) print \"u\" i \",US,p\" (i%5)}' | card --csv --columns user_id,country,plan >/dev/null"
