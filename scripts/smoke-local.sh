#!/usr/bin/env sh

set -eu

bin="${TOOLBOX_BIN:-$(pwd)/bin}"

expect_status() {
	want="$1"
	shift
	set +e
	"$@" >/dev/null 2>&1
	got="$?"
	set -e
	if [ "$got" -ne "$want" ]; then
		printf 'FAIL %s exited %s, want %s\n' "$*" "$got" "$want" >&2
		exit 1
	fi
}

for tool in hello ksetoff rdbsh hll bf card heavy sample; do
	expect_status 0 "$bin/$tool" --version
	"$bin/$tool" --version | grep -q "^$tool "
	expect_status 0 "$bin/$tool" -V
	"$bin/$tool" -V | grep -q "^$tool "
	expect_status 0 "$bin/$tool" --help
	"$bin/$tool" --help 2>&1 | grep -q "Usage:"
	expect_status 2 "$bin/$tool" --version extra
	expect_status 2 "$bin/$tool" --version --version
	expect_status 2 "$bin/$tool" --version=true
done

[ "$("$bin/hello")" = "Hello, world!" ]
expect_status 2 "$bin/hello" extra
expect_status 2 "$bin/ksetoff"
expect_status 2 "$bin/rdbsh"
expect_status 2 "$bin/hll" nope
expect_status 2 "$bin/bf" nope
expect_status 2 "$bin/card"
expect_status 2 "$bin/heavy" --top 0
expect_status 2 "$bin/sample"

printf 'LOCAL CLI SMOKE TEST PASSED\n'
