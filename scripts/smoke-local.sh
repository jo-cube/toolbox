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

for tool in hello ksetoff kshape rdbsh hll bf card heavy sample; do
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
expect_status 2 "$bin/kshape" nope
expect_status 2 "$bin/rdbsh"
expect_status 2 "$bin/hll" nope
expect_status 2 "$bin/bf" nope
expect_status 2 "$bin/card"
expect_status 2 "$bin/heavy" --top 0
expect_status 2 "$bin/sample"

[ "$(printf '1::a\n2::b\n3::a\n' | "$bin/hll" count -d :: -f 2 | head -n 1)" = "approx_unique=2" ]
[ "$(printf '1::a\n2::b\n3::a\n' | "$bin/hll" build -d :: -f 2 | "$bin/hll" estimate - | head -n 1)" = "approx_unique=2" ]
printf '1::a\n2::b\n3::a\n' | "$bin/heavy" --exact --top 1 -d :: -f 2 --tsv | grep -q '1[[:space:]]2[[:space:]]2[[:space:]]a'
[ "$(printf 'a\nb\n' | "$bin/sample" --rate 0 --invert)" = "$(printf 'a\nb')" ]
expect_status 2 "$bin/hll" count --field 2
expect_status 2 "$bin/hll" build --delimiter ::
expect_status 2 "$bin/heavy" --field 2
expect_status 2 "$bin/sample" --count 1 --invert

printf 'LOCAL CLI SMOKE TEST PASSED\n'
