#!/usr/bin/env sh

set -eu
. "${RELEASE_TEST_DIR:-$(dirname "$0")/..}/lib.sh"

[ "$(kshape format)" = '%t\t%p\t%o\t%T\t%S\t%K\t%k\n' ] ||
	fail "kshape format"

printf 'events\t0\t0\t100\t3\t1\ta\nevents\t0\t3\t-1\t-1\t1\ta\n' |
	kshape build --bucket-width 4 --precision 8 > /tmp/a.kshape
printf 'events\t1\t5\t200\t0\t-1\t\n' |
	kshape build --bucket-width 4 --precision 8 > /tmp/b.kshape

kshape merge /tmp/a.kshape /tmp/b.kshape > /tmp/merged.kshape
cat /tmp/merged.kshape | kshape inspect - | grep -q 'type=kafka-retained-log-shape' ||
	fail "kshape inspect"
kshape inspect --json --bucket-width 8 /tmp/merged.kshape |
	jq -e '.topic == "events" and (.partitions | length) == 2 and .partitions[0].regions[0].observed_tombstones == 1' >/dev/null ||
	fail "kshape inspect json"
kshape show /tmp/merged.kshape | grep -q '^offset density$' ||
	fail "kshape show"
kshape render /tmp/merged.kshape >/tmp/merged.html
grep -q '<!doctype html>' /tmp/merged.html ||
	fail "kshape render"

printf 'not a kshape file\n' > /tmp/bad.kshape
expect_status 1 "kshape rejects corrupt state" kshape inspect /tmp/bad.kshape
expect_status 1 "kshape rejects malformed input" sh -c "printf bad | kshape build >/dev/null"
expect_status 2 "kshape rejects bad width" kshape build --bucket-width 3
expect_status 2 "kshape unknown command" kshape nope

pass "kshape"
