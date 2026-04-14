#!/usr/bin/env sh

set -eu

REPO="${GITHUB_REPOSITORY:-jo-cube/toolbox}"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/bin}"
VERSION="${VERSION:-latest}"

usage() {
	cat <<EOF
Usage: $0 <cli-name> [destination]

Installs a prebuilt macOS release binary for a CLI from ${REPO}.

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

OS="$(uname -s)"
ARCH="$(uname -m)"

if [ "$OS" != "Darwin" ]; then
	printf 'error: this installer currently supports macOS only (detected %s)\n' "$OS" >&2
	exit 1
fi

case "$ARCH" in
	arm64|aarch64)
		GOARCH="arm64"
		;;
	x86_64)
		GOARCH="amd64"
		;;
	*)
		printf 'error: unsupported macOS architecture: %s\n' "$ARCH" >&2
		exit 1
		;;
esac

ASSET_NAME="${CLI_NAME}_darwin_${GOARCH}.tar.gz"
TMP_DIR="$(mktemp -d)"
ARCHIVE_PATH="${TMP_DIR}/${ASSET_NAME}"

cleanup() {
	rm -rf "$TMP_DIR"
}

trap cleanup EXIT INT TERM

if command -v curl >/dev/null 2>&1; then
	DOWNLOAD_CMD='curl -fsSL'
elif command -v wget >/dev/null 2>&1; then
	DOWNLOAD_CMD='wget -qO-'
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

printf 'Downloading %s from %s\n' "$ASSET_NAME" "$DOWNLOAD_URL"
sh -c "$DOWNLOAD_CMD \"$DOWNLOAD_URL\" > \"$ARCHIVE_PATH\""

mkdir -p "$INSTALL_DIR"
tar -xzf "$ARCHIVE_PATH" -C "$TMP_DIR"
install -m 0755 "${TMP_DIR}/${CLI_NAME}" "${INSTALL_DIR}/${CLI_NAME}"

printf 'Installed %s to %s\n' "$CLI_NAME" "${INSTALL_DIR}/${CLI_NAME}"

case ":$PATH:" in
	*":${INSTALL_DIR}:"*)
		;;
	*)
		printf 'Add %s to your PATH if it is not already available.\n' "$INSTALL_DIR"
		;;
esac
