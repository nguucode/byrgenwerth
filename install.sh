#!/bin/sh
# Install or update Byrgenwerth on macOS 14 or newer. Run in Terminal:
#   curl -fsSL https://raw.githubusercontent.com/nguucode/byrgenwerth/main/install.sh | sh
# Data lives in ~/Library/Application Support/Byrgenwerth and is kept on update.
set -eu
repo=nguucode/byrgenwerth
dest="$HOME/Applications"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
# Downloaded with curl, so macOS does not quarantine this unnotarized app.
curl -fsSL "https://github.com/$repo/releases/latest/download/Byrgenwerth-macos.zip" -o "$tmp/app.zip" || {
	echo "Could not download the Mac build. If a release just came out, it may still be building: try again in a few minutes." >&2
	exit 1
}
ditto -x -k "$tmp/app.zip" "$tmp"

pkill -x Byrgenwerth 2>/dev/null && sleep 1 || true
mkdir -p "$dest"
rm -rf "$dest/Byrgenwerth.app"
mv "$tmp/Byrgenwerth.app" "$dest/Byrgenwerth.app"
echo "Installed to $dest/Byrgenwerth.app"
echo "The first time, allow Screen Recording when macOS asks, then open the app again."
open "$dest/Byrgenwerth.app"
