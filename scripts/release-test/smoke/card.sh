#!/usr/bin/env sh

set -eu
. "${RELEASE_TEST_DIR:-$(dirname "$0")/..}/lib.sh"

cat >/tmp/users.csv <<EOF
user_id,country,plan
u1,US,free
u2,US,pro
u3,IN,free
u1,,free
EOF

card --csv --columns user_id,country,plan /tmp/users.csv |
	grep -q "approx_unique" || fail "card csv"
card --output-json --csv --columns user_id,country,plan /tmp/users.csv |
	jq -e '
		length == 3 and
		(.[] | select(.field == "country") | .empty == 1 and .total == 4)
	' >/dev/null || fail "card csv json"

cat >/tmp/fields.tsv <<EOF
u1	US	free
u2	US	pro
u3	IN	free
u4
EOF
card --delimiter "$(printf '\t')" --columns 1,2,3 /tmp/fields.tsv |
	grep -q "field" || fail "card delimiter"
card --output-json --delimiter "$(printf '\t')" --columns 1,2,3 /tmp/fields.tsv |
	jq -e '
		length == 3 and
		(.[] | select(.field == "3") | .missing == 1 and .total == 4)
	' >/dev/null || fail "card delimiter json"

cat >/tmp/events.jsonl <<EOF
{"user_id":"u1","event_type":"login","metadata":{"country":"US"}}
{"user_id":"u2","event_type":"buy","metadata":{"country":"US"}}
{"user_id":"u1","event_type":"login","metadata":{}}
{"user_id":null,"event_type":"logout","metadata":{"country":"IN"}}
{"user_id":42,"event_type":"","metadata":{"country":null}}
EOF

card --json .user_id .event_type .metadata.country /tmp/events.jsonl |
	grep -q ".metadata.country" || fail "card json"
card --output-json --json .user_id .event_type .metadata.country /tmp/events.jsonl |
	jq -e '
		length == 3 and
		(.[] | select(.field == ".user_id") | .nulls == 1 and .total == 5) and
		(.[] | select(.field == ".event_type") | .empty == 1 and .total == 5) and
		(.[] | select(.field == ".metadata.country") | .missing == 1 and .nulls == 1 and .total == 5)
	' >/dev/null || fail "card json counters"

perl -e 'print "{\"id\":\"" . ("x" x 1200000) . "\"}\n"' > /tmp/card-long.jsonl
card --json .id /tmp/card-long.jsonl | grep -q ".id" || fail "card long line"

printf '{"user_id":' > /tmp/card-bad.jsonl
expect_status 1 "card rejects malformed json" card --json .user_id /tmp/card-bad.jsonl
expect_status 1 "card rejects missing csv column" card --csv --columns missing /tmp/users.csv
expect_status 2 "card rejects multiple modes" card --csv --json .user_id --columns user_id /tmp/users.csv
expect_status 2 "card rejects bad precision" card --csv --columns user_id --precision 260 /tmp/users.csv

pass "card"
