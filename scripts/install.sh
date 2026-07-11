#!/usr/bin/env sh

set -eu

REPO="${GITHUB_REPOSITORY:-jo-cube/toolbox}"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/bin}"
VERSION="${VERSION:-latest}"

usage() {
	cat <<EOF
Usage: $0 <cli-name> [destination]

Installs a prebuilt release binary for a CLI from ${REPO}.

Arguments:
  cli-name      Name of the CLI to install, for example: hello
  destination   Optional install directory (default: ${INSTALL_DIR})

Environment:
  VERSION            Release tag to install, for example: v0.1.0 (default: latest)
  INSTALL_DIR        Destination directory (default: ~/.local/bin)
  GITHUB_REPOSITORY  GitHub repo in owner/name form (default: jo-cube/toolbox)
EOF
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
	usage
	exit 0
fi

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
	usage >&2
	exit 1
fi

CLI_NAME="$1"
if [ "$#" -eq 2 ]; then
	INSTALL_DIR="$2"
fi

case "$CLI_NAME" in
	hello|ksetoff|rdbsh|hll|bf|card|heavy|sample)
		;;
	*)
		printf 'error: unsupported CLI: %s\n' "$CLI_NAME" >&2
		exit 1
		;;
esac

case "$REPO" in
	*/*/*|/*|*/|*'..'*|*[!A-Za-z0-9._/-]*)
		printf 'error: invalid GITHUB_REPOSITORY: %s\n' "$REPO" >&2
		exit 1
		;;
	*/*)
		;;
	*)
		printf 'error: invalid GITHUB_REPOSITORY: %s\n' "$REPO" >&2
		exit 1
		;;
esac

case "$VERSION" in
	*[!A-Za-z0-9._-]*)
		printf 'error: invalid VERSION: %s\n' "$VERSION" >&2
		exit 1
		;;
	*)
		;;
esac

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
	Darwin)
		GOOS="darwin"
		;;
	Linux)
		GOOS="linux"
		;;
	*)
		printf 'error: unsupported operating system: %s\n' "$OS" >&2
		exit 1
		;;
esac

case "$ARCH" in
	arm64|aarch64)
		GOARCH="arm64"
		;;
	x86_64)
		GOARCH="amd64"
		;;
	*)
		printf 'error: unsupported architecture: %s\n' "$ARCH" >&2
		exit 1
		;;
esac

ASSET_NAME="${CLI_NAME}_${GOOS}_${GOARCH}.tar.gz"
TMP_DIR="$(mktemp -d)"
ARCHIVE_PATH="${TMP_DIR}/${ASSET_NAME}"

cleanup() {
	rm -rf "$TMP_DIR"
}

trap cleanup EXIT INT TERM

if command -v curl >/dev/null 2>&1; then
	DOWNLOAD_TOOL='curl'
elif command -v wget >/dev/null 2>&1; then
	DOWNLOAD_TOOL='wget'
else
	printf 'error: curl or wget is required to download release assets\n' >&2
	exit 1
fi

case "$VERSION" in
	latest)
		DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET_NAME}"
		;;
	*)
		DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET_NAME}"
		;;
esac
CHECKSUM_URL="${DOWNLOAD_URL}.sha256"
CHECKSUM_PATH="${ARCHIVE_PATH}.sha256"

printf 'Downloading %s from %s\n' "$ASSET_NAME" "$DOWNLOAD_URL"
case "$DOWNLOAD_TOOL" in
	curl)
		curl -fsSL "$DOWNLOAD_URL" -o "$ARCHIVE_PATH"
		curl -fsSL "$CHECKSUM_URL" -o "$CHECKSUM_PATH"
		;;
	wget)
		wget -qO "$ARCHIVE_PATH" "$DOWNLOAD_URL"
		wget -qO "$CHECKSUM_PATH" "$CHECKSUM_URL"
		;;
esac

printf 'Verifying checksum\n'
EXPECTED_SHA="$(awk '{print $1}' "$CHECKSUM_PATH")"
if command -v sha256sum >/dev/null 2>&1; then
	ACTUAL_SHA="$(sha256sum "$ARCHIVE_PATH" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
	ACTUAL_SHA="$(shasum -a 256 "$ARCHIVE_PATH" | awk '{print $1}')"
else
	printf 'error: sha256sum or shasum is required to verify release assets\n' >&2
	exit 1
fi
if [ "$EXPECTED_SHA" != "$ACTUAL_SHA" ]; then
	printf 'error: checksum mismatch for %s\n' "$ASSET_NAME" >&2
	exit 1
fi

if [ "$(tar -tzf "$ARCHIVE_PATH")" != "$CLI_NAME" ]; then
	printf 'error: release archive must contain only %s\n' "$CLI_NAME" >&2
	exit 1
fi
tar -xzf "$ARCHIVE_PATH" -C "$TMP_DIR"

if [ "$CLI_NAME" = "rdbsh" ]; then
	RUNTIME_ERROR="${TMP_DIR}/rdbsh-runtime-error"
	if ! "${TMP_DIR}/rdbsh" --version >/dev/null 2>"$RUNTIME_ERROR"; then
		printf 'error: rdbsh requires a compatible RocksDB runtime library\n' >&2
		case "$GOOS" in
		linux)
			printf 'On Ubuntu or Debian, install it with: sudo apt-get install librocksdb-dev\n' >&2
			;;
		darwin)
			printf 'On macOS, install it with: brew install rocksdb\n' >&2
			;;
		esac
		printf 'Loader error:\n' >&2
		cat "$RUNTIME_ERROR" >&2
		exit 1
	fi
fi

mkdir -p "$INSTALL_DIR"
install -m 0755 "${TMP_DIR}/${CLI_NAME}" "${INSTALL_DIR}/${CLI_NAME}"

printf 'Installed %s to %s\n' "$CLI_NAME" "${INSTALL_DIR}/${CLI_NAME}"

case ":$PATH:" in
	*":${INSTALL_DIR}:"*)
		;;
	*)
		printf 'Add %s to your PATH if it is not already available.\n' "$INSTALL_DIR"
		;;
esac
