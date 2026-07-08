#!/usr/bin/env sh

set -eu
. "${RELEASE_TEST_DIR:-$(dirname "$0")/..}/lib.sh"

rm -rf /tmp/rdb
ldb --db=/tmp/rdb --create_if_missing put alpha one >/dev/null
ldb --db=/tmp/rdb put beta two >/dev/null

rdbsh --db /tmp/rdb --exec "get alpha" | grep -qx "one" || fail "rdbsh get"
rdbsh --db /tmp/rdb --exec "get missing" | grep -qx "(not found)" || fail "rdbsh missing key"
rdbsh --db /tmp/rdb --exec "count" | grep -q "2 keys total" || fail "rdbsh count"
rdbsh --db /tmp/rdb --exec "count a" | grep -q "1 keys (prefix: a)" || fail "rdbsh count prefix"
rdbsh --db /tmp/rdb --exec "keys a 1" | grep -q "alpha" || fail "rdbsh keys prefix"
rdbsh --db /tmp/rdb --exec "scan b 1" | grep -Eq "beta[[:space:]]+two" || fail "rdbsh scan prefix"
rdbsh --db /tmp/rdb --exec "export - json" > /tmp/rdb-export.json 2>/tmp/rdb-export.err
jq -e 'length == 2 and any(.[]; .key == "alpha" and .value == "one")' /tmp/rdb-export.json >/dev/null ||
	fail "rdbsh export"
rdbsh --db /tmp/rdb --exec "export - csv a" > /tmp/rdb-export.csv 2>/tmp/rdb-export-csv.err
grep -qx "key,value" /tmp/rdb-export.csv || fail "rdbsh csv export header"
grep -qx "alpha,one" /tmp/rdb-export.csv || fail "rdbsh csv export row"

rdbsh --db /tmp/rdb --writable --exec "put 0x01 0x00" >/tmp/rdb-put.out
rdbsh --db /tmp/rdb --exec "get 0x01" | grep -qx "0x00" || fail "rdbsh hex"
rdbsh --db /tmp/rdb --writable --exec "put \"space key\" \"space value\"" >/tmp/rdb-put-space.out
rdbsh --db /tmp/rdb --exec "get \"space key\"" | grep -qx "space value" || fail "rdbsh quoted args"
rdbsh --db /tmp/rdb --writable --exec "delete beta" >/tmp/rdb-delete.out
rdbsh --db /tmp/rdb --exec "get beta" | grep -qx "(not found)" || fail "rdbsh delete"

rdbsh --db /tmp/rdb --exec "export /tmp/rdb.csv csv" | grep -q "exported" || fail "rdbsh export file"
expect_status 1 "rdbsh refuses export overwrite" \
	rdbsh --db /tmp/rdb --exec "export /tmp/rdb.csv csv"
rdbsh --db /tmp/rdb --force --exec "export /tmp/rdb.csv csv" | grep -q "exported" || fail "rdbsh force export"

expect_status 1 "rdbsh read-only put" \
	rdbsh --db /tmp/rdb --exec "put gamma three"
expect_status 1 "rdbsh rejects invalid hex" \
	rdbsh --db /tmp/rdb --exec "get 0x0g"
expect_status 1 "rdbsh rejects bad command" \
	rdbsh --db /tmp/rdb --exec "definitely-not-a-command"
expect_status 2 "rdbsh requires db" rdbsh --exec "count"

pass "rdbsh"
