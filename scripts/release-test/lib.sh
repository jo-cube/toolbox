#!/usr/bin/env sh

fail() {
	printf 'FAIL %s\n' "$1" >&2
	exit 1
}

pass() {
	printf 'PASS %s\n' "$1"
}

expect_status() {
	want="$1"
	label="$2"
	shift 2
	out="${TMPDIR:-/tmp}/toolbox-release-test-$$.out"
	err="${TMPDIR:-/tmp}/toolbox-release-test-$$.err"

	set +e
	"$@" >"$out" 2>"$err"
	status="$?"
	set -e

	if [ "$status" -ne "$want" ]; then
		cat "$out"
		cat "$err" >&2
		fail "$label: got status $status, want $want"
	fi

	rm -f "$out" "$err"
}
