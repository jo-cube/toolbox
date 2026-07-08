#!/usr/bin/env sh

set -eu
. "${RELEASE_TEST_DIR:-$(dirname "$0")/..}/lib.sh"

[ "$(hello)" = "Hello, world!" ] || fail "hello output"
expect_status 2 "hello rejects args" hello extra
hello --version | grep -q "^hello " || fail "hello version"

pass "hello"
