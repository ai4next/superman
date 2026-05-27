#!/bin/sh
set -eu

REPO="${REPO:-ai4next/superman}"
BIN_NAME="${BIN_NAME:-sm}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
VERSION="${VERSION:-latest}"

usage() {
  cat <<EOF
Install Superman CLI.

Usage:
  curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | sh

Environment:
  VERSION=v0.0.1              Install a specific version. Default: latest
  INSTALL_DIR=\$HOME/.local/bin Install location. Default: /usr/local/bin
  REPO=owner/repo             GitHub repository. Default: ${REPO}
EOF
}

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

detect_os() {
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    darwin|linux) printf '%s' "$os" ;;
    *) echo "error: unsupported OS: $os" >&2; exit 1 ;;
  esac
}

detect_arch() {
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) printf 'amd64' ;;
    arm64|aarch64) printf 'arm64' ;;
    *) echo "error: unsupported architecture: $arch" >&2; exit 1 ;;
  esac
}

latest_version() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" |
      sed 's#.*/tag/##'
    return
  fi

  if command -v wget >/dev/null 2>&1; then
    wget -qO- "https://github.com/${REPO}/releases/latest" 2>/dev/null |
      sed -n 's/.*href="\/'"$(printf '%s' "$REPO" | sed 's/[.[\*^$()+?{}|]/\\&/g')"'\/releases\/tag\/\([^"]*\)".*/\1/p' |
      head -n 1
    return
  fi

  echo "error: curl or wget is required" >&2
  exit 1
}

download() {
  url="$1"
  out="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -fL "$url" -o "$out"
    return
  fi

  wget -O "$out" "$url"
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  usage
  exit 0
fi

need uname
need chmod
need mktemp

if [ "$VERSION" = "latest" ]; then
  VERSION="$(latest_version)"
fi

if [ -z "$VERSION" ]; then
  echo "error: could not resolve latest release version" >&2
  exit 1
fi

OS="$(detect_os)"
ARCH="$(detect_arch)"
ASSET="sm-${VERSION}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
TMP_DIR="$(mktemp -d)"
TMP_BIN="${TMP_DIR}/${BIN_NAME}"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

echo "Downloading ${URL}"
download "$URL" "$TMP_BIN"
chmod +x "$TMP_BIN"

if [ ! -d "$INSTALL_DIR" ]; then
  if ! mkdir -p "$INSTALL_DIR" 2>/dev/null; then
    echo "Creating ${INSTALL_DIR} with sudo"
    sudo mkdir -p "$INSTALL_DIR"
  fi
fi

TARGET="${INSTALL_DIR}/${BIN_NAME}"
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_BIN" "$TARGET"
else
  echo "Installing to ${TARGET} with sudo"
  sudo mv "$TMP_BIN" "$TARGET"
fi

echo "Installed ${BIN_NAME} ${VERSION} to ${TARGET}"
echo "Run: ${BIN_NAME} --help"
