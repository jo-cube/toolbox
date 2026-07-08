#!/usr/bin/env sh

set -eu
. "${RELEASE_TEST_DIR:-$(dirname "$0")/..}/lib.sh"

seq 1 1000 > /tmp/hll-values.txt
seq 1 1000 >> /tmp/hll-values.txt

hll count /tmp/hll-values.txt | grep -q "approx_unique=" || fail "hll count"
hll count --json /tmp/hll-values.txt |
	jq -e '.approx_unique >= 950 and .approx_unique <= 1050 and .relative_error > 0' >/dev/null ||
	fail "hll count json"

printf " a \n\nb\nb\n" > /tmp/hll-trim.txt
hll count --trim --ignore-empty --json /tmp/hll-trim.txt |
	jq -e '.approx_unique == 2' >/dev/null || fail "hll trim ignore-empty"

printf "a\0b\0a" > /tmp/hll-nul.txt
hll count -0 --json /tmp/hll-nul.txt |
	jq -e '.approx_unique == 2' >/dev/null || fail "hll nul input"

hll build /tmp/hll-values.txt > /tmp/a.hll
seq 501 1500 | hll build > /tmp/b.hll
hll merge /tmp/a.hll /tmp/b.hll > /tmp/merged.hll
hll inspect /tmp/merged.hll | grep -q "type=hyperloglog" || fail "hll inspect"
hll inspect --json /tmp/merged.hll |
	jq -e '.type == "hyperloglog" and .hash == "fnv1a64-avalanche-v1"' >/dev/null ||
	fail "hll inspect json"
hll estimate --json /tmp/a.hll |
	jq -e '.approx_unique >= 950 and .approx_unique <= 1050' >/dev/null ||
	fail "hll estimate json"

hll build --precision 5 /tmp/hll-values.txt > /tmp/p5.hll
expect_status 1 "hll rejects incompatible merge" hll merge /tmp/a.hll /tmp/p5.hll
printf "not an hll file\n" > /tmp/bad.hll
expect_status 1 "hll rejects corrupt state" hll inspect /tmp/bad.hll

expect_status 2 "hll bad precision" hll count --precision 260 /tmp/hll-values.txt
expect_status 2 "hll unknown command" hll nope

pass "hll"
