#!/usr/bin/env sh

set -eu
. "${RELEASE_TEST_DIR:-$(dirname "$0")/..}/lib.sh"

printf "alice\nbob\ncarol\n" |
	bf build --expected-items 10 --false-positive-rate 0.01 > /tmp/names.bf

printf "alice\nbob\ncarol\n" | bf test /tmp/names.bf > /tmp/bf-inserted.out
printf "alice\nbob\ncarol\n" > /tmp/bf-inserted.want
cmp /tmp/bf-inserted.out /tmp/bf-inserted.want || fail "bf has no false negatives for inserted values"

printf "alice\ndave\ncarol\n" | bf test /tmp/names.bf > /tmp/bf-hit.out
printf "alice\ncarol\n" > /tmp/bf-hit.want
cmp /tmp/bf-hit.out /tmp/bf-hit.want || fail "bf test"

printf "alice\ndave\ncarol\n" | bf test --invert /tmp/names.bf > /tmp/bf-miss.out
printf "dave\n" > /tmp/bf-miss.want
cmp /tmp/bf-miss.out /tmp/bf-miss.want || fail "bf invert"

bf inspect /tmp/names.bf | grep -q "type=bloom-filter" || fail "bf inspect"
bf inspect --json /tmp/names.bf |
	jq -e '.type == "bloom-filter" and .inserted_items == 3 and .set_bits > 0 and .fill_ratio > 0 and .estimated_false_positive_rate > 0 and .hash == "fnv1a64-avalanche-v1"' >/dev/null ||
	fail "bf inspect json"
cat /tmp/names.bf | bf inspect - | grep -q "type=bloom-filter" || fail "bf inspect stdin"

printf "  alice  \n\nbob\n" |
	bf build --expected-items 10 --false-positive-rate 0.01 --trim --ignore-empty > /tmp/trim.bf
printf "alice\nbob\n" | bf test /tmp/trim.bf > /tmp/bf-trim.out
printf "alice\nbob\n" > /tmp/bf-trim.want
cmp /tmp/bf-trim.out /tmp/bf-trim.want || fail "bf trim ignore-empty"

printf "aa\0bb\0" |
	bf build -0 --expected-items 10 --false-positive-rate 0.01 > /tmp/nul.bf
printf "aa\0cc\0bb\0" | bf test -0 /tmp/nul.bf > /tmp/bf-nul.out
printf "aa\0bb\0" > /tmp/bf-nul.want
cmp /tmp/bf-nul.out /tmp/bf-nul.want || fail "bf nul input"

printf "a\nb\na\n" |
	bf dedupe --expected-items 100 --false-positive-rate 0.000001 > /tmp/bf-dedupe.out
printf "a\nb\n" > /tmp/bf-dedupe.want
cmp /tmp/bf-dedupe.out /tmp/bf-dedupe.want || fail "bf dedupe"

printf "1\talice\tfirst\n2\tbob\tsecond\n" |
	bf build --expected-items 100 --false-positive-rate 0.000001 \
	-d "$(printf '\t')" -f 2 > /tmp/fields.bf
printf "3\talice\tlogin\n4\tcarol\tlogin\n5\tbob\tlogout\n" |
	bf test -d "$(printf '\t')" -f 2 /tmp/fields.bf > /tmp/bf-fields.out
printf "3\talice\tlogin\n5\tbob\tlogout\n" > /tmp/bf-fields.want
cmp /tmp/bf-fields.out /tmp/bf-fields.want || fail "bf field test preserves records"

printf "1\ta\n2\tb\n3\ta\n" |
	bf dedupe --expected-items 100 --false-positive-rate 0.000001 \
	-d "$(printf '\t')" -f 2 > /tmp/bf-field-dedupe.out
printf "1\ta\n2\tb\n" > /tmp/bf-field-dedupe.want
cmp /tmp/bf-field-dedupe.out /tmp/bf-field-dedupe.want || fail "bf field dedupe"

printf "alice\n" | bf build --expected-items 10 --false-positive-rate 0.01 > /tmp/union-a.bf
printf "dave\n" | bf build --expected-items 10 --false-positive-rate 0.01 > /tmp/union-b.bf
cat /tmp/union-b.bf | bf union /tmp/union-a.bf - > /tmp/union.bf
printf "alice\ndave\n" | bf test /tmp/union.bf > /tmp/bf-union.out
printf "alice\ndave\n" > /tmp/bf-union.want
cmp /tmp/bf-union.out /tmp/bf-union.want || fail "bf union"

printf "erin\n" | bf build --expected-items 20 --false-positive-rate 0.01 > /tmp/incompatible.bf
expect_status 1 "bf rejects incompatible union" bf union /tmp/union-a.bf /tmp/incompatible.bf
printf "not a bloom filter\n" > /tmp/bad.bf
expect_status 1 "bf rejects corrupt state" bf inspect /tmp/bad.bf
expect_status 2 "bf rejects bad sizing" bf build --expected-items 0 --false-positive-rate 0.01
expect_status 2 "bf requires field with delimiter" \
	bf build --expected-items 10 --false-positive-rate 0.01 -d ,
expect_status 2 "bf unknown command" bf nope

pass "bf"
