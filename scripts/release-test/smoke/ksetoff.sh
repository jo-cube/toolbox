#!/usr/bin/env sh

set -eu
. "${RELEASE_TEST_DIR:-$(dirname "$0")/..}/lib.sh"

ksetoff --help >/tmp/ksetoff-help.out 2>&1
grep -q "timestamp:<ISO-8601>" /tmp/ksetoff-help.out || fail "ksetoff help"

expect_status 1 "ksetoff missing config" \
	ksetoff -F /missing.conf -group g -topic t -offset latest -dry-run
printf "security.protocol=INVALID\n" > /tmp/ksetoff-bad.conf
expect_status 1 "ksetoff rejects invalid config" \
	ksetoff -F /tmp/ksetoff-bad.conf -group g -topic t -offset latest -dry-run
printf "bootstrap.servers=localhost:1\n" > /tmp/ksetoff-local.conf
expect_status 2 "ksetoff rejects bad offset" \
	ksetoff -F /tmp/ksetoff-local.conf -group g -topic t -offset wat -dry-run
expect_status 2 "ksetoff rejects bad partition list" \
	ksetoff -F /tmp/ksetoff-local.conf -group g -topic t -offset latest -partitions a -dry-run

pass "ksetoff local validation"
