#!/usr/bin/env sh

set -eu
. "${RELEASE_TEST_DIR:-$(dirname "$0")/..}/lib.sh"

seq 1 100 > /tmp/sample-values.txt

sample --rate 1 /tmp/sample-values.txt > /tmp/sample-all.out
cmp /tmp/sample-values.txt /tmp/sample-all.out || fail "sample rate one"
sample --rate 0 /tmp/sample-values.txt > /tmp/sample-none.out
[ ! -s /tmp/sample-none.out ] || fail "sample rate zero"

sample --rate 0.2 --stable --seed 42 /tmp/sample-values.txt > /tmp/sample-a.out
sample --rate 0.2 --stable --seed 42 /tmp/sample-values.txt > /tmp/sample-b.out
cmp /tmp/sample-a.out /tmp/sample-b.out || fail "sample stable"

sample --rate 0.2 --seed 7 /tmp/sample-values.txt > /tmp/sample-random-a.out
sample --rate 0.2 --seed 7 /tmp/sample-values.txt > /tmp/sample-random-b.out
cmp /tmp/sample-random-a.out /tmp/sample-random-b.out || fail "sample seeded random"

[ "$(sample --count 10 --seed 99 /tmp/sample-values.txt | wc -l)" -eq 10 ] || fail "sample count"
[ "$(sample --count 200 --seed 99 /tmp/sample-values.txt | wc -l)" -eq 100 ] || fail "sample count larger than input"

printf "  keep spaces  \nlast-no-newline" > /tmp/sample-preserve.txt
sample --rate 1 /tmp/sample-preserve.txt > /tmp/sample-preserve.out
cmp /tmp/sample-preserve.txt /tmp/sample-preserve.out || fail "sample preserves records exactly"

printf "aa\0bb\0" | sample -0 --rate 1 - > /tmp/sample-nul.out
printf "aa\0bb\0" > /tmp/sample-nul.want
cmp /tmp/sample-nul.out /tmp/sample-nul.want || fail "sample nul records"

printf "1\tgroup-a\tfirst\n2\tgroup-b\tsecond\n" > /tmp/sample-fields.txt
sample --rate 1 --stable -d "$(printf '\t')" -f 2 /tmp/sample-fields.txt > /tmp/sample-fields.out
cmp /tmp/sample-fields.txt /tmp/sample-fields.out || fail "sample stable field preserves records"

expect_status 2 "sample ambiguous flags" \
	sample --rate 0.1 --count 10 /tmp/sample-values.txt
expect_status 2 "sample rejects explicitly set zero count with rate" \
	sample --rate 0.1 --count 0 /tmp/sample-values.txt
expect_status 2 "sample rejects stable count" \
	sample --count 10 --stable /tmp/sample-values.txt
expect_status 2 "sample restricts fields to stable mode" \
	sample --rate 0.5 -d "$(printf '\t')" -f 2 /tmp/sample-fields.txt
expect_status 2 "sample requires mode" sample /tmp/sample-values.txt
expect_status 2 "sample rejects bad rate" sample --rate 1.5 /tmp/sample-values.txt

pass "sample"
