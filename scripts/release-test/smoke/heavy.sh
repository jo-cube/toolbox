#!/usr/bin/env sh

set -eu
. "${RELEASE_TEST_DIR:-$(dirname "$0")/..}/lib.sh"

cat >/tmp/heavy-values.txt <<EOF
a
a
a
b
b
c
EOF

heavy --top 2 --exact /tmp/heavy-values.txt |
	grep -Eq "1[[:space:]]+3[[:space:]]+a" || fail "heavy exact"

heavy --top 2 --json /tmp/heavy-values.txt |
	jq -e "length == 2 and .[0].rank == 1" >/dev/null || fail "heavy json"

heavy --top 2 --tsv --exact /tmp/heavy-values.txt > /tmp/heavy.tsv
grep -q "rank	count_estimate	item" /tmp/heavy.tsv || fail "heavy tsv header"
grep -q "1	3	a" /tmp/heavy.tsv || fail "heavy tsv rows"

printf " a \n\nb\nb\n" > /tmp/heavy-trim.txt
heavy --top 2 --exact --trim --ignore-empty /tmp/heavy-trim.txt |
	grep -Eq "1[[:space:]]+2[[:space:]]+b" || fail "heavy trim ignore-empty"

printf "aa\0bb\0aa" > /tmp/heavy-nul.txt
heavy -0 --top 1 --exact /tmp/heavy-nul.txt |
	grep -Eq "1[[:space:]]+2[[:space:]]+aa" || fail "heavy nul input"

expect_status 2 "heavy rejects json tsv combo" heavy --json --tsv /tmp/heavy-values.txt
expect_status 2 "heavy rejects bad top" heavy --top 0 /tmp/heavy-values.txt
expect_status 2 "heavy rejects small capacity" heavy --top 10 --capacity 5 /tmp/heavy-values.txt

pass "heavy"
