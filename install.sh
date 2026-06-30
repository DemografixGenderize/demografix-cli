#!/bin/sh
# Install the demografix CLI from GitHub Releases.
#
#   curl -fsSL https://raw.githubusercontent.com/DemografixGenderize/demografix-cli/main/install.sh | sh
#
# Environment overrides:
#   VERSION                 pin a version (default: latest release)
#   DEMOGRAFIX_INSTALL_DIR  install directory (default: /usr/local/bin)
set -eu

REPO="DemografixGenderize/demografix-cli"
BIN="demografix"
INSTALL_DIR="${DEMOGRAFIX_INSTALL_DIR:-/usr/local/bin}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux | darwin) ;;
  *) echo "unsupported OS: $os" >&2; exit 1 ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac

version="${VERSION:-}"
if [ -z "$version" ]; then
  version=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | head -1 | sed -E 's/.*"v?([^"]+)".*/\1/')
fi
if [ -z "$version" ]; then
  echo "could not determine the latest version" >&2; exit 1
fi

asset="${BIN}_${version}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/v${version}"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $asset ..." >&2
curl -fsSL "$base/$asset" -o "$tmp/$asset"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"

if command -v sha256sum >/dev/null 2>&1; then
  sum="sha256sum"
else
  sum="shasum -a 256"
fi
( cd "$tmp" && grep " ${asset}\$" checksums.txt | $sum -c - >/dev/null )

tar -C "$tmp" -xzf "$tmp/$asset"

if install -m 0755 "$tmp/$BIN" "$INSTALL_DIR/$BIN" 2>/dev/null; then
  :
else
  echo "Elevating to write $INSTALL_DIR ..." >&2
  sudo install -m 0755 "$tmp/$BIN" "$INSTALL_DIR/$BIN"
fi

echo "Installed $("$INSTALL_DIR/$BIN" version)" >&2
