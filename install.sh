#!/bin/sh
# reelterm installer: downloads the latest static binary from GitHub releases.
#
#   curl -fsSL https://github.com/jkpair/reelterm/releases/latest/download/install.sh | sh
#
# Options (pass after `sh -s --` when piping):
#   --user        install to ~/.local/bin instead of /usr/local/bin
#   --dir <path>  install to a specific directory
#   --version <v> install a specific release tag (e.g. v1.0.0)
set -eu

repo="jkpair/reelterm"
dir="/usr/local/bin"
version="latest"

while [ $# -gt 0 ]; do
  case "$1" in
  --user) dir="$HOME/.local/bin" ;;
  --dir) dir="$2"; shift ;;
  --version) version="$2"; shift ;;
  -h | --help) echo "usage: install.sh [--user] [--dir <path>] [--version <tag>]"; exit 0 ;;
  *) echo "unknown option: $1" >&2; exit 1 ;;
  esac
  shift
done

case "$(uname -s)" in
Linux) os=linux ;;
Darwin) os=darwin ;;
*) echo "unsupported OS: $(uname -s) (download a binary from https://github.com/$repo/releases)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
armv7* | armv8l) arch=armv7 ;;
*) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

asset="reelterm-$os-$arch"
if [ "$version" = latest ]; then
  url="https://github.com/$repo/releases/latest/download/$asset"
else
  url="https://github.com/$repo/releases/download/$version/$asset"
fi

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
echo "Downloading $asset ($version)..."
if command -v curl >/dev/null 2>&1; then
  curl -fL --progress-bar "$url" -o "$tmp"
elif command -v wget >/dev/null 2>&1; then
  wget -q --show-progress -O "$tmp" "$url"
else
  echo "curl or wget is required" >&2
  exit 1
fi
chmod 755 "$tmp"

sudo=""
mkdir -p "$dir" 2>/dev/null || true
if [ ! -w "$dir" ]; then
  if command -v sudo >/dev/null 2>&1; then
    sudo="sudo"
    echo "$dir is not writable, using sudo"
    $sudo mkdir -p "$dir"
  else
    echo "$dir is not writable; re-run with --user" >&2
    exit 1
  fi
fi
$sudo install -m 755 "$tmp" "$dir/reelterm"
echo "Installed $("$dir/reelterm" --version) to $dir/reelterm"

case ":$PATH:" in
*":$dir:"*) ;;
*) echo "Note: $dir is not in your PATH" ;;
esac

echo
"$dir/reelterm" --check-deps || echo "Run 'reelterm --install' to install the missing dependencies."
