#!/bin/sh
set -eu

REPO="${TRIMIA_REPO:-romerramos/trimia}"
VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY="trimia"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "trimia installer: missing required command: $1" >&2
    exit 1
  fi
}

need curl
need tar
need grep
need cut
need mktemp

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
  Linux) os="Linux" ;;
  Darwin) os="Darwin" ;;
  *)
    echo "trimia installer: unsupported OS: $os" >&2
    exit 1
    ;;
esac

case "$arch" in
  x86_64|amd64) arch="x86_64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "trimia installer: unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

if [ "$VERSION" = "latest" ]; then
  need sed
  VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)"
  if [ -z "$VERSION" ]; then
    echo "trimia installer: could not resolve latest release" >&2
    exit 1
  fi
fi

asset="trimia_${os}_${arch}.tar.gz"
base_url="https://github.com/$REPO/releases/download/$VERSION"
tmpdir="$(mktemp -d)"

cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT INT TERM

echo "Installing trimia $VERSION for $os/$arch"

curl -fsSL "$base_url/$asset" -o "$tmpdir/$asset"
curl -fsSL "$base_url/checksums.txt" -o "$tmpdir/checksums.txt"

expected="$(grep "  $asset$" "$tmpdir/checksums.txt" | cut -d ' ' -f 1)"
if [ -z "$expected" ]; then
  echo "trimia installer: checksum missing for $asset" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmpdir/$asset" | cut -d ' ' -f 1)"
elif command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "$tmpdir/$asset" | cut -d ' ' -f 1)"
else
  echo "trimia installer: missing sha256sum or shasum" >&2
  exit 1
fi

if [ "$actual" != "$expected" ]; then
  echo "trimia installer: checksum verification failed" >&2
  exit 1
fi

tar -xzf "$tmpdir/$asset" -C "$tmpdir" "$BINARY"

if [ ! -d "$INSTALL_DIR" ]; then
  mkdir -p "$INSTALL_DIR"
fi

target="$INSTALL_DIR/$BINARY"
if [ -w "$INSTALL_DIR" ]; then
  install -m 0755 "$tmpdir/$BINARY" "$target"
else
  echo "trimia installer: $INSTALL_DIR is not writable, trying sudo" >&2
  sudo install -m 0755 "$tmpdir/$BINARY" "$target"
fi

echo "trimia installed to $target"
echo "Run: trimia --version"
echo "Note: ffmpeg, ffprobe, and DEEPGRAM_API_KEY are required at runtime."
