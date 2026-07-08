#!/usr/bin/env sh

set -eu

tool="${1:?usage: install-tool.sh <tool>}"
repo="${GITHUB_REPOSITORY:-jo-cube/toolbox}"
version="${VERSION:?set VERSION to the release tag, for example v0.3.0}"
bin="${TOOLBOX_BIN:-/tmp/toolbox-bin}"
tmp="$(mktemp)"

cleanup() {
	rm -f "$tmp"
}

trap cleanup EXIT INT TERM

mkdir -p "$bin"
curl -fsSL "https://raw.githubusercontent.com/${repo}/${version}/scripts/install.sh" -o "$tmp"
VERSION="$version" INSTALL_DIR="$bin" GITHUB_REPOSITORY="$repo" sh "$tmp" "$tool" "$bin" >/dev/null
"$bin/$tool" --version | grep -q "$tool"
"$bin/$tool" --help 2>&1 | grep -q "Usage:"
printf 'installed %s\n' "$tool"
