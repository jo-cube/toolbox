#!/usr/bin/env sh

set -eu

script_dir="$(CDPATH= cd "$(dirname "$0")" && pwd)"
version="${VERSION:?set VERSION to the release tag, for example v0.3.0}"
image="${IMAGE:-ubuntu:24.04}"
name="${NAME:-toolbox-release-perf}"

docker run --rm \
	--name "$name" \
	-e VERSION="$version" \
	-v "$script_dir:/release-test:ro" \
	"$image" \
	sh -lc '
set -eu
export DEBIAN_FRONTEND=noninteractive
export RELEASE_TEST_DIR=/release-test
export TOOLBOX_BIN=/tmp/toolbox-bin

apt-get update -qq >/dev/null
apt-get install -y -qq curl ca-certificates time >/dev/null

for tool in hll bf card heavy sample; do
	sh /release-test/install-tool.sh "$tool"
done

export PATH="$TOOLBOX_BIN:$PATH"

for test in /release-test/perf/*.sh; do
	sh "$test"
done
'
