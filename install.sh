#!/bin/sh
# Install or update Byrgenwerth on macOS 14 or newer. Run in Terminal:
#   curl -fsSL https://raw.githubusercontent.com/nguucode/byrgenwerth/main/install.sh | sh
# Data lives in ~/Library/Application Support/Byrgenwerth and is kept on update.
set -eu
repo=nguucode/byrgenwerth
dest="$HOME/Applications"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
fail() {
	echo "Could not download the Mac build. If a release just came out, it may still be building: try again in a few minutes." >&2
	exit 1
}
# Through the API, not github.com/.../releases/download: that link can answer 504
# while the API asset link still works.
curl -fsSL "https://api.github.com/repos/$repo/releases/latest" -o "$tmp/release.json" || fail
url= i=0
while name=$(plutil -extract "assets.$i.name" raw "$tmp/release.json" 2>/dev/null); do
	[ "$name" = Byrgenwerth-macos.zip ] && url=$(plutil -extract "assets.$i.url" raw "$tmp/release.json") && break
	i=$((i + 1))
done
[ -n "$url" ] || fail
# Downloaded with curl, so macOS does not quarantine this unnotarized app.
curl -fsSL -H 'Accept: application/octet-stream' "$url" -o "$tmp/app.zip" || fail
ditto -x -k "$tmp/app.zip" "$tmp"

pkill -x Byrgenwerth 2>/dev/null && sleep 1 || true
# The ad-hoc signature changes with every build, so a Screen Recording grant for
# the old build stays switched on but no longer applies. Clear it so macOS asks again.
[ -d "$dest/Byrgenwerth.app" ] && tccutil reset ScreenCapture com.nguucode.byrgenwerth >/dev/null 2>&1 || true
mkdir -p "$dest"
rm -rf "$dest/Byrgenwerth.app"
mv "$tmp/Byrgenwerth.app" "$dest/Byrgenwerth.app"
echo "Installed to $dest/Byrgenwerth.app"
echo "The first time, allow Screen Recording when macOS asks, then open the app again."
open "$dest/Byrgenwerth.app"
